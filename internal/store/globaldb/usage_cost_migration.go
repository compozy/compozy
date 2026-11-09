package globaldb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"os"
	"path/filepath"
	"slices"

	"github.com/compozy/compozy/internal/acp"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
	"github.com/compozy/compozy/internal/store/sessiondb"
)

type repairedDailyCostKey struct {
	day, profile, workspace, agent string
}

type repairedCost struct {
	amount   float64
	currency string
	turns    int64
	unknown  bool
}

func (cost *repairedCost) add(usage store.TokenUsage) {
	cost.turns++
	if usage.CostAmount == nil || usage.CostCurrency == nil || *usage.CostCurrency == "" ||
		*usage.CostAmount < 0 || math.IsNaN(*usage.CostAmount) || math.IsInf(*usage.CostAmount, 0) ||
		(cost.currency != "" && cost.currency != *usage.CostCurrency) {
		cost.unknown = true
		return
	}
	cost.currency = *usage.CostCurrency
	cost.amount += *usage.CostAmount
	if math.IsInf(cost.amount, 0) {
		cost.unknown = true
	}
}

// repairUsageCosts completes migration 199 before any runtime can append usage.
// Session migration 11 is lossless and idempotent; one global transaction commits
// only agent-reported aggregates and the completion marker. Interrupted opens retry.
func repairUsageCosts(ctx context.Context, db *sql.DB, path string) error {
	queries := sqlcgen.New(db)
	pending, err := queries.UsageCostRepairPending(ctx)
	if err != nil {
		return fmt.Errorf("store: inspect usage cost repair: %w", err)
	}
	if pending == 0 {
		return nil
	}
	rows, err := queries.ListUsageCostRepairSessions(ctx)
	if err != nil {
		return fmt.Errorf("store: read usage cost repair sessions: %w", err)
	}
	return store.ExecuteWrite(ctx, db, func(ctx context.Context, tx *store.WriteTx) error {
		writer := sqlcgen.New(tx)
		// Capture original provenance before invalidation. A matching reconstructed
		// daily key must never overwrite a previously estimated/included row.
		days, err := writer.ListUsageCostRepairDays(ctx)
		if err != nil {
			return fmt.Errorf("store: read usage cost repair days: %w", err)
		}
		if err := writer.InvalidateReportedSessionCosts(ctx); err != nil {
			return fmt.Errorf("store: invalidate reported session costs: %w", err)
		}
		if err := writer.InvalidateReportedDailyCosts(ctx); err != nil {
			return fmt.Errorf("store: invalidate reported daily costs: %w", err)
		}
		daily := make(map[repairedDailyCostKey]*repairedCost)
		for _, row := range rows {
			if err := repairSessionCost(ctx, writer, path, row, daily); err != nil {
				return err
			}
		}
		for _, day := range days {
			key := repairedDailyCostKey{day.Day, day.ProfileID, day.WorkspaceID, day.AgentName}
			cost := daily[key]
			if cost == nil || cost.unknown || cost.turns == 0 {
				continue
			}
			if err := writer.RepairDailyUsageCost(ctx, sqlcgen.RepairDailyUsageCostParams{
				Amount: sql.NullFloat64{
					Float64: cost.amount,
					Valid:   true,
				},
				Currency:    sql.NullString{String: cost.currency, Valid: true},
				Day:         key.day,
				ProfileID:   key.profile,
				WorkspaceID: key.workspace,
				AgentName:   key.agent,
				TurnCount:   cost.turns,
			}); err != nil {
				return fmt.Errorf("store: repair daily usage cost: %w", err)
			}
		}
		return writer.CompleteUsageCostRepair(ctx)
	})
}

func repairSessionCost(
	ctx context.Context,
	writer *sqlcgen.Queries,
	path string,
	row sqlcgen.ListUsageCostRepairSessionsRow,
	daily map[repairedDailyCostKey]*repairedCost,
) error {
	usage, err := migrationSessionUsage(ctx, path, store.SessionDBOwner{
		SessionID: row.SessionID, WorkspaceID: row.WorkspaceID,
	}, row.AgentName)
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		slog.WarnContext(
			ctx,
			"usage cost repair left unavailable ledger unknown",
			"session_id",
			row.SessionID,
			"error",
			err,
		)
		return nil
	}
	// A missing or partial ledger cannot certify an aggregate. This also rejects
	// legacy sessions whose per-agent attribution cannot be recovered exactly.
	if int64(len(usage)) != row.TurnCount || len(usage) == 0 {
		return nil
	}
	var total repairedCost
	for _, turn := range usage {
		total.add(turn)
		key := repairedDailyCostKey{store.LocalDay(turn.Timestamp), row.ProfileID, row.WorkspaceID, row.AgentName}
		if daily[key] == nil {
			daily[key] = &repairedCost{}
		}
		daily[key].add(turn)
	}
	if total.unknown {
		return nil
	}
	if err := writer.RepairSessionUsageCost(ctx, sqlcgen.RepairSessionUsageCostParams{
		ID:       row.ID,
		Amount:   sql.NullFloat64{Float64: total.amount, Valid: true},
		Currency: sql.NullString{String: total.currency, Valid: true},
	}); err != nil {
		return fmt.Errorf("store: repair session usage cost: %w", err)
	}
	return nil
}

func migrationSessionUsage(
	ctx context.Context,
	path string,
	owner store.SessionDBOwner,
	agent string,
) (_ []store.TokenUsage, retErr error) {
	if filepath.Base(owner.SessionID) != owner.SessionID || owner.SessionID == "." || owner.SessionID == ".." {
		return nil, errors.New("store: invalid cost repair session identity")
	}
	root := filepath.Join(filepath.Dir(path), "sessions", owner.SessionID)
	ledgerPath := store.SessionDBFile(root)
	if err := requireCostRepairLedgerReady(ledgerPath); err != nil {
		return nil, err
	}
	// #nosec G703 -- session identity is validated by the owning sessiondb opener below.
	if _, err := os.Lstat(ledgerPath); err != nil {
		return nil, err // Never create an empty database for missing historical evidence.
	}
	ledger, err := sessiondb.OpenSessionDB(ctx, owner, ledgerPath)
	if err != nil {
		return nil, err
	}
	defer func() { retErr = errors.Join(retErr, ledger.Close(context.WithoutCancel(ctx))) }()
	usage, err := ledger.ListTokenUsage(ctx)
	if err != nil {
		return nil, err
	}
	turns := make(map[string]bool)
	query := store.EventQuery{Type: acp.EventTypeDone, AgentName: agent, Forward: true, Limit: 256}
	for {
		rows, err := ledger.Query(ctx, query)
		if err != nil {
			return nil, err
		}
		for _, event := range rows {
			turns[event.TurnID] = true
			query.AfterSequence = event.Sequence
		}
		if len(rows) < query.Limit {
			break
		}
	}
	return slices.DeleteFunc(usage, func(turn store.TokenUsage) bool { return !turns[turn.TurnID] }), nil
}

// Clear recovery owns byte-hashed database/WAL snapshots. Boot cost repair runs
// before the session manager can settle them, so never open/migrate that family.
func requireCostRepairLedgerReady(path string) error {
	for _, suffix := range []string{
		".clear-manifest", ".clear-committed", ".clear-backup",
		"-wal.clear-backup", "-shm.clear-backup", "-journal.clear-backup",
	} {
		if _, err := os.Lstat(path + suffix); err == nil {
			return errors.New("store: cost repair ledger awaits clear recovery")
		} else if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("store: inspect cost repair ledger recovery: %w", err)
		}
	}
	return nil
}
