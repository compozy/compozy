package globaldb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/agentinstance"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
)

// warnForAcceptedBudgetReceipt runs inside the same immediate write transaction
// as the new canonical receipt and both overview projections. A replay never
// reaches this path and can never emit another warning.
func warnForAcceptedBudgetReceipt(
	ctx context.Context, q *sqlcgen.Queries, sessionID string, at time.Time,
) error {
	owner, err := q.GetTokenUsageReceiptSessionOwner(ctx, sessionID)
	if err != nil {
		return fmt.Errorf("store: resolve budget receipt warning owner: %w", err)
	}
	if owner.InstanceID == "" {
		return nil
	}
	instance, err := q.GetAgentInstance(ctx, owner.InstanceID)
	if err != nil {
		return fmt.Errorf("store: resolve budget warning instance: %w", err)
	}
	if instance.ProfileID != owner.ProfileID {
		return agentinstance.ErrScopeMismatch
	}
	// An in-flight turn may finish after an instance was archived. Its usage
	// still belongs in the ledger, but archived instances receive no new bells.
	if instance.ArchivedAt.Valid {
		return nil
	}
	limitsRow, err := q.GetAgentInstanceDailyBudgetLimits(ctx, owner.InstanceID)
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
		InstanceID: owner.InstanceID, Day: day,
	})
	if err != nil || issued > 0 {
		return err
	}
	usage, err := readDailyBudgetUsage(ctx, q, owner.ProfileID, owner.InstanceID, day)
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
	noticeID, err := store.NewID("onot")
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
