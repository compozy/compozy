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

// DailyBudgetSessionOwner resolves a session's current instance
// even if no cap has ever been configured. Deletion must account for possible
// unobserved Done events before erasing the last ledger.
func (g *ObserveRepo) DailyBudgetSessionOwner(
	ctx context.Context, sessionID string,
) (store.DailyBudgetSessionOwnership, error) {
	if err := g.checkReady(ctx, "resolve budget recovery owner"); err != nil {
		return store.DailyBudgetSessionOwnership{}, err
	}
	owner, err := g.readQueries.GetTokenUsageReceiptSessionOwner(ctx, sessionID)
	if errors.Is(err, sql.ErrNoRows) {
		count, lookupErr := g.readQueries.CountDailyBudgetSessionCatalogEntry(ctx, sessionID)
		if lookupErr != nil {
			return store.DailyBudgetSessionOwnership{}, lookupErr
		}
		if count != 0 {
			return store.DailyBudgetSessionOwnership{}, fmt.Errorf(
				"store: invalid budget recovery ownership for session %q",
				sessionID,
			)
		}
		return store.DailyBudgetSessionOwnership{}, nil
	}
	if err != nil {
		return store.DailyBudgetSessionOwnership{}, err
	}
	return store.DailyBudgetSessionOwnership{
		InstanceID: owner.InstanceID, Unresolved: owner.OwnershipUnresolved != 0,
	}, nil
}

func (g *ObserveRepo) DailyBudgetInstanceHasLimits(ctx context.Context, instanceID string) (bool, error) {
	if err := g.checkReady(ctx, "inspect configured budget limits"); err != nil {
		return false, err
	}
	row, err := g.readQueries.GetAgentInstanceDailyBudgetLimits(ctx, instanceID)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	limits, err := decodeInstanceDailyBudgetLimits(row)
	return limits.Tokens != nil || limits.USD != nil, err
}

func (g *ObserveRepo) MarkUncertainDailyBudgetDay(
	ctx context.Context, instanceID, sessionID string, at time.Time,
) error {
	if err := g.checkReady(ctx, "record uncertain budget history"); err != nil {
		return err
	}
	if strings.TrimSpace(instanceID) == "" || strings.TrimSpace(sessionID) == "" {
		return errors.New("store: uncertain budget requires a session and instance")
	}
	return store.ExecuteWriteOperation(ctx, g.db, "mark uncertain daily spend", func(
		ctx context.Context, tx *store.WriteTx,
	) error {
		q := sqlcgen.New(tx)
		row, err := q.GetAgentInstanceDailyBudgetLimits(ctx, instanceID)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		if err == nil && (row.TokenCap.Valid || row.UsdCap.Valid) {
			return agentinstance.ErrBudgetHistoryUncertain
		}
		return q.MarkUncertainDailyBudgetDay(ctx, sqlcgen.MarkUncertainDailyBudgetDayParams{
			InstanceID: instanceID, Day: store.LocalDay(at), SourceSessionID: sessionID,
			RecordedAt: store.FormatTimestamp(g.now()),
		})
	})
}

func (g *ObserveRepo) DailyBudgetRecoveryCursor(ctx context.Context, sessionID string) (int64, error) {
	if err := g.checkReady(ctx, "read budget recovery cursor"); err != nil {
		return 0, err
	}
	seq, err := g.readQueries.ReadDailyBudgetRecoveryCursor(ctx, sessionID)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	return seq, err
}

func (g *ObserveRepo) AdvanceDailyBudgetRecoveryCursor(ctx context.Context, sessionID string, sequence int64) error {
	if err := g.checkReady(ctx, "advance budget recovery cursor"); err != nil {
		return err
	}
	if sequence < 0 {
		return errors.New("store: budget recovery cursor cannot be negative")
	}
	return store.ExecuteWriteOperation(ctx, g.db, "advance budget recovery cursor", func(
		ctx context.Context, tx *store.WriteTx,
	) error {
		return sqlcgen.New(tx).AdvanceDailyBudgetRecoveryCursor(ctx, sqlcgen.AdvanceDailyBudgetRecoveryCursorParams{
			SessionID: sessionID, AfterSequence: sequence, UpdatedAt: store.FormatTimestamp(g.now()),
		})
	})
}

// ListDailyBudgetReconcileSessions selects sessions belonging to configured
// always-on instances (including non-fork descendants). The global updated_at
// timestamp is not a safe filter: a crash may persist Done without updating
// that projection. It also returns potentially owned legacy history whose
// deleted edge cannot establish authority to attribute current-day spend.
func (g *ObserveRepo) ListDailyBudgetReconcileSessions(
	ctx context.Context, instanceID string,
) ([]store.DailyBudgetReconcileSession, error) {
	if err := g.checkReady(ctx, "list budget recovery sessions"); err != nil {
		return nil, err
	}
	rows, err := g.readQueries.ListDailyBudgetReconcileSessions(ctx, instanceID)
	if err != nil {
		return nil, fmt.Errorf("store: list daily budget recovery sessions: %w", err)
	}
	sessions := make([]store.DailyBudgetReconcileSession, 0, len(rows))
	for _, row := range rows {
		unresolved := row.AncestryGap != 0
		if unresolved {
			owner, err := g.readQueries.GetTokenUsageReceiptSessionOwner(ctx, row.ID)
			if err != nil && !errors.Is(err, sql.ErrNoRows) {
				return nil, err
			}
			if err == nil && owner.OwnershipUnresolved == 0 && owner.InstanceID != row.InstanceID {
				continue // the visible root is only a hint, and the actual owner differs
			}
			unresolved = errors.Is(err, sql.ErrNoRows) || owner.OwnershipUnresolved != 0
		}
		session, err := dailyBudgetRecoverySessionFromRow(row, unresolved)
		if err != nil {
			return nil, err
		}
		sessions = append(sessions, session)
	}
	return sessions, nil
}

func dailyBudgetRecoverySessionFromRow(
	row sqlcgen.ListDailyBudgetReconcileSessionsRow, unresolved bool,
) (store.DailyBudgetReconcileSession, error) {
	var instanceCreatedAt time.Time
	if unresolved {
		var err error
		instanceCreatedAt, err = store.ParseTimestamp(row.InstanceCreatedAt)
		if err != nil {
			return store.DailyBudgetReconcileSession{}, err
		}
	}
	info := store.SessionInfo{
		ID: row.ID, ProfileID: row.ProfileID, WorkspaceID: row.WorkspaceID,
		AgentName: row.AgentName, Provider: row.Provider, Model: row.Model,
		InstanceID: row.InstanceID,
	}
	return store.DailyBudgetReconcileSession{
		SessionInfo:     info,
		UnresolvedOwner: unresolved, InstanceCreatedAt: instanceCreatedAt,
	}, nil
}

// DailyBudgetReceiptState distinguishes recorded metrics from settled instance
// ownership before recovery tries to resolve historical provider configuration.
func (g *ObserveRepo) DailyBudgetReceiptState(
	ctx context.Context, sessionID, turnID, profileID string,
) (store.DailyBudgetReceiptState, error) {
	if err := g.checkReady(ctx, "read recovered token receipt"); err != nil {
		return store.DailyBudgetReceiptState{}, err
	}
	owner, err := g.readQueries.GetTokenUsageReceiptProfile(ctx, sqlcgen.GetTokenUsageReceiptProfileParams{
		SessionID: sessionID, TurnID: turnID,
	})
	if errors.Is(err, sql.ErrNoRows) {
		return store.DailyBudgetReceiptState{}, nil
	}
	if err != nil {
		return store.DailyBudgetReceiptState{}, err
	}
	if owner.ProfileID != profileID {
		return store.DailyBudgetReceiptState{}, fmt.Errorf(
			"store: receipt %q/%q belongs to different profile",
			sessionID,
			turnID,
		)
	}
	return store.DailyBudgetReceiptState{
		Day: owner.Day, Present: true, OwnershipUnresolved: owner.OwnershipUnresolved != 0,
	}, nil
}

// QuarantineDailyBudgetInstance records the recovery fence without changing an existing pause or start.
func (g *ObserveRepo) QuarantineDailyBudgetInstance(ctx context.Context, instanceID string) error {
	if err := g.checkReady(ctx, "quarantine daily budget instance"); err != nil {
		return err
	}
	return store.ExecuteWriteOperation(ctx, g.db, "quarantine daily budget instance", func(
		ctx context.Context, tx *store.WriteTx,
	) error {
		q := sqlcgen.New(tx)
		if err := q.RecordAgentInstanceDailyBudgetRecovery(ctx, instanceID); err != nil {
			return err
		}
		changed, err := q.QuarantineAgentInstanceDailyBudget(ctx, sqlcgen.QuarantineAgentInstanceDailyBudgetParams{
			UpdatedAt: store.FormatTimestamp(g.now()), ID: instanceID,
		})
		if err != nil || changed == 0 {
			return err
		}
		return q.CancelInstanceWake(ctx, instanceID)
	})
}

func (g *ObserveRepo) DailyBudgetRecoveryGeneration(ctx context.Context, instanceID string) (int64, error) {
	if err := g.checkReady(ctx, "read daily budget recovery generation"); err != nil {
		return 0, err
	}
	return dailyBudgetRecoveryGeneration(ctx, g.readQueries, instanceID)
}

func dailyBudgetRecoveryGeneration(ctx context.Context, q *sqlcgen.Queries, instanceID string) (int64, error) {
	gen, err := q.GetAgentInstanceDailyBudgetRecovery(ctx, instanceID)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	return gen, err
}

// ResolveDailyBudgetRecovery clears only the fence covered by the completed scan.
func (g *ObserveRepo) ResolveDailyBudgetRecovery(ctx context.Context, instanceID string, generation int64) error {
	if err := g.checkReady(ctx, "resolve daily budget recovery"); err != nil {
		return err
	}
	if generation == 0 {
		return nil
	}
	return store.ExecuteWriteOperation(ctx, g.db, "resolve daily budget recovery", func(
		ctx context.Context, tx *store.WriteTx,
	) error {
		q := sqlcgen.New(tx)
		cleared, err := q.ClearAgentInstanceDailyBudgetRecovery(ctx,
			sqlcgen.ClearAgentInstanceDailyBudgetRecoveryParams{InstanceID: instanceID, Generation: generation})
		if err != nil || cleared == 0 {
			return err
		}
		return q.RestoreAgentInstanceAfterDailyBudgetRecovery(ctx,
			sqlcgen.RestoreAgentInstanceAfterDailyBudgetRecoveryParams{
				ID: instanceID, UpdatedAt: store.FormatTimestamp(g.now()),
			})
	})
}
