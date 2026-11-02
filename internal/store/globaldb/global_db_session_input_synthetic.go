package globaldb

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
)

// coalesceWakeSeparator divides the outcomes of a coalesced wake row.
const coalesceWakeSeparator = "\n\n---\n\n"

// coalesceSessionInput merges a wake into the session's open row of its key and reports false when no
// row is open. The transaction serializes it with the claim, so a member lands either in a row that is
// still waiting or in a new one, and never in a row that already reached the agent.
func coalesceSessionInput(
	ctx context.Context,
	exec globalSQLExecutor,
	req store.SessionInputQueueInsert,
) (store.SessionInputQueueEntry, bool, error) {
	queries := sqlcgen.New(exec)
	row, err := queries.GetCoalescibleSessionInput(ctx, sqlcgen.GetCoalescibleSessionInputParams{
		SessionID: req.SessionID, CoalesceKey: req.SyntheticPrompt.CoalesceKey,
	})
	if errors.Is(err, sql.ErrNoRows) {
		return store.SessionInputQueueEntry{}, false, nil
	}
	if err != nil {
		return store.SessionInputQueueEntry{}, false, fmt.Errorf("store: read open coalesced input: %w", err)
	}
	open, err := sessionInputQueueFromGenerated(&row)
	if err != nil {
		return store.SessionInputQueueEntry{}, false, err
	}
	// A wake read before a generation advance stays out of the new generation's row.
	if open.SessionGeneration != req.SessionGeneration {
		return store.SessionInputQueueEntry{}, false, nil
	}
	members := open.SyntheticPrompt.CoalescedWakes()
	if members >= req.SyntheticPrompt.Coalesce.MaxMembers {
		return store.SessionInputQueueEntry{}, false, &store.SessionInputQueueFullError{
			SessionID: req.SessionID, Cap: req.SyntheticPrompt.Coalesce.MaxMembers, Count: members,
		}
	}
	text := req.Text
	if len(open.Text)+len(text) > req.SyntheticPrompt.Coalesce.MaxFullBytes &&
		req.SyntheticPrompt.Coalesce.CompactText != "" {
		text = req.SyntheticPrompt.Coalesce.CompactText
	}
	prompt := open.SyntheticPrompt.Clone()
	prompt.CoalescedMessageIDs = append(prompt.CoalescedMessageIDs, req.MessageID)
	encoded, err := encodeSyntheticQueuePrompt(prompt)
	if err != nil {
		return store.SessionInputQueueEntry{}, false, err
	}
	affected, err := queries.AppendCoalescedSessionInput(ctx, sqlcgen.AppendCoalescedSessionInputParams{
		Text: open.Text + coalesceWakeSeparator + text, SyntheticPromptJson: encoded,
		Now: store.FormatTimestamp(req.Now), ID: open.ID, SessionID: req.SessionID,
	})
	if err != nil {
		return store.SessionInputQueueEntry{}, false, fmt.Errorf("store: append coalesced input: %w", err)
	}
	if affected != 1 {
		return store.SessionInputQueueEntry{}, false, store.ErrSessionInputQueueEntryNotQueued
	}
	merged, err := getSessionInputQueueEntry(ctx, exec, req.SessionID, open.ID)
	return merged, err == nil, err
}

func encodeSyntheticQueuePrompt(prompt *store.SessionInputSyntheticPrompt) (sql.NullString, error) {
	if prompt == nil {
		return sql.NullString{}, nil
	}
	encoded, err := json.Marshal(prompt)
	if err != nil {
		return sql.NullString{}, fmt.Errorf("store: encode synthetic queued prompt: %w", err)
	}
	return sql.NullString{String: string(encoded), Valid: true}, nil
}

func decodeSyntheticQueuePrompt(raw sql.NullString, entry *store.SessionInputQueueEntry) error {
	if !raw.Valid {
		return nil
	}
	if err := json.Unmarshal([]byte(raw.String), &entry.SyntheticPrompt); err != nil {
		return fmt.Errorf("store: decode synthetic queued prompt: %w", err)
	}
	return nil
}
