package globaldb

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/store/globaldb/sqlcgen"
)

func applyNetworkWorkMutation(
	ctx context.Context,
	exec networkSQLExecutor,
	entry store.NetworkConversationMessage,
) (networkWorkMutation, error) {
	if strings.TrimSpace(entry.WorkID) == "" {
		return networkWorkMutation{}, nil
	}

	current, err := getNetworkWorkWithExecutor(ctx, exec, entry.WorkspaceID, entry.WorkID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return openNetworkWorkWithExecutor(ctx, exec, entry)
		}
		return networkWorkMutation{}, err
	}
	if !networkWorkMatchesMessage(current, entry) {
		return networkWorkMutation{}, fmt.Errorf("%w: work_id=%q", store.ErrNetworkWorkContainerMismatch, entry.WorkID)
	}
	if networkWorkStateIsTerminal(current.State) {
		return networkWorkMutation{}, fmt.Errorf("%w: work_id=%q", store.ErrNetworkWorkClosed, entry.WorkID)
	}

	next, transitioned, err := nextNetworkWorkState(current.State, entry)
	if err != nil {
		return networkWorkMutation{}, err
	}
	if !transitioned {
		return networkWorkMutation{state: current.State}, nil
	}

	terminalAt := sql.NullString{}
	if networkWorkStateIsTerminal(next) {
		terminalAt = sql.NullString{String: store.FormatTimestamp(entry.Timestamp), Valid: true}
	}
	if err := sqlcgen.New(exec).UpdateNetworkWork(ctx, sqlcgen.UpdateNetworkWorkParams{
		State: next, LastActivityAt: store.FormatTimestamp(entry.Timestamp), TerminalAt: terminalAt,
		WorkspaceID: entry.WorkspaceID, WorkID: entry.WorkID,
	}); err != nil {
		return networkWorkMutation{}, fmt.Errorf("store: update network work: %w", err)
	}
	return networkWorkMutation{transitioned: true, state: next}, nil
}

func openNetworkWorkWithExecutor(
	ctx context.Context,
	exec networkSQLExecutor,
	entry store.NetworkConversationMessage,
) (networkWorkMutation, error) {
	if entry.Kind != store.NetworkKindSay && entry.Kind != store.NetworkKindCapability {
		return networkWorkMutation{}, fmt.Errorf(
			"store: network work %q does not exist: %w",
			entry.WorkID,
			sql.ErrNoRows,
		)
	}
	if strings.TrimSpace(entry.PeerTo) == "" {
		return networkWorkMutation{}, fmt.Errorf("store: network work target peer is required")
	}
	if err := sqlcgen.New(exec).InsertNetworkWork(ctx, sqlcgen.InsertNetworkWorkParams{
		WorkID: entry.WorkID, WorkspaceID: entry.WorkspaceID, Channel: entry.Channel,
		Surface: entry.Surface, ThreadID: nullableNetworkString(entry.ThreadID),
		DirectID: nullableNetworkString(entry.DirectID), OpenedBySessionID: entry.SessionID,
		TargetSessionID: nullableNetworkString(entry.PeerTo),
		State:           store.NetworkWorkStateSubmitted, OpenedAt: store.FormatTimestamp(entry.Timestamp),
		LastActivityAt: store.FormatTimestamp(entry.Timestamp),
	}); err != nil {
		return networkWorkMutation{}, fmt.Errorf("store: insert network work: %w", err)
	}
	return networkWorkMutation{opened: true, state: store.NetworkWorkStateSubmitted}, nil
}

func getNetworkWorkWithExecutor(
	ctx context.Context,
	exec networkSQLExecutor,
	workspaceID string,
	workID string,
) (store.NetworkWorkEntry, error) {
	row, err := sqlcgen.New(exec).GetNetworkWork(ctx, sqlcgen.GetNetworkWorkParams{
		WorkspaceID: workspaceID, WorkID: workID,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return store.NetworkWorkEntry{}, fmt.Errorf(
				"%w: network work: %w",
				store.ErrNetworkConversationNotFound,
				err,
			)
		}
		return store.NetworkWorkEntry{}, fmt.Errorf("store: get network work: %w", err)
	}
	return networkWorkFromGenerated(row)
}

func networkWorkFromGenerated(row sqlcgen.NetworkWork) (store.NetworkWorkEntry, error) {
	openedAt, err := store.ParseTimestamp(row.OpenedAt)
	if err != nil {
		return store.NetworkWorkEntry{}, fmt.Errorf("store: parse network work opened_at: %w", err)
	}
	lastActivityAt, err := store.ParseTimestamp(row.LastActivityAt)
	if err != nil {
		return store.NetworkWorkEntry{}, fmt.Errorf("store: parse network work last_activity_at: %w", err)
	}
	entry := store.NetworkWorkEntry{
		WorkID: row.WorkID, WorkspaceID: row.WorkspaceID, Channel: row.Channel, Surface: row.Surface,
		ThreadID: row.ThreadID.String, DirectID: row.DirectID.String,
		OpenedBySessionID: row.OpenedBySessionID, TargetSessionID: row.TargetSessionID.String, State: row.State,
		OpenedAt: openedAt, LastActivityAt: lastActivityAt,
	}
	if row.TerminalAt.Valid {
		terminalAt, parseErr := store.ParseTimestamp(row.TerminalAt.String)
		if parseErr != nil {
			return store.NetworkWorkEntry{}, fmt.Errorf("store: parse network work terminal_at: %w", parseErr)
		}
		entry.TerminalAt = &terminalAt
	}
	if err := entry.Validate(); err != nil {
		return store.NetworkWorkEntry{}, err
	}
	return entry, nil
}

func networkWorkMatchesMessage(work store.NetworkWorkEntry, entry store.NetworkConversationMessage) bool {
	return work.WorkspaceID == entry.WorkspaceID &&
		work.Channel == entry.Channel &&
		work.Surface == entry.Surface &&
		strings.TrimSpace(work.ThreadID) == strings.TrimSpace(entry.ThreadID) &&
		strings.TrimSpace(work.DirectID) == strings.TrimSpace(entry.DirectID)
}

func nextNetworkWorkState(current string, entry store.NetworkConversationMessage) (string, bool, error) {
	switch entry.Kind {
	case store.NetworkKindSay, store.NetworkKindCapability:
		if current == store.NetworkWorkStateNeedsInput {
			return store.NetworkWorkStateWorking, true, nil
		}
		return current, false, nil
	case store.NetworkKindReceipt:
		return nextNetworkWorkStateFromReceipt(current, entry.Body)
	case store.NetworkKindTrace:
		return nextNetworkWorkStateFromTrace(current, entry.Body)
	default:
		return current, false, nil
	}
}

func nextNetworkWorkStateFromReceipt(current string, body json.RawMessage) (string, bool, error) {
	var receipt struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal(body, &receipt); err != nil {
		return "", false, fmt.Errorf("store: decode network receipt body: %w", err)
	}
	switch strings.TrimSpace(receipt.Status) {
	case "accepted", "duplicate", "expired", "unsupported":
		return current, false, nil
	case globalDBNetworkConversationsRejectedKey:
		return store.NetworkWorkStateFailed, true, nil
	case "canceled":
		return store.NetworkWorkStateCanceled, true, nil
	default:
		return "", false, fmt.Errorf("store: unsupported network receipt status %q", receipt.Status)
	}
}

func nextNetworkWorkStateFromTrace(current string, body json.RawMessage) (string, bool, error) {
	var trace struct {
		State string `json:"state"`
	}
	if err := json.Unmarshal(body, &trace); err != nil {
		return "", false, fmt.Errorf("store: decode network trace body: %w", err)
	}
	next := strings.TrimSpace(trace.State)
	if !canAdvanceNetworkWorkState(current, next) {
		return "", false, fmt.Errorf("store: invalid network work transition %s -> %s", current, next)
	}
	return next, true, nil
}

func canAdvanceNetworkWorkState(current string, next string) bool {
	if networkWorkStateIsTerminal(current) {
		return false
	}
	switch current {
	case store.NetworkWorkStateSubmitted, store.NetworkWorkStateWorking, store.NetworkWorkStateNeedsInput:
	default:
		return false
	}
	switch next {
	case store.NetworkWorkStateWorking,
		store.NetworkWorkStateNeedsInput,
		store.NetworkWorkStateCompleted,
		store.NetworkWorkStateFailed,
		store.NetworkWorkStateCanceled:
		return true
	default:
		return false
	}
}

func networkWorkStateIsTerminal(state string) bool {
	switch state {
	case store.NetworkWorkStateCompleted, store.NetworkWorkStateFailed, store.NetworkWorkStateCanceled:
		return true
	default:
		return false
	}
}
