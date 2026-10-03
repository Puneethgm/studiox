package messaging

import (
	"context"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
)

// A studio whose send loop is stuck (disconnected channel, long pacing wait)
// must not delay another studio's jobs, and must not be re-claimed while busy.
func TestOutboundWorkerIsolatesStudios(t *testing.T) {
	studioA, studioB := uuid.New(), uuid.New()
	release := make(chan struct{})
	bDone := make(chan struct{})

	var mu sync.Mutex
	var excludes [][]uuid.UUID
	calls := 0

	w := &OutboundWorker{
		log:  slog.New(slog.NewTextHandler(io.Discard, nil)),
		busy: make(map[uuid.UUID]struct{}),
	}
	w.claimFn = func(_ context.Context, _ int, exclude []uuid.UUID) ([]OutboundJob, error) {
		mu.Lock()
		defer mu.Unlock()
		excludes = append(excludes, exclude)
		calls++
		if calls == 1 {
			return []OutboundJob{{ID: 1, StudioID: studioA}, {ID: 2, StudioID: studioB}}, nil
		}
		return nil, nil
	}
	w.dispatchFn = func(_ context.Context, j OutboundJob) {
		if j.StudioID == studioA {
			<-release // studio A is stuck until released
			return
		}
		close(bDone)
	}

	w.tick(context.Background())

	select {
	case <-bDone:
	case <-time.After(2 * time.Second):
		t.Fatal("studio B's job was blocked by studio A's stuck send")
	}

	// While A is still sending, the next claim must exclude A (and only A).
	w.tick(context.Background())
	mu.Lock()
	got := excludes[len(excludes)-1]
	mu.Unlock()
	if len(got) != 1 || got[0] != studioA {
		t.Fatalf("second claim exclude = %v, want [%v]", got, studioA)
	}

	close(release)
	w.wg.Wait()
	w.tick(context.Background())
	mu.Lock()
	got = excludes[len(excludes)-1]
	mu.Unlock()
	if len(got) != 0 {
		t.Fatalf("after A finished, exclude = %v, want empty", got)
	}
}
