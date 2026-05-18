package channels

// MetaWebhookPayload is the top-level envelope for ALL Meta webhooks (WA, IG, FB).
type MetaWebhookPayload struct {
	Object string             `json:"object"` // "whatsapp_business_account", "instagram", or "page"
	Entry  []MetaWebhookEntry `json:"entry"`
}

type MetaWebhookEntry struct {
	ID      string                `json:"id"`
	Time    int64                 `json:"time"`
	Changes []MetaWebhookChange   `json:"changes,omitempty"`   // WhatsApp / some IG fields
	Messaging []MetaWebhookMessaging `json:"messaging,omitempty"` // Instagram DMs / FB Messenger
}

type MetaWebhookChange struct {
	Field string               `json:"field"`
	Value WhatsAppWebhookValue `json:"value"` // WhatsApp uses this
}

// MetaWebhookMessaging is used by Instagram DMs and Facebook Messenger.
type MetaWebhookMessaging struct {
	Sender    MetaWebhookUser    `json:"sender"`
	Recipient MetaWebhookUser    `json:"recipient"`
	Timestamp int64              `json:"timestamp"`
	Message   *MetaWebhookMessage `json:"message,omitempty"`
}

type MetaWebhookUser struct {
	ID string `json:"id"` // The Scoped User ID (PSID or IGSID)
}

type MetaWebhookMessage struct {
	Mid  string `json:"mid"`
	Text string `json:"text,omitempty"`
	// Attachments, replies, etc can be added here as needed.
}
