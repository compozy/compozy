package globaldb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
)

func (g *SessionRepo) DispatchSessionReports(
	ctx context.Context, workspaceID, sessionID string, messageIDs []string, now time.Time,
) ([]store.SessionPromptReport, error) {
	if err := g.checkReady(ctx, "dispatch prompt reports"); err != nil {
		return nil, err
	}
	var reports []store.SessionPromptReport
	err := g.withImmediateTransaction(ctx, "dispatch prompt reports", func(exec globalSQLExecutor) error {
		queries := sqlcgen.New(exec)
		for _, messageID := range messageIDs {
			if messageID == "" {
				continue
			}
			admission, err := queries.GetSessionPromptAdmissionByMessageID(
				ctx,
				sqlcgen.GetSessionPromptAdmissionByMessageIDParams{
					WorkspaceID: workspaceID, SessionID: sessionID, MessageID: messageID,
				},
			)
			if errors.Is(err, sql.ErrNoRows) {
				continue
			}
			if err != nil {
				return err
			}
			rows, err := queries.ListPromptReports(ctx, admission.ID)
			if err != nil {
				return err
			}
			for _, row := range rows {
				if row.State != "reserved" || len(reports) == 20 {
					continue
				}
				count, err := queries.DeliverPromptReport(
					ctx,
					sqlcgen.DeliverPromptReportParams{ID: row.ID, Now: nullableSessionTime(now)},
				)
				if err != nil {
					return err
				}
				if count != 1 {
					return fmt.Errorf("store: report %s lost its reservation", row.ID)
				}
				reports = append(
					reports,
					store.SessionPromptReport{
						ID:         row.ID,
						InstanceID: row.InstanceID,
						SourceKey:  row.SourceKey,
						Kind:       row.Kind,
						Payload:    []byte(row.Payload),
					},
				)
			}
			if err := queries.ReleasePromptReports(ctx, admission.ID); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return reports, nil
}

func (g *SessionRepo) IndeterminatePromptReports(
	ctx context.Context,
	after string,
) ([]store.IndeterminatePromptReport, error) {
	rows, err := g.queries.ListIndeterminatePromptReports(ctx, after)
	if err != nil {
		return nil, err
	}
	result := make([]store.IndeterminatePromptReport, 0, len(rows))
	for _, row := range rows {
		at, err := store.ParseTimestamp(row.CreatedAt)
		if err != nil {
			return nil, err
		}
		result = append(
			result,
			store.IndeterminatePromptReport{At: at, ID: row.ID, SessionID: row.SessionID, TurnID: row.TurnID},
		)
	}
	return result, nil
}

func (g *SessionRepo) FailSessionReports(
	ctx context.Context,
	workspaceID, sessionID string,
	messageIDs []string,
) error {
	return g.withImmediateTransaction(ctx, "fail prompt reports", func(exec globalSQLExecutor) error {
		queries := sqlcgen.New(exec)
		for _, id := range messageIDs {
			row, err := queries.GetSessionPromptAdmissionByMessageID(
				ctx,
				sqlcgen.GetSessionPromptAdmissionByMessageIDParams{
					WorkspaceID: workspaceID, SessionID: sessionID, MessageID: id,
				},
			)
			if errors.Is(err, sql.ErrNoRows) {
				continue
			}
			if err != nil {
				return err
			}
			if err := queries.FailPromptReports(ctx, row.ID); err != nil {
				return err
			}
		}
		return nil
	})
}

func (g *SessionRepo) MarkIndeterminateReportNoted(ctx context.Context, id string) error {
	return g.queries.MarkIndeterminateReportNoted(ctx, id)
}
