package globaldb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/compozy/agh/internal/store"
)

// ListThreads returns one counted, bounded page of public-thread summaries.
func (g *NetworkRepo) ListThreads(
	ctx context.Context,
	ref store.NetworkChannelRef,
	query store.NetworkThreadQuery,
) (page store.NetworkThreadPage, err error) {
	if err := g.checkReady(ctx, "list network threads"); err != nil {
		return store.NetworkThreadPage{}, err
	}
	ref = normalizeNetworkChannelRef(ref)
	if err := ref.Validate(); err != nil {
		return store.NetworkThreadPage{}, err
	}
	if err := query.Validate(); err != nil {
		return store.NetworkThreadPage{}, fmt.Errorf("store: validate network thread query: %w", err)
	}
	query = query.Normalize()
	page.Limit = query.Limit

	err = g.withNetworkReadTransaction(ctx, "list network threads", func(exec networkSQLExecutor) error {
		total, countErr := countNetworkThreads(ctx, exec, ref, query)
		if countErr != nil {
			return countErr
		}
		page.Total = total

		statement, args, buildErr := buildNetworkThreadListQuery(
			ctx,
			exec,
			networkThreadSummarySelect(),
			ref,
			query,
			query.Limit+1,
		)
		if buildErr != nil {
			return buildErr
		}
		threads, queryErr := queryNetworkThreadSummaries(ctx, exec, statement, args)
		if queryErr != nil {
			return queryErr
		}
		page.Threads = threads
		if len(page.Threads) > page.Limit {
			page.Threads = page.Threads[:page.Limit]
			page.HasMore = true
			cursor, cursorErr := encodeNetworkThreadCursor(
				ctx,
				exec,
				ref,
				query,
				page.Threads[len(page.Threads)-1],
			)
			if cursorErr != nil {
				return cursorErr
			}
			page.NextCursor = cursor
		}
		return nil
	})
	if err != nil {
		return store.NetworkThreadPage{}, err
	}
	return page, nil
}

// ListDirectRooms returns one counted, bounded page of direct-room summaries.
func (g *NetworkRepo) ListDirectRooms(
	ctx context.Context,
	ref store.NetworkChannelRef,
	query store.NetworkDirectRoomQuery,
) (page store.NetworkDirectRoomPage, err error) {
	if err := g.checkReady(ctx, "list network direct rooms"); err != nil {
		return store.NetworkDirectRoomPage{}, err
	}
	ref = normalizeNetworkChannelRef(ref)
	if err := ref.Validate(); err != nil {
		return store.NetworkDirectRoomPage{}, err
	}
	if err := query.Validate(); err != nil {
		return store.NetworkDirectRoomPage{}, fmt.Errorf("store: validate network direct room query: %w", err)
	}
	query = query.Normalize()
	page.Limit = query.Limit

	err = g.withNetworkReadTransaction(ctx, "list network direct rooms", func(exec networkSQLExecutor) error {
		total, countErr := countNetworkDirectRooms(ctx, exec, ref, query)
		if countErr != nil {
			return countErr
		}
		page.Total = total

		statement, args, buildErr := buildNetworkDirectRoomListQuery(
			ctx,
			exec,
			networkDirectRoomSummarySelect(),
			ref,
			query,
			query.Limit+1,
		)
		if buildErr != nil {
			return buildErr
		}
		directs, queryErr := queryNetworkDirectRoomSummaries(ctx, exec, statement, args)
		if queryErr != nil {
			return queryErr
		}
		page.Directs = directs
		if len(page.Directs) > page.Limit {
			page.Directs = page.Directs[:page.Limit]
			page.HasMore = true
			cursor, cursorErr := encodeNetworkDirectRoomCursor(
				ctx,
				exec,
				ref,
				query,
				page.Directs[len(page.Directs)-1],
			)
			if cursorErr != nil {
				return cursorErr
			}
			page.NextCursor = cursor
		}
		return nil
	})
	if err != nil {
		return store.NetworkDirectRoomPage{}, err
	}
	return page, nil
}

func countNetworkThreads(
	ctx context.Context,
	exec networkSQLExecutor,
	ref store.NetworkChannelRef,
	query store.NetworkThreadQuery,
) (int, error) {
	// dynamic-sql: optional peer/title/work filters and cursor scope change the count predicate set.
	where, args := networkThreadListFilterClauses(ref, query)
	var total int
	if err := exec.QueryRowContext(
		ctx,
		store.AppendWhere("SELECT COUNT(*) FROM network_threads", where),
		args...,
	).Scan(&total); err != nil {
		return 0, fmt.Errorf("store: count network threads: %w", err)
	}
	return total, nil
}

func countNetworkDirectRooms(
	ctx context.Context,
	exec networkSQLExecutor,
	ref store.NetworkChannelRef,
	query store.NetworkDirectRoomQuery,
) (int, error) {
	// dynamic-sql: optional peer/work filters and cursor scope change the count predicate set.
	where, args := networkDirectRoomListFilterClauses(ref, query)
	var total int
	if err := exec.QueryRowContext(
		ctx,
		store.AppendWhere("SELECT COUNT(*) FROM network_direct_rooms", where),
		args...,
	).Scan(&total); err != nil {
		return 0, fmt.Errorf("store: count network direct rooms: %w", err)
	}
	return total, nil
}

func queryNetworkThreadSummaries(
	ctx context.Context,
	exec networkSQLExecutor,
	statement string,
	args []any,
) (summaries []store.NetworkThreadSummary, err error) {
	// dynamic-sql: statement is the keyset page query built from optional filters and cursor direction.
	rows, err := exec.QueryContext(ctx, statement, args...)
	if err != nil {
		return nil, fmt.Errorf("store: query network threads: %w", err)
	}
	defer closeNetworkPageRows(rows, "network thread", &err)
	return scanNetworkThreadSummaries(rows)
}

func queryNetworkDirectRoomSummaries(
	ctx context.Context,
	exec networkSQLExecutor,
	statement string,
	args []any,
) (summaries []store.NetworkDirectRoomSummary, err error) {
	// dynamic-sql: statement is the keyset page query built from optional filters and cursor direction.
	rows, err := exec.QueryContext(ctx, statement, args...)
	if err != nil {
		return nil, fmt.Errorf("store: query network direct rooms: %w", err)
	}
	defer closeNetworkPageRows(rows, "network direct room", &err)
	return scanNetworkDirectRoomSummaries(rows)
}

func closeNetworkPageRows(rows *sql.Rows, label string, target *error) {
	if rows == nil {
		return
	}
	if err := rows.Close(); err != nil {
		closeErr := fmt.Errorf("store: close %s rows: %w", label, err)
		if *target == nil {
			*target = closeErr
			return
		}
		*target = errors.Join(*target, closeErr)
	}
}

func networkThreadSummarySelect() string {
	return `SELECT
		workspace_id, channel, thread_id, root_message_id, title, opened_by_peer_id, opened_session_id,
		opened_at, opened_sequence, last_activity_at, last_activity_sequence,
		message_count, participant_count, open_work_count,
		COALESCE((
			SELECT SUM(stats.delivered_count)
			FROM network_thread_session_token_stats AS stats
			WHERE stats.workspace_id = network_threads.workspace_id
				AND stats.channel = network_threads.channel
				AND stats.thread_id = network_threads.thread_id
		), 0),
		COALESCE((
			SELECT SUM(stats.prompt_size_bytes)
			FROM network_thread_session_token_stats AS stats
			WHERE stats.workspace_id = network_threads.workspace_id
				AND stats.channel = network_threads.channel
				AND stats.thread_id = network_threads.thread_id
		), 0),
		COALESCE((
			SELECT SUM(stats.estimated_prompt_tokens)
			FROM network_thread_session_token_stats AS stats
			WHERE stats.workspace_id = network_threads.workspace_id
				AND stats.channel = network_threads.channel
				AND stats.thread_id = network_threads.thread_id
		), 0),
		last_message_preview
	FROM network_threads`
}

func networkDirectRoomSummarySelect() string {
	return `SELECT
		workspace_id, channel, direct_id, session_a, session_b,
		opened_at, opened_sequence, last_activity_at, last_activity_sequence,
		message_count, open_work_count, last_message_preview
	FROM network_direct_rooms`
}
