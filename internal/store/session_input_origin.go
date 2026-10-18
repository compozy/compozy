package store

import (
	"encoding/json"
	"strings"
)

// Session input origin classes rank queued input for dispatch: human operator
// input first, then input sent by another session, then daemon-owned wakes.
const (
	SessionInputOriginClassHuman     = "human"
	SessionInputOriginClassAgent     = "agent"
	SessionInputOriginClassSynthetic = "synthetic"
)

const sessionInputOriginKindSession = "session"

// SessionInputOriginClassFor derives the persisted origin class and sender
// session from the owner kind and the stored prompt origin. Rows owned by the
// daemon (synthetic wakes, Goal prompts) are synthetic; rows carrying a session
// origin are agent-authored; everything else is human operator input.
func SessionInputOriginClassFor(ownerKind string, origin json.RawMessage) (class, senderSessionID string) {
	if strings.TrimSpace(ownerKind) != "" {
		return SessionInputOriginClassSynthetic, ""
	}
	if len(origin) == 0 {
		return SessionInputOriginClassHuman, ""
	}
	var decoded struct {
		Kind      string `json:"kind"`
		SessionID string `json:"session_id"`
	}
	if err := json.Unmarshal(origin, &decoded); err != nil ||
		strings.TrimSpace(decoded.Kind) != sessionInputOriginKindSession {
		return SessionInputOriginClassHuman, ""
	}
	return SessionInputOriginClassAgent, strings.TrimSpace(decoded.SessionID)
}

// IsHumanSessionInputOriginClass reports whether a class belongs to the human cap and priority tier.
func IsHumanSessionInputOriginClass(class string) bool {
	return strings.TrimSpace(class) == "" || strings.TrimSpace(class) == SessionInputOriginClassHuman
}

func (r SessionInputQueueInsert) normalizeOriginClass() SessionInputQueueInsert {
	r.OriginClass = strings.TrimSpace(r.OriginClass)
	r.OriginSessionID = strings.TrimSpace(r.OriginSessionID)
	if r.OriginClass == "" {
		r.OriginClass, r.OriginSessionID = SessionInputOriginClassFor(r.OwnerKind, r.Origin)
	}
	if r.OriginClass == SessionInputOriginClassHuman {
		// Human-initiated input (including a promoted agent row) supersedes only human steers.
		r.OriginSessionID = ""
	}
	return r
}

// SessionInputStaleDispatchSummary is the failure summary of a lease that a
// daemon restart left without a known dispatch outcome.
const SessionInputStaleDispatchSummary = "daemon restarted while this input was dispatching; delivery outcome unknown"
