package globaldb

import (
	"context"

	"errors"
	"fmt"
	"strings"

	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/store/globaldb/sqlcgen"
)

const (
	globalDBNetworkConversationsWorkspaceIDValue = "workspace_id = ?"
)

const (
	globalDBNetworkConversationsChannelValue = "channel = ?"
	globalDBNetworkConversationsDirectIDKey  = "direct_id"
	globalDBNetworkConversationsRejectedKey  = "rejected"
	globalDBNetworkConversationsThreadIDKey  = "thread_id"
)

type networkWorkMutation struct {
	opened       bool
	transitioned bool
	state        string
}

// ResolveDirectRoom inserts or returns the deterministic two-party room.
func (g *NetworkRepo) ResolveDirectRoom(
	ctx context.Context,
	entry store.NetworkDirectRoomEntry,
) (summary store.NetworkDirectRoomSummary, err error) {
	if err := g.checkReady(ctx, "resolve network direct room"); err != nil {
		return store.NetworkDirectRoomSummary{}, err
	}
	normalized, err := g.normalizeDirectRoomEntry(entry)
	if err != nil {
		return store.NetworkDirectRoomSummary{}, err
	}

	if err := g.withNetworkImmediateTransaction(
		ctx,
		"resolve network direct room",
		func(exec networkSQLExecutor) error {
			resolved, _, resolveErr := resolveDirectRoomWithExecutor(ctx, exec, normalized)
			if resolveErr != nil {
				return resolveErr
			}
			summary = resolved
			return nil
		},
	); err != nil {
		return store.NetworkDirectRoomSummary{}, err
	}
	return summary, nil
}

// WriteConversationMessage persists one message and its derived state atomically.
func (g *NetworkRepo) WriteConversationMessage(
	ctx context.Context,
	entry store.NetworkConversationMessage,
) (result store.NetworkConversationWriteResult, err error) {
	if err := g.checkReady(ctx, "write network conversation message"); err != nil {
		return store.NetworkConversationWriteResult{}, err
	}
	normalized, err := g.normalizeConversationMessage(entry)
	if err != nil {
		return store.NetworkConversationWriteResult{}, err
	}
	result.MessageID = normalized.MessageID

	if err := g.withNetworkImmediateTransaction(
		ctx,
		"write network conversation message",
		func(exec networkSQLExecutor) error {
			persisted, _, persistErr := persistNetworkConversationMessageWithExecutor(ctx, exec, normalized)
			result = persisted
			return persistErr
		},
	); err != nil {
		return store.NetworkConversationWriteResult{}, err
	}
	return result, nil
}

func persistNetworkConversationMessageWithExecutor(
	ctx context.Context,
	exec networkSQLExecutor,
	message store.NetworkConversationMessage,
) (store.NetworkConversationWriteResult, int64, error) {
	result := store.NetworkConversationWriteResult{MessageID: message.MessageID}
	inserted, sequence, err := insertNetworkTimelineMessageWithExecutor(ctx, exec, message)
	if err != nil {
		return store.NetworkConversationWriteResult{}, 0, err
	}
	if !inserted {
		sequence, result.LastActivityAt, err = lookupNetworkMessageAcceptance(
			ctx,
			exec,
			message.WorkspaceID,
			message.MessageID,
		)
		result.Duplicate = true
		return result, sequence, err
	}
	message.Sequence = sequence

	result.ConversationOpened, err = ensureNetworkConversationContainer(ctx, exec, message)
	if err != nil {
		return store.NetworkConversationWriteResult{}, 0, err
	}
	work, err := applyNetworkWorkMutation(ctx, exec, message)
	if err != nil {
		return store.NetworkConversationWriteResult{}, 0, err
	}
	result.WorkOpened = work.opened
	result.WorkTransitioned = work.transitioned
	result.WorkState = work.state
	if err := persistNetworkTimelineWorkProjection(ctx, exec, message, work); err != nil {
		return store.NetworkConversationWriteResult{}, 0, err
	}
	if message.Surface == store.NetworkSurfaceThread {
		if err := upsertNetworkThreadParticipantsForMessage(ctx, exec, message); err != nil {
			return store.NetworkConversationWriteResult{}, 0, err
		}
	}
	if err := refreshNetworkConversationSummary(ctx, exec, message); err != nil {
		return store.NetworkConversationWriteResult{}, 0, err
	}
	if err := insertNetworkAuditWithExecutor(ctx, exec, auditEntryForConversationMessage(message)); err != nil {
		return store.NetworkConversationWriteResult{}, 0, err
	}
	result.LastActivityAt = message.Timestamp
	return result, sequence, nil
}

func upsertNetworkThreadParticipantsForMessage(
	ctx context.Context,
	exec networkSQLExecutor,
	message store.NetworkConversationMessage,
) error {
	return upsertNetworkThreadParticipant(ctx, exec, message)
}

// GetThread returns one public-thread summary.
func (g *NetworkRepo) GetThread(
	ctx context.Context,
	channelRef store.NetworkChannelRef,
	threadID string,
) (store.NetworkThreadSummary, error) {
	if err := g.checkReady(ctx, "get network thread"); err != nil {
		return store.NetworkThreadSummary{}, err
	}
	ref := store.NetworkConversationRef{
		WorkspaceID: strings.TrimSpace(channelRef.WorkspaceID),
		Channel:     strings.TrimSpace(channelRef.Channel),
		Surface:     store.NetworkSurfaceThread,
		ThreadID:    strings.TrimSpace(threadID),
	}
	if err := ref.Validate(); err != nil {
		return store.NetworkThreadSummary{}, err
	}

	row, err := g.queries.GetNetworkThread(ctx, sqlcgen.GetNetworkThreadParams{
		WorkspaceID: ref.WorkspaceID, Channel: ref.Channel, ThreadID: ref.ThreadID,
	})
	return networkThreadFromGenerated(row, err)
}

// GetDirectRoom returns one direct-room summary.
func (g *NetworkRepo) GetDirectRoom(
	ctx context.Context,
	channelRef store.NetworkChannelRef,
	directID string,
) (store.NetworkDirectRoomSummary, error) {
	if err := g.checkReady(ctx, "get network direct room"); err != nil {
		return store.NetworkDirectRoomSummary{}, err
	}
	ref := store.NetworkConversationRef{
		WorkspaceID: strings.TrimSpace(channelRef.WorkspaceID),
		Channel:     strings.TrimSpace(channelRef.Channel),
		Surface:     store.NetworkSurfaceDirect,
		DirectID:    strings.TrimSpace(directID),
	}
	if err := ref.Validate(); err != nil {
		return store.NetworkDirectRoomSummary{}, err
	}

	row, err := g.queries.GetNetworkDirectRoom(ctx, sqlcgen.GetNetworkDirectRoomParams{
		WorkspaceID: ref.WorkspaceID, Channel: ref.Channel, DirectID: ref.DirectID,
	})
	return networkDirectRoomFromGenerated(row, err)
}

// ListConversationMessages returns messages isolated to one conversation container.
func (g *NetworkRepo) ListConversationMessages(
	ctx context.Context,
	ref store.NetworkConversationRef,
	query store.NetworkConversationMessageQuery,
) (entries []store.NetworkConversationMessage, err error) {
	if err := g.checkReady(ctx, "list network conversation messages"); err != nil {
		return nil, err
	}
	normalizedRef := normalizeNetworkConversationRef(ref)
	if err := normalizedRef.Validate(); err != nil {
		return nil, err
	}
	if err := query.Validate(); err != nil {
		return nil, fmt.Errorf("store: validate network conversation message query: %w", err)
	}

	// dynamic-sql: container type, optional filters, cursor direction, sort order, and limit change the statement shape.
	sqlQuery := networkConversationMessageSelect()
	where, args := networkConversationMessageFilterClauses(normalizedRef, query)
	reverseResults := false
	switch {
	case strings.TrimSpace(query.BeforeMessageID) != "":
		cursor, cursorErr := g.lookupNetworkConversationMessageCursor(ctx, normalizedRef, query.BeforeMessageID, query)
		if cursorErr != nil {
			return nil, cursorErr
		}
		where = append(where, "sequence < ?")
		args = append(args, cursor.Sequence)
		reverseResults = true
	case strings.TrimSpace(query.AfterMessageID) != "":
		cursor, cursorErr := g.lookupNetworkConversationMessageCursor(ctx, normalizedRef, query.AfterMessageID, query)
		if cursorErr != nil {
			return nil, cursorErr
		}
		where = append(where, "sequence > ?")
		args = append(args, cursor.Sequence)
	default:
		reverseResults = query.Limit > 0
	}
	sqlQuery = store.AppendWhere(sqlQuery, where)
	if reverseResults {
		sqlQuery += " ORDER BY sequence DESC"
	} else {
		sqlQuery += " ORDER BY sequence ASC"
	}
	sqlQuery, args = store.AppendLimit(sqlQuery, args, query.Limit)

	rows, err := g.db.QueryContext(ctx, sqlQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("store: query network conversation messages: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			closeErr = fmt.Errorf("store: close network conversation message rows: %w", closeErr)
			if err != nil {
				err = errors.Join(err, closeErr)
				return
			}
			err = closeErr
		}
	}()

	entries, err = loadNetworkMessageEntries(rows)
	if err != nil {
		return nil, err
	}
	if reverseResults {
		reverseNetworkMessages(entries)
	}
	return entries, nil
}

// GetWork returns one network work row by workspace_id and work_id.
func (g *NetworkRepo) GetWork(ctx context.Context, workspaceID string, workID string) (store.NetworkWorkEntry, error) {
	if err := g.checkReady(ctx, "get network work"); err != nil {
		return store.NetworkWorkEntry{}, err
	}
	trimmedWorkspaceID, err := normalizeRequiredNetworkField(workspaceID, "network work workspace_id")
	if err != nil {
		return store.NetworkWorkEntry{}, err
	}
	trimmed := strings.TrimSpace(workID)
	if err := validateNetworkWorkID(trimmed); err != nil {
		return store.NetworkWorkEntry{}, err
	}
	return getNetworkWorkWithExecutor(ctx, g.db, trimmedWorkspaceID, trimmed)
}
