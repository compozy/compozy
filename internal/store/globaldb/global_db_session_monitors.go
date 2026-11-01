package globaldb

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/compozy/compozy/internal/monitor"
	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
)

var _ monitor.Store = (*SessionMonitorRepo)(nil)

func (g *SessionMonitorRepo) SaveMonitor(ctx context.Context, record monitor.Record) error {
	if err := g.checkReady(ctx, "save session monitor"); err != nil {
		return err
	}
	mon := record.Monitor
	exitCode := sql.NullInt64{}
	if mon.ExitCode != nil {
		exitCode = sql.NullInt64{Int64: int64(*mon.ExitCode), Valid: true}
	}
	firedAt := ""
	if !mon.FiredAt.IsZero() {
		firedAt = mon.FiredAt.UTC().Format(time.RFC3339Nano)
	}
	checkFailedAt := ""
	if !record.OwnerCheckFailedAt.IsZero() {
		checkFailedAt = record.OwnerCheckFailedAt.UTC().Format(time.RFC3339Nano)
	}
	if err := g.queries.UpsertSessionMonitor(ctx, sqlcgen.UpsertSessionMonitorParams{
		ID: mon.ID, SessionID: mon.SessionID, WorkspaceID: record.WorkspaceID, ProfileID: record.ProfileID,
		Kind: string(mon.Kind), Description: mon.Description, TerminalID: mon.TerminalID, Until: mon.Until,
		Pattern: mon.Pattern, TimeoutMs: record.Timeout.Milliseconds(), Status: string(mon.Status),
		CreatedAt:  mon.CreatedAt.UTC().Format(time.RFC3339Nano),
		DeadlineAt: mon.DeadlineAt.UTC().Format(time.RFC3339Nano),
		FiredAt:    firedAt, Reason: mon.Reason, ExitCode: exitCode, OwnerBinding: record.OwnerBinding,
		OwnerCheckFailedAt: checkFailedAt, Version: record.Version,
	}); err != nil {
		return fmt.Errorf("store: save session monitor: %w", err)
	}
	return nil
}

func (g *SessionMonitorRepo) LoadMonitors(ctx context.Context) ([]monitor.Record, error) {
	if err := g.checkReady(ctx, "load session monitors"); err != nil {
		return nil, err
	}
	rows, err := g.queries.ListSessionMonitors(ctx)
	if err != nil {
		return nil, fmt.Errorf("store: load session monitors: %w", err)
	}
	records := make([]monitor.Record, 0, len(rows))
	for _, row := range rows {
		record, err := sessionMonitorRecord(row)
		if err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	return records, nil
}

func (g *SessionMonitorRepo) PruneFinishedMonitors(ctx context.Context, cutoff time.Time) error {
	if err := g.checkReady(ctx, "prune session monitors"); err != nil {
		return err
	}
	if err := g.queries.PruneFinishedSessionMonitors(ctx, cutoff.UTC().Format(time.RFC3339Nano)); err != nil {
		return fmt.Errorf("store: prune session monitors: %w", err)
	}
	return nil
}

func sessionMonitorRecord(row sqlcgen.SessionMonitor) (monitor.Record, error) {
	created, err := time.Parse(time.RFC3339Nano, row.CreatedAt)
	if err != nil {
		return monitor.Record{}, fmt.Errorf("store: parse session monitor %s created_at: %w", row.ID, err)
	}
	deadline, err := time.Parse(time.RFC3339Nano, row.DeadlineAt)
	if err != nil {
		return monitor.Record{}, fmt.Errorf("store: parse session monitor %s deadline_at: %w", row.ID, err)
	}
	var fired time.Time
	if row.FiredAt != "" {
		if fired, err = time.Parse(time.RFC3339Nano, row.FiredAt); err != nil {
			return monitor.Record{}, fmt.Errorf("store: parse session monitor %s fired_at: %w", row.ID, err)
		}
	}
	var checkFailed time.Time
	if row.OwnerCheckFailedAt != "" {
		if checkFailed, err = time.Parse(time.RFC3339Nano, row.OwnerCheckFailedAt); err != nil {
			return monitor.Record{}, fmt.Errorf(
				"store: parse session monitor %s owner_check_failed_at: %w", row.ID, err)
		}
	}
	var exitCode *int
	if row.ExitCode.Valid {
		exitCode = new(int(row.ExitCode.Int64))
	}
	return monitor.Record{
		Monitor: monitor.Monitor{
			ID: row.ID, SessionID: row.SessionID, Kind: monitor.Kind(row.Kind), Description: row.Description,
			TerminalID: row.TerminalID, Until: row.Until, Pattern: row.Pattern, Status: monitor.Status(row.Status),
			CreatedAt: created, DeadlineAt: deadline, FiredAt: fired, Reason: row.Reason, ExitCode: exitCode,
		},
		WorkspaceID: row.WorkspaceID, ProfileID: row.ProfileID,
		Timeout:      time.Duration(row.TimeoutMs) * time.Millisecond,
		OwnerBinding: row.OwnerBinding, OwnerCheckFailedAt: checkFailed, Version: row.Version,
	}, nil
}
