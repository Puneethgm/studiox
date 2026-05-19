package messaging

import (
	"context"
	"encoding/json"
	"sync"

	"github.com/google/uuid"
)

// In-process pub/sub. SSE subscribes here for live UI updates today; phase D
// (automations) and phase E (AI suggester) will subscribe to the same bus.
//
// In-process is fine for L1 with a single API replica. When we go multi-replica
// we swap the implementation for Postgres LISTEN/NOTIFY (or Redis Pub/Sub) —
// the Bus interface stays identical so subscribers don't change.

type EventKind string

const (
	EvtMessageReceived EventKind = "message.received" // inbound from customer
	EvtMessageSent     EventKind = "message.sent"     // outbound delivered to channel
	EvtConversationUpdated EventKind = "conversation.updated"
)

type Event struct {
	Kind            EventKind  `json:"kind"`
	StudioID        uuid.UUID  `json:"studioId"`
	ConversationID  uuid.UUID  `json:"conversationId"`
	MessageID       *uuid.UUID `json:"messageId,omitempty"`
}

func (e Event) JSON() string {
	b, _ := json.Marshal(e)
	return string(b)
}

// Bus is the pub/sub interface. Buffered per-subscriber channel so a slow
// client (e.g. an SSE consumer behind a sluggish proxy) doesn't block publishes.
type Bus interface {
	Publish(ctx context.Context, evt Event)
	Subscribe(studioID uuid.UUID) (<-chan Event, func())
}

// ----- in-process implementation -----

type InProcBus struct {
	mu   sync.RWMutex
	subs map[uuid.UUID]map[int]chan Event
	next int
}

func NewInProcBus() *InProcBus {
	return &InProcBus{subs: make(map[uuid.UUID]map[int]chan Event)}
}

func (b *InProcBus) Publish(_ context.Context, evt Event) {
	b.mu.RLock()
	chans := b.subs[evt.StudioID]
	pending := make([]chan Event, 0, len(chans))
	for _, c := range chans {
		pending = append(pending, c)
	}
	b.mu.RUnlock()

	for _, c := range pending {
		// Non-blocking send — drop if subscriber's buffer is full so a slow
		// client never wedges the publisher.
		select {
		case c <- evt:
		default:
		}
	}
}

func (b *InProcBus) Subscribe(studioID uuid.UUID) (<-chan Event, func()) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.subs[studioID] == nil {
		b.subs[studioID] = make(map[int]chan Event)
	}
	id := b.next
	b.next++
	ch := make(chan Event, 16)
	b.subs[studioID][id] = ch

	unsub := func() {
		b.mu.Lock()
		defer b.mu.Unlock()
		if m := b.subs[studioID]; m != nil {
			if c, ok := m[id]; ok {
				close(c)
				delete(m, id)
			}
			if len(m) == 0 {
				delete(b.subs, studioID)
			}
		}
	}
	return ch, unsub
}
