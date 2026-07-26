package globaldb

import (
	"context"
	"fmt"
	"strings"

	"github.com/compozy/agh/internal/listcursor"
	"github.com/compozy/agh/internal/store"
)

const (
	networkConversationCursorVersion = 3
	networkThreadCursorKind          = "network_threads"
	networkDirectRoomCursorKind      = "network_direct_rooms"
)

type networkThreadCursor struct {
	ThreadID        string `json:"thread_id"`
	Title           string `json:"title"`
	AnchorMessageID string `json:"anchor_message_id,omitempty"`
	Sequence        int64  `json:"-"`
}

type networkDirectRoomCursor struct {
	DirectID        string `json:"direct_id"`
	SessionA        string `json:"session_a"`
	SessionB        string `json:"session_b"`
	OpenedAt        string `json:"opened_at,omitempty"`
	AnchorMessageID string `json:"anchor_message_id,omitempty"`
	EmptyAnchor     bool   `json:"empty_anchor,omitempty"`
	Sequence        int64  `json:"-"`
}

func buildNetworkThreadListQuery(
	ctx context.Context,
	exec networkSQLExecutor,
	statement string,
	ref store.NetworkChannelRef,
	query store.NetworkThreadQuery,
	fetchLimit int,
) (string, []any, error) {
	where, args := networkThreadListFilterClauses(ref, query)
	if after := strings.TrimSpace(query.After); after != "" {
		cursor, err := decodeNetworkThreadCursor(ctx, exec, ref, query, after)
		if err != nil {
			return "", nil, err
		}
		clause, cursorArgs := networkThreadCursorClause(cursor, query.Sort)
		where = append(where, clause)
		args = append(args, cursorArgs...)
	}
	statement = store.AppendWhere(statement, where)
	statement += networkThreadOrderBy(query.Sort)
	statement, args = store.AppendLimit(statement, args, fetchLimit)
	return statement, args, nil
}

func buildNetworkDirectRoomListQuery(
	ctx context.Context,
	exec networkSQLExecutor,
	statement string,
	ref store.NetworkChannelRef,
	query store.NetworkDirectRoomQuery,
	fetchLimit int,
) (string, []any, error) {
	where, args := networkDirectRoomListFilterClauses(ref, query)
	if after := strings.TrimSpace(query.After); after != "" {
		cursor, err := decodeNetworkDirectRoomCursor(ctx, exec, ref, query, after)
		if err != nil {
			return "", nil, err
		}
		clause, cursorArgs := networkDirectRoomCursorClause(cursor, query.Sort)
		where = append(where, clause)
		args = append(args, cursorArgs...)
	}
	statement = store.AppendWhere(statement, where)
	statement += networkDirectRoomOrderBy(query.Sort)
	statement, args = store.AppendLimit(statement, args, fetchLimit)
	return statement, args, nil
}

func networkThreadListFilterClauses(
	ref store.NetworkChannelRef,
	query store.NetworkThreadQuery,
) ([]string, []any) {
	where := []string{globalDBNetworkConversationsWorkspaceIDValue, globalDBNetworkConversationsChannelValue}
	args := []any{ref.WorkspaceID, ref.Channel}
	if search := strings.TrimSpace(query.Search); search != "" {
		pattern := "%" + escapeNetworkLikePattern(search) + "%"
		where = append(where, "(title LIKE ? ESCAPE '\\' OR last_message_preview LIKE ? ESCAPE '\\')")
		args = append(args, pattern, pattern)
	}
	if sessionID := strings.TrimSpace(query.SessionID); sessionID != "" {
		where = append(where, `EXISTS (
			SELECT 1 FROM network_thread_participants AS participant
			WHERE participant.workspace_id = network_threads.workspace_id
				AND participant.channel = network_threads.channel
				AND participant.thread_id = network_threads.thread_id
				AND participant.session_id = ?
		)`)
		args = append(args, sessionID)
	}
	if query.HasWork != nil {
		operator := "= 0"
		if *query.HasWork {
			operator = "> 0"
		}
		where = append(where, "open_work_count "+operator)
	}
	return where, args
}

func networkDirectRoomListFilterClauses(
	ref store.NetworkChannelRef,
	query store.NetworkDirectRoomQuery,
) ([]string, []any) {
	where := []string{globalDBNetworkConversationsWorkspaceIDValue, globalDBNetworkConversationsChannelValue}
	args := []any{ref.WorkspaceID, ref.Channel}
	if search := strings.TrimSpace(query.Search); search != "" {
		pattern := "%" + escapeNetworkLikePattern(search) + "%"
		where = append(
			where,
			"(session_a LIKE ? ESCAPE '\\' OR session_b LIKE ? ESCAPE '\\' OR last_message_preview LIKE ? ESCAPE '\\')",
		)
		args = append(args, pattern, pattern, pattern)
	}
	if sessionID := strings.TrimSpace(query.SessionID); sessionID != "" {
		where = append(where, "(session_a = ? OR session_b = ?)")
		args = append(args, sessionID, sessionID)
	}
	if query.HasWork != nil {
		operator := "= 0"
		if *query.HasWork {
			operator = "> 0"
		}
		where = append(where, "open_work_count "+operator)
	}
	return where, args
}

func networkThreadCursorClause(cursor networkThreadCursor, sort string) (string, []any) {
	switch store.NormalizeNetworkConversationSort(sort) {
	case store.NetworkConversationSortCreated:
		value := cursor.Sequence
		return "(opened_sequence > ? OR (opened_sequence = ? AND thread_id > ?))", []any{value, value, cursor.ThreadID}
	case store.NetworkConversationSortAlphabetical:
		return `(title COLLATE NOCASE > ? COLLATE NOCASE
			OR (title COLLATE NOCASE = ? COLLATE NOCASE AND thread_id > ?))`,
			[]any{cursor.Title, cursor.Title, cursor.ThreadID}
	default:
		value := cursor.Sequence
		return "(last_activity_sequence < ? OR (last_activity_sequence = ? AND thread_id > ?))", []any{
			value,
			value,
			cursor.ThreadID,
		}
	}
}

func networkDirectRoomCursorClause(cursor networkDirectRoomCursor, sort string) (string, []any) {
	switch store.NormalizeNetworkConversationSort(sort) {
	case store.NetworkConversationSortCreated:
		return "(opened_at > ? OR (opened_at = ? AND direct_id > ?))", []any{
			cursor.OpenedAt,
			cursor.OpenedAt,
			cursor.DirectID,
		}
	case store.NetworkConversationSortAlphabetical:
		return `(session_a COLLATE NOCASE > ? COLLATE NOCASE
			OR (session_a COLLATE NOCASE = ? COLLATE NOCASE AND (
				session_b COLLATE NOCASE > ? COLLATE NOCASE
				OR (session_b COLLATE NOCASE = ? COLLATE NOCASE AND direct_id > ?)
			)))`, []any{cursor.SessionA, cursor.SessionA, cursor.SessionB, cursor.SessionB, cursor.DirectID}
	default:
		value := cursor.Sequence
		return "(last_activity_sequence < ? OR (last_activity_sequence = ? AND direct_id > ?))", []any{
			value,
			value,
			cursor.DirectID,
		}
	}
}

func networkThreadOrderBy(sort string) string {
	switch store.NormalizeNetworkConversationSort(sort) {
	case store.NetworkConversationSortCreated:
		return " ORDER BY opened_sequence ASC, thread_id ASC"
	case store.NetworkConversationSortAlphabetical:
		return " ORDER BY title COLLATE NOCASE ASC, thread_id ASC"
	default:
		return " ORDER BY last_activity_sequence DESC, thread_id ASC"
	}
}

func networkDirectRoomOrderBy(sort string) string {
	switch store.NormalizeNetworkConversationSort(sort) {
	case store.NetworkConversationSortCreated:
		return " ORDER BY opened_at ASC, direct_id ASC"
	case store.NetworkConversationSortAlphabetical:
		return " ORDER BY session_a COLLATE NOCASE ASC, session_b COLLATE NOCASE ASC, direct_id ASC"
	default:
		return " ORDER BY last_activity_sequence DESC, direct_id ASC"
	}
}

func escapeNetworkLikePattern(value string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(value)
}

func networkConversationCursorFingerprint(
	ref store.NetworkChannelRef,
	search string,
	sessionID string,
	sortOrder string,
	hasWork *bool,
	limit int,
) (string, error) {
	return listcursor.Fingerprint(struct {
		WorkspaceID string `json:"workspace_id"`
		Channel     string `json:"channel"`
		Search      string `json:"query"`
		SessionID   string `json:"session_id"`
		Sort        string `json:"sort"`
		HasWork     *bool  `json:"has_work"`
		Limit       int    `json:"limit"`
	}{
		WorkspaceID: ref.WorkspaceID,
		Channel:     ref.Channel,
		Search:      search,
		SessionID:   sessionID,
		Sort:        sortOrder,
		HasWork:     hasWork,
		Limit:       limit,
	})
}

func decodeNetworkThreadCursor(
	ctx context.Context,
	exec networkSQLExecutor,
	ref store.NetworkChannelRef,
	query store.NetworkThreadQuery,
	raw string,
) (networkThreadCursor, error) {
	fingerprint, err := networkConversationCursorFingerprint(
		ref,
		query.Search,
		query.SessionID,
		query.Sort,
		query.HasWork,
		query.Limit,
	)
	if err != nil {
		return networkThreadCursor{}, fmt.Errorf("store: fingerprint network thread cursor: %w", err)
	}
	cursor, err := listcursor.Decode[networkThreadCursor](
		raw,
		networkConversationCursorVersion,
		networkThreadCursorKind,
		fingerprint,
		listcursor.DefaultMaxEncodedSize,
	)
	if err != nil {
		return networkThreadCursor{}, fmt.Errorf("%w: %w", store.ErrNetworkCursorInvalid, err)
	}
	if strings.TrimSpace(cursor.ThreadID) == "" {
		return networkThreadCursor{}, fmt.Errorf(
			"%w: thread cursor position is incomplete",
			store.ErrNetworkCursorInvalid,
		)
	}
	if store.NormalizeNetworkConversationSort(query.Sort) != store.NetworkConversationSortAlphabetical {
		if strings.TrimSpace(cursor.AnchorMessageID) == "" {
			return networkThreadCursor{}, fmt.Errorf(
				"%w: thread cursor causal anchor is incomplete",
				store.ErrNetworkCursorInvalid,
			)
		}
		cursor.Sequence, err = lookupNetworkConversationCursorSequence(
			ctx,
			exec,
			ref,
			store.NetworkSurfaceThread,
			cursor.ThreadID,
			cursor.AnchorMessageID,
		)
		if err != nil {
			return networkThreadCursor{}, err
		}
	}
	return cursor, nil
}

func decodeNetworkDirectRoomCursor(
	ctx context.Context,
	exec networkSQLExecutor,
	ref store.NetworkChannelRef,
	query store.NetworkDirectRoomQuery,
	raw string,
) (networkDirectRoomCursor, error) {
	fingerprint, err := networkConversationCursorFingerprint(
		ref,
		query.Search,
		query.SessionID,
		query.Sort,
		query.HasWork,
		query.Limit,
	)
	if err != nil {
		return networkDirectRoomCursor{}, fmt.Errorf("store: fingerprint network direct cursor: %w", err)
	}
	cursor, err := listcursor.Decode[networkDirectRoomCursor](
		raw,
		networkConversationCursorVersion,
		networkDirectRoomCursorKind,
		fingerprint,
		listcursor.DefaultMaxEncodedSize,
	)
	if err != nil {
		return networkDirectRoomCursor{}, fmt.Errorf("%w: %w", store.ErrNetworkCursorInvalid, err)
	}
	if strings.TrimSpace(cursor.DirectID) == "" {
		return networkDirectRoomCursor{}, fmt.Errorf(
			"%w: direct cursor position is incomplete",
			store.ErrNetworkCursorInvalid,
		)
	}
	switch store.NormalizeNetworkConversationSort(query.Sort) {
	case store.NetworkConversationSortCreated:
		openedAt, parseErr := store.ParseTimestamp(cursor.OpenedAt)
		if parseErr != nil {
			return networkDirectRoomCursor{}, fmt.Errorf(
				"%w: direct cursor opened_at is invalid: %w",
				store.ErrNetworkCursorInvalid,
				parseErr,
			)
		}
		cursor.OpenedAt = store.FormatTimestamp(openedAt)
	case store.NetworkConversationSortAlphabetical:
	default:
		switch {
		case cursor.EmptyAnchor && strings.TrimSpace(cursor.AnchorMessageID) == "":
			cursor.Sequence = 0
		case !cursor.EmptyAnchor && strings.TrimSpace(cursor.AnchorMessageID) != "":
			cursor.Sequence, err = lookupNetworkConversationCursorSequence(
				ctx,
				exec,
				ref,
				store.NetworkSurfaceDirect,
				cursor.DirectID,
				cursor.AnchorMessageID,
			)
			if err != nil {
				return networkDirectRoomCursor{}, err
			}
		default:
			return networkDirectRoomCursor{}, fmt.Errorf(
				"%w: direct cursor causal anchor is incomplete",
				store.ErrNetworkCursorInvalid,
			)
		}
	}
	return cursor, nil
}

func encodeNetworkThreadCursor(
	ctx context.Context,
	exec networkSQLExecutor,
	ref store.NetworkChannelRef,
	query store.NetworkThreadQuery,
	thread store.NetworkThreadSummary,
) (string, error) {
	fingerprint, err := networkConversationCursorFingerprint(
		ref,
		query.Search,
		query.SessionID,
		query.Sort,
		query.HasWork,
		query.Limit,
	)
	if err != nil {
		return "", fmt.Errorf("store: fingerprint network thread cursor: %w", err)
	}
	cursor := networkThreadCursor{ThreadID: thread.ThreadID, Title: thread.Title}
	if store.NormalizeNetworkConversationSort(query.Sort) != store.NetworkConversationSortAlphabetical {
		sequence := thread.LastActivitySequence
		if store.NormalizeNetworkConversationSort(query.Sort) == store.NetworkConversationSortCreated {
			sequence = thread.OpenedSequence
		}
		cursor.AnchorMessageID, err = lookupNetworkConversationCursorMessageID(
			ctx,
			exec,
			ref,
			store.NetworkSurfaceThread,
			thread.ThreadID,
			sequence,
		)
		if err != nil {
			return "", err
		}
	}
	encoded, err := listcursor.Encode(
		networkConversationCursorVersion,
		networkThreadCursorKind,
		fingerprint,
		cursor,
	)
	if err != nil {
		return "", fmt.Errorf("store: encode network thread cursor: %w", err)
	}
	return encoded, nil
}

func encodeNetworkDirectRoomCursor(
	ctx context.Context,
	exec networkSQLExecutor,
	ref store.NetworkChannelRef,
	query store.NetworkDirectRoomQuery,
	direct store.NetworkDirectRoomSummary,
) (string, error) {
	fingerprint, err := networkConversationCursorFingerprint(
		ref,
		query.Search,
		query.SessionID,
		query.Sort,
		query.HasWork,
		query.Limit,
	)
	if err != nil {
		return "", fmt.Errorf("store: fingerprint network direct cursor: %w", err)
	}
	cursor := networkDirectRoomCursor{DirectID: direct.DirectID, SessionA: direct.SessionA, SessionB: direct.SessionB}
	switch store.NormalizeNetworkConversationSort(query.Sort) {
	case store.NetworkConversationSortCreated:
		cursor.OpenedAt = store.FormatTimestamp(direct.OpenedAt)
	case store.NetworkConversationSortAlphabetical:
	default:
		sequence := direct.LastActivitySequence
		cursor.EmptyAnchor = sequence == 0
		cursor.AnchorMessageID, err = lookupNetworkConversationCursorMessageID(
			ctx,
			exec,
			ref,
			store.NetworkSurfaceDirect,
			direct.DirectID,
			sequence,
		)
		if err != nil {
			return "", err
		}
	}
	encoded, err := listcursor.Encode(
		networkConversationCursorVersion,
		networkDirectRoomCursorKind,
		fingerprint,
		cursor,
	)
	if err != nil {
		return "", fmt.Errorf("store: encode network direct cursor: %w", err)
	}
	return encoded, nil
}
