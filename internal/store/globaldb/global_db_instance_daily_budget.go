package globaldb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/agentinstance"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
)

// SetDailyBudgetLimits atomically replaces the two optional caps of one live
// instance. Nil dimensions are unrestricted; a pointer to zero is a real cap.
func (r *InstanceRepo) SetDailyBudgetLimits(
	ctx context.Context,
	instanceID string,
	limits agentinstance.DailyBudgetLimits,
) error {
	if err := r.checkReady(ctx, "set instance daily budget limits"); err != nil {
		return err
	}
	if strings.TrimSpace(instanceID) == "" || strings.TrimSpace(instanceID) != instanceID {
		return errors.New("agentinstance: canonical instance id required")
	}
	if err := limits.Validate(); err != nil {
		return err
	}
	return store.ExecuteWriteOperation(ctx, r.db, "set instance daily budget limits", func(
		ctx context.Context, tx *store.WriteTx,
	) error {
		q := sqlcgen.New(tx)
		row, err := q.GetAgentInstance(ctx, instanceID)
		if errors.Is(err, sql.ErrNoRows) {
			return agentinstance.ErrNotFound
		}
		if err != nil {
			return fmt.Errorf("store: resolve daily budget instance: %w", err)
		}
		if row.ArchivedAt.Valid {
			return agentinstance.ErrNotFound
		}
		var tokens sql.NullInt64
		if limits.Tokens != nil {
			tokens = sql.NullInt64{Int64: *limits.Tokens, Valid: true}
		}
		var usd sql.NullString
		if limits.USD != nil {
			usd = sql.NullString{String: strconv.FormatFloat(*limits.USD, 'f', -1, 64), Valid: true}
		}
		return q.UpsertAgentInstanceDailyBudgetLimits(ctx, sqlcgen.UpsertAgentInstanceDailyBudgetLimitsParams{
			InstanceID: instanceID, TokenCap: tokens, UsdCap: usd,
			UpdatedAt: store.FormatTimestamp(r.now()),
		})
	})
}

// ReserveDailyBudgetWarning commits the once-per-local-day deduplication row
// and the persistent in-app bell notice in the same transaction. It must only
// be called after the authoritative usage assessment crosses the threshold.
func (r *InstanceRepo) ReserveDailyBudgetWarning(
	ctx context.Context,
	instanceID string,
	at time.Time,
) (issued bool, err error) {
	if err := r.checkReady(ctx, "reserve instance daily budget warning"); err != nil {
		return false, err
	}
	if strings.TrimSpace(instanceID) == "" || strings.TrimSpace(instanceID) != instanceID || at.IsZero() {
		return false, errors.New("agentinstance: warning requires a canonical instance and time")
	}
	err = store.ExecuteWriteOperation(ctx, r.db, "reserve instance daily budget warning", func(
		ctx context.Context, tx *store.WriteTx,
	) error {
		issued = false
		q := sqlcgen.New(tx)
		instance, err := q.GetAgentInstance(ctx, instanceID)
		if errors.Is(err, sql.ErrNoRows) {
			return agentinstance.ErrNotFound
		}
		if err != nil {
			return fmt.Errorf("store: get instance for daily warning: %w", err)
		}
		if instance.ArchivedAt.Valid {
			return agentinstance.ErrNotFound
		}
		if !instance.MainSessionID.Valid || strings.TrimSpace(instance.MainSessionID.String) == "" {
			return errors.New("agentinstance: daily budget warning requires a bound main session")
		}
		issued, err = reserveDailyBudgetWarningTx(ctx, q, instance, at)
		return err
	})
	if err != nil {
		return false, err
	}
	return issued, nil
}

// DailyBudgetLimits restores the effective caps without confusing an unset
// dimension with an explicit zero. Invalid stored amounts fail closed.
func (r *InstanceRepo) DailyBudgetLimits(
	ctx context.Context,
	instanceID string,
) (agentinstance.DailyBudgetLimits, error) {
	if err := r.checkReady(ctx, "read instance daily budget limits"); err != nil {
		return agentinstance.DailyBudgetLimits{}, err
	}
	if strings.TrimSpace(instanceID) == "" || strings.TrimSpace(instanceID) != instanceID {
		return agentinstance.DailyBudgetLimits{}, errors.New("agentinstance: canonical instance id required")
	}
	row, err := r.readQueries.GetAgentInstanceDailyBudgetLimits(ctx, instanceID)
	if errors.Is(err, sql.ErrNoRows) {
		if _, getErr := r.Get(ctx, instanceID); getErr != nil {
			return agentinstance.DailyBudgetLimits{}, getErr
		}
		return agentinstance.DailyBudgetLimits{}, nil
	}
	if err != nil {
		return agentinstance.DailyBudgetLimits{}, fmt.Errorf("store: read instance daily budget limits: %w", err)
	}
	return decodeInstanceDailyBudgetLimits(row)
}

func decodeInstanceDailyBudgetLimits(
	row sqlcgen.GetAgentInstanceDailyBudgetLimitsRow,
) (agentinstance.DailyBudgetLimits, error) {
	limits := agentinstance.DailyBudgetLimits{}
	if row.TokenCap.Valid {
		limits.Tokens = &row.TokenCap.Int64
	}
	if row.UsdCap.Valid {
		amount, parseErr := strconv.ParseFloat(row.UsdCap.String, 64)
		if parseErr != nil {
			return agentinstance.DailyBudgetLimits{}, fmt.Errorf("store: invalid stored daily USD cap: %w", parseErr)
		}
		if strconv.FormatFloat(amount, 'f', -1, 64) != row.UsdCap.String {
			return agentinstance.DailyBudgetLimits{}, errors.New("store: stored daily USD cap is not a canonical decimal")
		}
		limits.USD = &amount
	}
	if err := limits.Validate(); err != nil {
		return agentinstance.DailyBudgetLimits{}, fmt.Errorf("store: invalid stored daily budget: %w", err)
	}
	return limits, nil
}
