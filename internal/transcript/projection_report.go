package transcript

import (
	"encoding/json"

	"github.com/compozy/compozy/internal/store"
)

// ReportEventType is the durable event a stays-on agent's report lands as. It starts no turn.
const ReportEventType = "session.report"

const reportPartType = "data-compozy-report"

// projectReportEntry carries the stored report through as one data part, redacted like every
// other display payload. A report that doesn't decode stays out of the transcript.
func projectReportEntry(stored store.SessionEvent, identity EntryIdentity) *Entry {
	raw := json.RawMessage(stored.Content)
	if !json.Valid(raw) {
		return nil
	}
	var head struct {
		ID string `json:"report_id"`
	}
	if err := json.Unmarshal(raw, &head); err != nil || head.ID == "" {
		return nil
	}
	return &Entry{
		Message: UIMessage{
			ID:    fallbackMessageID(identity.MessageID, identity.BaseMessageID),
			Role:  UIRoleSystem,
			Parts: []UIMessagePart{{Type: reportPartType, ID: head.ID, Data: redactRawMessage(raw)}},
		},
		StartSequence: identity.StartSequence,
		Sequence:      identity.UpdatedSequence,
		EventType:     ReportEventType,
	}
}
