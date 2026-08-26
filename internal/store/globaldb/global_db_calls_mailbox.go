package globaldb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	callspkg "github.com/compozy/compozy/internal/calls"
	"github.com/compozy/compozy/internal/store"
)

type messageSession struct {
	id, profileID, workspaceID, state, parentID, agentName string
	parkedAt, idleExpiresAt                                sql.NullString
	drainingAt                                             sql.NullString
	pendingPermission                                      int
}

func (g *CallRepo) AcceptMessage(
	ctx context.Context,
	admission callspkg.MessageAdmission,
) (record callspkg.MessageRecord, err error) {
	if err := g.checkReady(ctx, "accept call message"); err != nil {
		return callspkg.MessageRecord{}, err
	}
	err = g.tasks.withTaskImmediateTransaction(ctx, "accept call message", func(exec taskSQLExecutor) error {
		from, target, resolveErr := resolveMessageEdge(ctx, exec, admission)
		if resolveErr != nil {
			return resolveErr
		}
		if target.pendingPermission > 0 {
			return &callspkg.Error{
				Code:    callspkg.CodeMessageTargetBlocked,
				Message: "target is awaiting a human decision; use the pending decision surface",
			}
		}
		if err := enforceMessageLoopBreakers(ctx, exec, target.id, admission); err != nil {
			return err
		}
		nowText := store.FormatTimestamp(admission.Record.CreatedAt)
		deliveryID := "delivery_" + strings.TrimPrefix(admission.Record.MessageID, "msg_")
		wakeID := "wake_" + strings.TrimPrefix(admission.Record.MessageID, "msg_")
		callID := nullableTaskString(admission.Record.CallID)
		_, err := exec.ExecContext(ctx, `INSERT INTO call_messages (
			message_id, profile_id, scope, workspace_id, from_kind, from_id,
			to_session_id, call_id, body, dedup_hash, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			admission.Record.MessageID, admission.Record.ProfileID, string(admission.Record.Scope),
			admission.Record.WorkspaceID, admission.Record.From.Kind, admission.Record.From.ID,
			target.id, callID, admission.Record.Body, admission.Record.DedupHash, nowText,
		)
		if err != nil {
			return fmt.Errorf("store: insert call message %q: %w", admission.Record.MessageID, err)
		}
		state, reason := "pending", ""
		if target.state == "stopped" && !target.parkedAt.Valid {
			state, reason = "failed", "target_stopped"
		}
		ownerKey := "session:" + target.id
		if admission.Record.From.Kind == "session" {
			ownerKey = "session:" + from.id
		}
		_, err = exec.ExecContext(ctx, `INSERT INTO call_deliveries (
			delivery_id, kind, subject_id, recipient_session_id, owner_key, wake_event_id,
			state, reason, created_at, updated_at
		) VALUES (?, 'message', ?, ?, ?, ?, ?, ?, ?, ?)`,
			deliveryID, admission.Record.MessageID, target.id, ownerKey, wakeID,
			state, reason, nowText, nowText,
		)
		if err != nil {
			return fmt.Errorf("store: insert message delivery %q: %w", deliveryID, err)
		}
		if target.parkedAt.Valid {
			if _, err := exec.ExecContext(ctx, `UPDATE sessions
				SET idle_expires_at = NULL, updated_at = ? WHERE id = ?`, nowText, target.id); err != nil {
				return fmt.Errorf("store: clear message target idle clock %q: %w", target.id, err)
			}
		}
		record = admission.Record
		record.ToSessionID = target.id
		record.FromAgentName = from.agentName
		record.Delivery = projectDeliveryState(state)
		record.DeliveryReason = reason
		return nil
	})
	if err != nil {
		return callspkg.MessageRecord{}, err
	}
	return record, nil
}

func resolveMessageEdge(
	ctx context.Context,
	exec taskSQLExecutor,
	admission callspkg.MessageAdmission,
) (messageSession, messageSession, error) {
	record := admission.Record
	var from messageSession
	if record.From.Kind == "session" {
		loaded, err := loadMessageSession(ctx, exec, record.From.ID)
		if err != nil {
			return messageSession{}, messageSession{}, messageTargetError(err, "sender")
		}
		from = loaded
		if from.profileID != record.ProfileID {
			return messageSession{}, messageSession{}, &callspkg.Error{
				Code: callspkg.CodeTargetDenied, Message: "sender belongs to another profile",
			}
		}
		if from.workspaceID != record.WorkspaceID {
			return messageSession{}, messageSession{}, &callspkg.Error{
				Code: callspkg.CodeWorkspaceDenied, Message: "sender belongs to another workspace",
			}
		}
	}
	targetID := strings.TrimSpace(admission.Target)
	if targetID == "parent" {
		if record.From.Kind != "session" || from.parentID == "" {
			return messageSession{}, messageSession{}, &callspkg.Error{
				Code: callspkg.CodeMessageTargetDenied, Message: "sender has no parent target",
			}
		}
		targetID = from.parentID
	}
	target, err := loadMessageSession(ctx, exec, targetID)
	if err != nil {
		return messageSession{}, messageSession{}, messageTargetError(err, "target")
	}
	if target.drainingAt.Valid {
		return messageSession{}, messageSession{}, &callspkg.Error{
			Code: callspkg.CodeMessageTargetDenied, Message: "target is draining",
		}
	}
	if target.profileID != record.ProfileID {
		return messageSession{}, messageSession{}, &callspkg.Error{
			Code: callspkg.CodeTargetDenied, Message: "target belongs to another profile",
		}
	}
	if target.workspaceID != record.WorkspaceID {
		return messageSession{}, messageSession{}, &callspkg.Error{
			Code: callspkg.CodeWorkspaceDenied, Message: "target belongs to another workspace",
		}
	}
	if record.From.Kind == "session" && target.id != from.parentID && target.parentID != from.id {
		return messageSession{}, messageSession{}, &callspkg.Error{
			Code: callspkg.CodeMessageTargetDenied, Message: "target is outside the sender lineage edge",
		}
	}
	if record.CallID != "" {
		var marker int
		err := exec.QueryRowContext(ctx, `SELECT 1 FROM calls
			WHERE call_id = ? AND profile_id = ? AND scope = ? AND workspace_id = ?`,
			record.CallID, record.ProfileID, string(record.Scope), record.WorkspaceID).Scan(&marker)
		if err != nil {
			return messageSession{}, messageSession{}, messageTargetError(err, "call")
		}
	}
	return from, target, nil
}

func loadMessageSession(ctx context.Context, exec taskSQLExecutor, sessionID string) (messageSession, error) {
	var item messageSession
	err := exec.QueryRowContext(ctx, `SELECT id, profile_id, workspace_id, state,
		COALESCE(parent_session_id, ''), agent_name, parked_at, idle_expires_at,
		pending_permission_count, draining_at FROM sessions WHERE id = ?`, strings.TrimSpace(sessionID)).Scan(
		&item.id, &item.profileID, &item.workspaceID, &item.state, &item.parentID,
		&item.agentName, &item.parkedAt, &item.idleExpiresAt, &item.pendingPermission, &item.drainingAt,
	)
	return item, err
}

func messageTargetError(err error, subject string) error {
	if errors.Is(err, sql.ErrNoRows) {
		return &callspkg.Error{Code: callspkg.CodeMessageTargetDenied, Message: subject + " was not found"}
	}
	return fmt.Errorf("store: inspect message %s: %w", subject, err)
}

func enforceMessageLoopBreakers(
	ctx context.Context,
	exec taskSQLExecutor,
	recipientSessionID string,
	admission callspkg.MessageAdmission,
) error {
	record := admission.Record
	minuteAgo := store.FormatTimestamp(record.CreatedAt.Add(-time.Minute))
	var sent int
	if err := exec.QueryRowContext(ctx, `SELECT COUNT(1) FROM call_messages
		WHERE from_kind = ? AND from_id = ? AND created_at >= ?`,
		record.From.Kind, record.From.ID, minuteAgo).Scan(&sent); err != nil {
		return fmt.Errorf("store: count recent call messages: %w", err)
	}
	if sent >= admission.RateLimit {
		return &callspkg.Error{
			Code:    callspkg.CodeMessageRateLimited,
			Message: fmt.Sprintf("sender exceeded %d messages per minute", admission.RateLimit),
			ResetAt: store.FormatTimestamp(record.CreatedAt.Add(time.Minute)),
		}
	}
	windowStart := store.FormatTimestamp(record.CreatedAt.Add(-admission.DedupWindow))
	var originalID string
	err := exec.QueryRowContext(ctx, `SELECT message_id FROM call_messages
		WHERE from_kind = ? AND from_id = ? AND dedup_hash = ? AND created_at >= ?
		ORDER BY created_at DESC, message_id DESC LIMIT 1`,
		record.From.Kind, record.From.ID, record.DedupHash, windowStart).Scan(&originalID)
	if err == nil {
		return &callspkg.Error{
			Code: callspkg.CodeMessageDuplicate, Message: "identical message is inside the deduplication window",
			OriginalID: originalID,
		}
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("store: inspect duplicate call message: %w", err)
	}
	var pending int
	if err := exec.QueryRowContext(ctx, `SELECT COUNT(1)
		FROM call_deliveries delivery
		WHERE delivery.kind = 'message' AND delivery.state = 'pending'
		AND delivery.recipient_session_id = ?`, recipientSessionID).Scan(&pending); err != nil {
		return fmt.Errorf("store: count pending call messages: %w", err)
	}
	if pending >= admission.PendingCap {
		return &callspkg.Error{
			Code:    callspkg.CodeMessagePendingCap,
			Message: fmt.Sprintf("recipient has %d queued messages; maximum is %d", pending, admission.PendingCap),
		}
	}
	return nil
}

func (g *CallRepo) GetMessage(
	ctx context.Context,
	scope callspkg.CallScope,
	messageID string,
) (callspkg.MessageRecord, error) {
	if err := g.checkReady(ctx, "get call message"); err != nil {
		return callspkg.MessageRecord{}, err
	}
	query := `SELECT message.message_id, message.profile_id, message.scope, message.workspace_id,
		message.from_kind, message.from_id, COALESCE(sender.agent_name, ''), message.to_session_id,
		COALESCE(message.call_id, ''), message.body, message.dedup_hash, message.created_at,
		delivery.state, delivery.reason, delivery.attempts, delivery.delivered_at
		FROM call_messages message
		JOIN call_deliveries delivery ON delivery.kind = 'message' AND delivery.subject_id = message.message_id
		LEFT JOIN sessions sender ON message.from_kind = 'session' AND sender.id = message.from_id
		WHERE message.message_id = ?`
	args := []any{strings.TrimSpace(messageID)}
	if strings.TrimSpace(scope.ProfileID) != "" {
		query += ` AND message.profile_id = ? AND message.scope = ? AND message.workspace_id = ?`
		args = append(args, scope.ProfileID, string(scope.Scope), scope.WorkspaceID)
	}
	return scanCallMessage(g.db.QueryRowContext(ctx, query, args...))
}

func scanCallMessage(scanner rowScanner) (callspkg.MessageRecord, error) {
	var record callspkg.MessageRecord
	var scope, state, createdAt string
	var deliveredAt sql.NullString
	if err := scanner.Scan(
		&record.MessageID, &record.ProfileID, &scope, &record.WorkspaceID,
		&record.From.Kind, &record.From.ID, &record.FromAgentName, &record.ToSessionID,
		&record.CallID, &record.Body, &record.DedupHash, &createdAt,
		&state, &record.DeliveryReason, &record.DeliveryAttempts, &deliveredAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return callspkg.MessageRecord{}, &callspkg.Error{
				Code: callspkg.CodeMessageNotFound, Message: "message was not found",
			}
		}
		return callspkg.MessageRecord{}, fmt.Errorf("store: scan call message: %w", err)
	}
	record.Scope = callspkg.Scope(scope)
	record.Delivery = projectDeliveryState(state)
	created, err := store.ParseTimestamp(createdAt)
	if err != nil {
		return callspkg.MessageRecord{}, fmt.Errorf("store: parse message created_at: %w", err)
	}
	record.CreatedAt = created
	if deliveredAt.Valid {
		delivered, err := store.ParseTimestamp(deliveredAt.String)
		if err != nil {
			return callspkg.MessageRecord{}, fmt.Errorf("store: parse message delivered_at: %w", err)
		}
		record.DeliveredAt = delivered
	}
	return record, nil
}

func projectDeliveryState(state string) string {
	switch state {
	case "pending":
		return "queued"
	case "injected":
		return "delivered-into-turn"
	case "woken":
		return "woke"
	case "failed":
		return "failed"
	default:
		return "failed"
	}
}
