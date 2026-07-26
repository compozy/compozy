package contract

import (
	"time"

	toolspkg "github.com/compozy/agh/internal/tools"
)

// ClarificationAnswerRequest resolves one live clarification through a public transport.
type ClarificationAnswerRequest struct {
	ChoiceIndex *int   `json:"choice_index,omitempty"`
	Text        string `json:"text,omitempty"`
}

// ClarificationAnswerPayload is the exact public clarification result.
type ClarificationAnswerPayload = toolspkg.ClarifyAnswer

// ClarificationPendingPayload is the public live pending projection.
type ClarificationPendingPayload struct {
	RequestID string    `json:"request_id"`
	SessionID string    `json:"session_id"`
	AgentName string    `json:"agent_name"`
	Question  string    `json:"question"`
	Choices   []string  `json:"choices,omitempty"`
	AskedAt   time.Time `json:"asked_at"`
	Deadline  time.Time `json:"deadline"`
}

// ClarificationsResponse lists the complete live pending projection for one session.
type ClarificationsResponse struct {
	Clarifications []ClarificationPendingPayload `json:"clarifications"`
}

// ClarificationPendingPayloadFromDomain removes internal workspace ownership from the response body.
func ClarificationPendingPayloadFromDomain(pending toolspkg.ClarifyPending) ClarificationPendingPayload {
	return ClarificationPendingPayload{
		RequestID: pending.RequestID,
		SessionID: pending.SessionID,
		AgentName: pending.AgentName,
		Question:  pending.Question,
		Choices:   append([]string(nil), pending.Choices...),
		AskedAt:   pending.AskedAt,
		Deadline:  pending.Deadline,
	}
}
