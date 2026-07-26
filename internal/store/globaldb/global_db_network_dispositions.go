package globaldb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/compozy/agh/internal/store"
)

func insertNetworkMessageDispositions(
	ctx context.Context,
	exec networkSQLExecutor,
	message store.NetworkConversationMessage,
	dispositions []store.NetworkMessageDisposition,
	acceptanceSeq int64,
	decidedAt time.Time,
) error {
	// dynamic-sql: recipient ownership must be checked in the acceptance transaction.
	const sessionQuery = `SELECT 1 FROM sessions WHERE workspace_id = ? AND id = ?`
	// dynamic-sql: immutable acceptance evidence is inserted from the pre-message snapshot.
	const insertQuery = `
INSERT INTO network_message_dispositions (
  workspace_id, message_id, recipient_session_id, decision, decided_at, acceptance_seq
) VALUES (?, ?, ?, ?, ?, ?)`
	for _, disposition := range dispositions {
		var exists int
		if err := exec.QueryRowContext(
			ctx,
			sessionQuery,
			message.WorkspaceID,
			disposition.RecipientSessionID,
		).Scan(&exists); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return fmt.Errorf(
					"store: network disposition recipient %q does not belong to workspace %q",
					disposition.RecipientSessionID,
					message.WorkspaceID,
				)
			}
			return fmt.Errorf("store: verify network disposition recipient: %w", err)
		}
		if _, err := exec.ExecContext(
			ctx,
			insertQuery,
			message.WorkspaceID,
			message.MessageID,
			disposition.RecipientSessionID,
			disposition.Decision,
			store.FormatTimestamp(decidedAt),
			acceptanceSeq,
		); err != nil {
			return fmt.Errorf("store: insert network message disposition: %w", err)
		}
	}
	return nil
}

func listNetworkMessageDispositions(
	ctx context.Context,
	exec networkSQLExecutor,
	workspaceID string,
	messageID string,
) (dispositions []store.NetworkMessageDisposition, err error) {
	// dynamic-sql: duplicate acceptance reloads only immutable evidence for one message.
	const query = `
SELECT recipient_session_id, decision
FROM network_message_dispositions
WHERE workspace_id = ? AND message_id = ?
ORDER BY recipient_session_id`
	rows, err := exec.QueryContext(ctx, query, workspaceID, messageID)
	if err != nil {
		return nil, fmt.Errorf("store: list network message dispositions: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			closeErr = fmt.Errorf("store: close network message disposition rows: %w", closeErr)
			if err == nil {
				err = closeErr
				return
			}
			err = errors.Join(err, closeErr)
		}
	}()
	for rows.Next() {
		var disposition store.NetworkMessageDisposition
		if scanErr := rows.Scan(&disposition.RecipientSessionID, &disposition.Decision); scanErr != nil {
			return nil, fmt.Errorf("store: scan network message disposition: %w", scanErr)
		}
		dispositions = append(dispositions, disposition)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: iterate network message dispositions: %w", err)
	}
	return dispositions, nil
}
