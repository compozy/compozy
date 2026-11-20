package globaldb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/agentinstance"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
)

// A warning failure must not undo the usage receipt or reserve a bell that was never delivered.
func warnForAcceptedBudgetReceipt(
	ctx context.Context, tx *store.WriteTx, receipt *sqlcgen.InsertTokenUsageReceiptParams, at time.Time,
) error {
	if receipt.InstanceID == "" {
		return nil
	}
	if _, err := tx.ExecContext(ctx, "SAVEPOINT daily_budget_bell"); err != nil {
		return fmt.Errorf("store: start daily budget bell savepoint: %w", err)
	}
	if err := assessAcceptedBudgetReceiptWarning(ctx, sqlcgen.New(tx), receipt, at); err != nil {
		if store.IsSQLiteBusy(err) {
			return err
		}
		if _, rollbackErr := tx.ExecContext(ctx, "ROLLBACK TO daily_budget_bell"); rollbackErr != nil {
			return errors.Join(err, fmt.Errorf("store: roll back daily budget bell: %w", rollbackErr))
		}
		slog.WarnContext(ctx, "store: daily budget bell could not be recorded",
			"instance_id", receipt.InstanceID, "session_id", receipt.SessionID, "turn_id", receipt.TurnID, "error", err)
	}
	if _, err := tx.ExecContext(ctx, "RELEASE daily_budget_bell"); err != nil {
		return fmt.Errorf("store: release daily budget bell savepoint: %w", err)
	}
	return nil
}

func assessAcceptedBudgetReceiptWarning(
	ctx context.Context, q *sqlcgen.Queries, receipt *sqlcgen.InsertTokenUsageReceiptParams, at time.Time,
) error {
	instance, err := q.GetAgentInstance(ctx, receipt.InstanceID)
	if err != nil {
		return fmt.Errorf("store: resolve budget warning instance: %w", err)
	}
	if instance.ProfileID != receipt.ProfileID {
		return agentinstance.ErrScopeMismatch
	}
	if instance.ArchivedAt.Valid {
		return nil
	}
	return warnForInstanceDailyBudgetUsage(ctx, q, instance, at)
}

func warnForInstanceDailyBudgetUsage(
	ctx context.Context, q *sqlcgen.Queries, instance sqlcgen.AgentInstance, at time.Time,
) error {
	limitsRow, err := q.GetAgentInstanceDailyBudgetLimits(ctx, instance.ID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("store: resolve budget receipt warning limits: %w", err)
	}
	limits, err := decodeInstanceDailyBudgetLimits(limitsRow)
	if err != nil {
		return err
	}
	if limits.Tokens == nil && limits.USD == nil {
		return nil
	}
	// An instance can be starting with no bound main session yet. Preserve its
	// usage receipt without inventing an unrelated recipient for the bell.
	if !instance.MainSessionID.Valid || strings.TrimSpace(instance.MainSessionID.String) == "" {
		return nil
	}
	if instance.WorkspaceID != "" {
		muted, err := q.IsAttentionWorkspaceMuted(ctx, sqlcgen.IsAttentionWorkspaceMutedParams{
			ProfileID: instance.ProfileID, WorkspaceID: instance.WorkspaceID,
		})
		if err != nil {
			return fmt.Errorf("store: read budget receipt warning mute: %w", err)
		}
		if muted {
			return nil
		}
	}
	day := store.LocalDay(at)
	issued, err := q.HasInstanceDailyBudgetNotice(ctx, sqlcgen.HasInstanceDailyBudgetNoticeParams{
		InstanceID: instance.ID, Day: day,
	})
	if err != nil || issued > 0 {
		return err
	}
	usage, err := readDailyBudgetUsage(ctx, q, instance.ProfileID, instance.ID, day)
	if errors.Is(err, errDailyBudgetTotalsInvalid) {
		return nil
	}
	if err != nil {
		return err
	}
	assessment, err := agentinstance.EvaluateDailyBudget(limits, usage, false)
	if err != nil || !assessment.Warn {
		return err
	}
	_, err = reserveDailyBudgetWarningTx(ctx, q, instance, at)
	return err
}

func reserveDailyBudgetWarningTx(
	ctx context.Context, q *sqlcgen.Queries, instance sqlcgen.AgentInstance, at time.Time,
) (bool, error) {
	if instance.WorkspaceID != "" {
		muted, err := q.IsAttentionWorkspaceMuted(ctx, sqlcgen.IsAttentionWorkspaceMutedParams{
			ProfileID: instance.ProfileID, WorkspaceID: instance.WorkspaceID,
		})
		if err != nil {
			return false, fmt.Errorf("store: read daily budget warning mute: %w", err)
		}
		if muted {
			return false, nil
		}
	}
	stamp := store.FormatTimestamp(at)
	rows, err := q.ReserveAgentInstanceDailyBudgetNotice(ctx, sqlcgen.ReserveAgentInstanceDailyBudgetNoticeParams{
		InstanceID: instance.ID, Day: store.LocalDay(at), IssuedAt: stamp,
	})
	if err != nil || rows == 0 {
		return false, err
	}
	noticeID, err := store.NewID("onot") //nolint:misspell // Preserve the existing opaque notice ID prefix.
	if err != nil {
		return false, err
	}
	if err := q.InsertOperatorNotice(ctx, sqlcgen.InsertOperatorNoticeParams{
		ID: noticeID, ProfileID: instance.ProfileID,
		WorkspaceID: sql.NullString{String: instance.WorkspaceID, Valid: instance.WorkspaceID != ""},
		SessionID:   instance.MainSessionID.String, Kind: "warning", Action: "review_budget",
		Resource: "agent_instance", Title: "Daily agent budget warning",
		Body:      "This always-on instance has crossed 80% of a configured daily spending limit.",
		CreatedAt: stamp,
	}); err != nil {
		return false, fmt.Errorf("store: persist daily budget bell notice: %w", err)
	}
	return true, nil
}
