package store

import (
	"encoding/json"
	"errors"
	"slices"
	"strings"
)

// SessionInputOwnerSynthetic identifies daemon-authored follow-ups in the shared queue.
const SessionInputOwnerSynthetic = "synthetic"

// SessionInputSyntheticPrompt preserves execution identity and opaque ACP metadata.
type SessionInputSyntheticPrompt struct {
	Delivery string          `json:"delivery,omitempty"`
	RunID    string          `json:"run_id"`
	Metadata json.RawMessage `json:"metadata"`
	// CoalesceKey marks a wake row that absorbs later wakes of the same key while it waits. Rows without
	// it (including every row queued before coalescing shipped) stay individual entries.
	CoalesceKey string `json:"coalesce_key,omitempty"`
	// CoalescedMessageIDs lists the wakes merged behind the row's own message id. It is the admission
	// ledger for those wakes, so a replayed wake finds its row across a restart.
	CoalescedMessageIDs []string `json:"coalesced_message_ids,omitempty"`
	// Coalesce is the merge request for this admission only; it never persists.
	Coalesce *SessionInputCoalesce `json:"-"`
}

// CoalesceBounds returns the merge request of a synthetic wake, or nil.
func (r SessionInputQueueInsert) CoalesceBounds() *SessionInputCoalesce {
	if r.SyntheticPrompt == nil {
		return nil
	}
	return r.SyntheticPrompt.Coalesce
}

// SessionInputCoalesce asks the store to merge a synthetic wake into the session's open row of the same
// key. Such a row sits outside the capacity pools: it is one row, bounded by MaxMembers.
type SessionInputCoalesce struct {
	// CompactText replaces the wake's text once the row holds MaxFullBytes, so a burst drops output tails
	// rather than outcomes.
	CompactText  string
	MaxFullBytes int
	// MaxMembers refuses a further wake with ErrSessionInputQueueFull until the row is delivered.
	MaxMembers int
}

// CoalescedWakes reports how many wakes a coalesced row carries; zero for any other row.
func (p *SessionInputSyntheticPrompt) CoalescedWakes() int {
	if p == nil || p.CoalesceKey == "" {
		return 0
	}
	return 1 + len(p.CoalescedMessageIDs)
}

// Clone isolates queued metadata from the caller's mutable buffer.
func (p *SessionInputSyntheticPrompt) Clone() *SessionInputSyntheticPrompt {
	if p == nil {
		return nil
	}
	return &SessionInputSyntheticPrompt{
		Delivery: p.Delivery,
		RunID:    strings.TrimSpace(p.RunID),
		Metadata: append(json.RawMessage(nil), p.Metadata...),

		CoalesceKey:         p.CoalesceKey,
		CoalescedMessageIDs: slices.Clone(p.CoalescedMessageIDs),
		Coalesce:            p.Coalesce,
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
	if r.CoalesceBounds() != nil &&
		(r.SyntheticPrompt.CoalesceKey == "" || r.MessageID == "" || r.SyntheticPrompt.Coalesce.MaxMembers <= 0) {
		return errors.New("store: coalesced input requires a coalesce key, message id, and member bound")
	}
	return nil
}
