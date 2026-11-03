package globaldb

import (
	"context"
	"database/sql"
	"encoding/json/v2"
	"fmt"
	"time"

	"github.com/compozy/compozy/internal/monitor"
	"github.com/compozy/compozy/internal/store"
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
	lastFiredAt := ""
	if !mon.LastFiredAt.IsZero() {
		lastFiredAt = mon.LastFiredAt.UTC().Format(time.RFC3339Nano)
	}
	targetSessionIDs, states, targets, err := encodeMonitorTargets(mon)
	if err != nil {
		return err
	}
	params := sqlcgen.UpsertSessionMonitorParams{
		ID: mon.ID, SessionID: mon.SessionID, WorkspaceID: record.WorkspaceID, ProfileID: record.ProfileID,
		Kind: string(mon.Kind), Description: mon.Description, TerminalID: mon.TerminalID, Until: mon.Until,
		Pattern: mon.Pattern, TimeoutMs: record.Timeout.Milliseconds(), Status: string(mon.Status),
		CreatedAt:  mon.CreatedAt.UTC().Format(time.RFC3339Nano),
		DeadlineAt: mon.DeadlineAt.UTC().Format(time.RFC3339Nano),
		FiredAt:    firedAt, Reason: mon.Reason, ExitCode: exitCode, OwnerBinding: record.OwnerBinding,
		OwnerCheckFailedAt: checkFailedAt, Version: record.Version, Branch: mon.Branch, RepoKey: record.RepoKey,
		PrCursor: record.Cursor, FireCount: int64(mon.FireCount), LastFiredAt: lastFiredAt,
		TargetSessionIds: targetSessionIDs, States: states, Targets: targets,
	}
	return store.ExecuteWriteOperation(
		ctx,
		g.db,
		"save session monitor",
		func(ctx context.Context, tx *store.WriteTx) error {
			q := sqlcgen.New(tx)
			changed, err := q.UpsertSessionMonitor(ctx, params)
			if err != nil || changed == 0 {
				return err
			}
			return g.reportLostInstanceMonitor(ctx, q, record)
		},
	)
}

func (g *SessionMonitorRepo) reportLostInstanceMonitor(
	ctx context.Context,
	q *sqlcgen.Queries,
	record monitor.Record,
) error {
	mon := record.Monitor
	switch mon.Status {
	case monitor.StatusInterrupted, monitor.StatusUndelivered, monitor.StatusCanceled:
	default:
		return nil
	}
	if mon.Reason == "shutdown" {
		return nil
	}
	cause := mon.Reason
	if cause == "" {
		cause = string(mon.Status)
	}
	payload, err := json.Marshal(struct {
		SessionID string `json:"session_id"`
		Cause     string `json:"cause"`
	}{SessionID: mon.SessionID, Cause: cause})
	if err != nil {
		return fmt.Errorf("encode lost monitor report: %w", err)
	}
	id, err := store.NewID("arpt")
	if err != nil {
		return err
	}
	return q.InsertLostInstanceMonitorReport(ctx, sqlcgen.InsertLostInstanceMonitorReportParams{
		ID: id, SourceKey: (monitor.Notification{Monitor: mon}).WakeMessageID(), Payload: string(payload),
		CreatedAt: store.FormatTimestamp(g.now()), SessionID: sql.NullString{String: mon.SessionID, Valid: true},
		ProfileID: record.ProfileID, WorkspaceID: record.WorkspaceID,
	})
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
	var lastFired time.Time
	if row.LastFiredAt != "" {
		if lastFired, err = time.Parse(time.RFC3339Nano, row.LastFiredAt); err != nil {
			return monitor.Record{}, fmt.Errorf("store: parse session monitor %s last_fired_at: %w", row.ID, err)
		}
	}
	var exitCode *int
	if row.ExitCode.Valid {
		exitCode = new(int(row.ExitCode.Int64))
	}
	mon := monitor.Monitor{
		ID: row.ID, SessionID: row.SessionID, Kind: monitor.Kind(row.Kind), Description: row.Description,
		TerminalID: row.TerminalID, Until: row.Until, Pattern: row.Pattern, Status: monitor.Status(row.Status),
		CreatedAt: created, DeadlineAt: deadline, FiredAt: fired, Reason: row.Reason, ExitCode: exitCode,
		Branch: row.Branch, FireCount: int(row.FireCount), LastFiredAt: lastFired,
	}
	if err := decodeMonitorTargets(row, &mon); err != nil {
		return monitor.Record{}, err
	}
	return monitor.Record{
		Monitor: mon, WorkspaceID: row.WorkspaceID, ProfileID: row.ProfileID,
		Timeout:      time.Duration(row.TimeoutMs) * time.Millisecond,
		OwnerBinding: row.OwnerBinding, OwnerCheckFailedAt: checkFailed, Version: row.Version,
		RepoKey: row.RepoKey, Cursor: row.PrCursor,
	}, nil
}

// encodeMonitorTargets renders the session monitor columns; other kinds store empty arrays.
func encodeMonitorTargets(mon monitor.Monitor) (targetSessionIDs, states, targets string, err error) {
	values := []any{mon.TargetSessionIDs, mon.States, mon.Targets}
	encoded := make([]string, len(values))
	for index, value := range values {
		data, err := json.Marshal(value)
		if err != nil {
			return "", "", "", fmt.Errorf("store: encode session monitor %s targets: %w", mon.ID, err)
		}
		encoded[index] = string(data)
	}
	return encoded[0], encoded[1], encoded[2], nil
}

func decodeMonitorTargets(row sqlcgen.SessionMonitor, mon *monitor.Monitor) error {
	if row.Kind != string(monitor.KindSession) {
		return nil
	}
	if err := json.Unmarshal([]byte(row.TargetSessionIds), &mon.TargetSessionIDs); err != nil {
		return fmt.Errorf("store: parse session monitor %s target_session_ids: %w", row.ID, err)
	}
	if err := json.Unmarshal([]byte(row.States), &mon.States); err != nil {
		return fmt.Errorf("store: parse session monitor %s states: %w", row.ID, err)
	}
	if err := json.Unmarshal([]byte(row.Targets), &mon.Targets); err != nil {
		return fmt.Errorf("store: parse session monitor %s targets: %w", row.ID, err)
	}
	if len(mon.Targets) == 0 {
		mon.Targets = nil
	}
	return nil
}
