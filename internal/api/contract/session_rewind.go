package contract

// SessionConversationRewindRequest carries the durable message identity and
// transcript fences observed by the caller.
type SessionConversationRewindRequest struct {
	MessageID           string `json:"message_id"`
	IdempotencyKey      string `json:"idempotency_key"`
	ExpectedEpoch       *int64 `json:"expected_epoch"`
	ExpectedGeneration  *int64 `json:"expected_generation"`
	ExpectedMaxSequence *int64 `json:"expected_max_sequence"`
}

// SessionConversationRewindResponse returns the restarted session, the new
// transcript coordinates, and the selected prompt text for composer prefill.
type SessionConversationRewindResponse struct {
	Session SessionPayload                   `json:"session"`
	Rewind  SessionConversationRewindPayload `json:"rewind"`
}

// SessionConversationRewindPayload describes the committed transcript cut.
type SessionConversationRewindPayload struct {
	TranscriptEpoch int64  `json:"transcript_epoch"`
	TargetMessageID string `json:"target_message_id"`
	ArchivedFrom    int64  `json:"archived_from_sequence"`
	ArchivedThrough int64  `json:"archived_through_sequence"`
	ArchivedEvents  int64  `json:"archived_event_count"`
	Generation      int64  `json:"generation"`
	MaxSequence     int64  `json:"max_sequence"`
	DraftText       string `json:"draft_text"`
	Replayed        bool   `json:"replayed"`
}
