package store

import (
	"encoding/json"
	"errors"
	"strings"
)

// SessionInputOwnerSynthetic identifies daemon-authored follow-ups in the shared queue.
const SessionInputOwnerSynthetic = "synthetic"

// SessionInputSyntheticPrompt preserves execution identity and opaque ACP metadata.
type SessionInputSyntheticPrompt struct {
	RunID    string          `json:"run_id"`
	Metadata json.RawMessage `json:"metadata"`
}

// Clone isolates queued metadata from the caller's mutable buffer.
func (p *SessionInputSyntheticPrompt) Clone() *SessionInputSyntheticPrompt {
	if p == nil {
		return nil
	}
	return &SessionInputSyntheticPrompt{
		RunID:    strings.TrimSpace(p.RunID),
		Metadata: append(json.RawMessage(nil), p.Metadata...),
	}
}

func (r SessionInputQueueInsert) validateSyntheticPrompt() error {
	if r.OwnerKind == "" && r.SyntheticPrompt == nil {
		return nil
	}
	if r.OwnerKind != SessionInputOwnerSynthetic || r.SyntheticPrompt == nil {
		return errors.New("store: synthetic queue owner and payload are required together")
	}
	if r.Mode != SessionInputQueueModeQueue || r.TurnID == "" || r.SyntheticPrompt.RunID == "" ||
		!json.Valid(r.SyntheticPrompt.Metadata) || string(r.SyntheticPrompt.Metadata) == "null" {
		return errors.New("store: synthetic input requires queued delivery, turn id, run id, and valid metadata")
	}
	return nil
}
