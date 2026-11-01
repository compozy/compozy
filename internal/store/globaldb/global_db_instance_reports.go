package globaldb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/compozy/compozy/internal/agentinstance"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
)

type InstanceReportRepo struct{ *repoBase }

var _ agentinstance.ReportStore = (*InstanceReportRepo)(nil)

func (g *GlobalDB) InstanceReports() *InstanceReportRepo {
	return &InstanceReportRepo{repoBase: newRepoBase(g.db, g.now, &g.closed)}
}

func (r *InstanceReportRepo) AppendReport(
	ctx context.Context,
	instanceID string,
	input agentinstance.ReportInput,
) (agentinstance.Report, error) {
	if err := r.checkReady(ctx, "append instance report"); err != nil {
		return agentinstance.Report{}, err
	}
	if err := input.Validate(); err != nil {
		return agentinstance.Report{}, err
	}
	id, err := store.NewID("arpt")
	if err != nil {
		return agentinstance.Report{}, err
	}
	err = r.queries.InsertAgentInstanceReport(
		ctx,
		sqlcgen.InsertAgentInstanceReportParams{
			ID:          id,
			InstanceID:  instanceID,
			SourceKey:   input.SourceKey,
			Kind:        input.Kind,
			Payload:     string(input.Payload),
			Fingerprint: input.Fingerprint,
			CreatedAt:   store.FormatTimestamp(r.now()),
		},
	)
	if err != nil {
		return agentinstance.Report{}, fmt.Errorf("append instance report: %w", err)
	}
	row, err := r.queries.GetAgentInstanceReportBySource(
		ctx,
		sqlcgen.GetAgentInstanceReportBySourceParams{SourceKey: input.SourceKey, InstanceID: instanceID},
	)
	if err != nil {
		return agentinstance.Report{}, fmt.Errorf("read appended instance report: %w", err)
	}
	return instanceReport(row)
}

func (r *InstanceReportRepo) ListReports(
	ctx context.Context,
	q agentinstance.ReportQuery,
) ([]agentinstance.Report, error) {
	if err := r.checkReady(ctx, "list instance reports"); err != nil {
		return nil, err
	}
	if q.InstanceID == "" || q.Limit < 1 || q.Limit > 100 {
		return nil, errors.New("instance report query requires instance and limit from 1 to 100")
	}
	after := ""
	if !q.AfterCreatedAt.IsZero() {
		after = store.FormatTimestamp(q.AfterCreatedAt)
	}
	rows, err := r.queries.ListAgentInstanceReports(
		ctx,
		sqlcgen.ListAgentInstanceReportsParams{
			InstanceID:     q.InstanceID,
			ReportState:    q.State,
			AfterCreatedAt: after,
			AfterID:        q.AfterID,
			PageLimit:      int64(q.Limit),
		},
	)
	if err != nil {
		return nil, err
	}
	return instanceReports(rows)
}

func (r *InstanceReportRepo) RecentReports(
	ctx context.Context,
	instanceID string,
	limit int,
) ([]agentinstance.Report, error) {
	if err := r.checkReady(ctx, "recent instance reports"); err != nil {
		return nil, err
	}
	if instanceID == "" || limit < 1 || limit > 100 {
		return nil, errors.New("recent reports require instance and limit from 1 to 100")
	}
	rows, err := r.queries.ListRecentAgentInstanceReports(ctx, sqlcgen.ListRecentAgentInstanceReportsParams{
		InstanceID: instanceID, PageLimit: int64(limit),
	})
	if err != nil {
		return nil, err
	}
	return instanceReports(rows)
}

func instanceReports(rows []sqlcgen.AgentInstanceReport) ([]agentinstance.Report, error) {
	result := make([]agentinstance.Report, 0, len(rows))
	for _, row := range rows {
		report, err := instanceReport(row)
		if err != nil {
			return nil, err
		}
		result = append(result, report)
	}
	return result, nil
}

func (r *InstanceReportRepo) UnpublishedReports(
	ctx context.Context,
	after string,
	limit int,
) ([]agentinstance.ReportPublication, error) {
	if err := r.checkReady(ctx, "list unpublished instance reports"); err != nil {
		return nil, err
	}
	if limit < 1 {
		return nil, errors.New("publication limit must be positive")
	}
	rows, err := r.queries.ListUnpublishedAgentInstanceReports(
		ctx,
		sqlcgen.ListUnpublishedAgentInstanceReportsParams{AfterID: after, PageLimit: int64(limit)},
	)
	if err != nil {
		return nil, err
	}
	result := make([]agentinstance.ReportPublication, 0, len(rows))
	for _, row := range rows {
		report, err := instanceReport(
			sqlcgen.AgentInstanceReport{
				ID:                row.ID,
				InstanceID:        row.InstanceID,
				SourceKey:         row.SourceKey,
				Kind:              row.Kind,
				Payload:           row.Payload,
				Fingerprint:       row.Fingerprint,
				State:             row.State,
				PromptAdmissionID: row.PromptAdmissionID,
				EventSessionID:    row.EventSessionID,
				EventSeq:          row.EventSeq,
				CreatedAt:         row.CreatedAt,
				DeliveredAt:       row.DeliveredAt,
			},
		)
		if err != nil {
			return nil, err
		}
		result = append(
			result,
			agentinstance.ReportPublication{Report: report, MainSessionID: row.MainSessionID.String},
		)
	}
	return result, nil
}

func (r *InstanceReportRepo) MarkReportPublished(ctx context.Context, id, sessionID string, seq int64) error {
	if err := r.checkReady(ctx, "mark instance report published"); err != nil {
		return err
	}
	if sessionID == "" || seq <= 0 {
		return errors.New("report publication requires session and positive event sequence")
	}
	return r.queries.MarkAgentInstanceReportPublished(
		ctx,
		sqlcgen.MarkAgentInstanceReportPublishedParams{ID: id, SessionID: sessionID, EventSeq: seq},
	)
}

func instanceReport(row sqlcgen.AgentInstanceReport) (agentinstance.Report, error) {
	created, err := store.ParseTimestamp(row.CreatedAt)
	if err != nil {
		return agentinstance.Report{}, err
	}
	var delivered *time.Time
	if row.DeliveredAt.Valid {
		value, err := store.ParseTimestamp(row.DeliveredAt.String)
		if err != nil {
			return agentinstance.Report{}, err
		}
		delivered = &value
	}
	return agentinstance.Report{
		ID:         row.ID,
		InstanceID: row.InstanceID,
		ReportInput: agentinstance.ReportInput{
			SourceKey:   row.SourceKey,
			Kind:        row.Kind,
			Payload:     []byte(row.Payload),
			Fingerprint: row.Fingerprint,
		},
		State:             row.State,
		PromptAdmissionID: row.PromptAdmissionID,
		EventSessionID:    row.EventSessionID,
		EventSeq:          row.EventSeq,
		CreatedAt:         created,
		DeliveredAt:       delivered,
	}, nil
}

func (r *InstanceReportRepo) SweepTerminalReports(
	ctx context.Context,
	now time.Time,
	retention time.Duration,
	limit int,
) (int64, error) {
	if err := r.checkReady(ctx, "sweep terminal instance reports"); err != nil {
		return 0, err
	}
	if now.IsZero() || limit <= 0 {
		return 0, errors.New("report retention requires time and positive batch limit")
	}
	retention = max(retention, 48*time.Hour)
	return r.queries.SweepTerminalInstanceReports(
		ctx,
		sqlcgen.SweepTerminalInstanceReportsParams{
			Cutoff:     sql.NullString{String: store.FormatTimestamp(now.Add(-retention)), Valid: true},
			BatchLimit: int64(limit),
		},
	)
}

func (r *SessionRepo) ReportContextExhaustion(ctx context.Context, sessionID, turnID string) error {
	if err := r.checkReady(ctx, "report exhausted context"); err != nil {
		return err
	}
	caller, err := r.queries.GetInstanceCaller(ctx, sessionID)
	if err != nil {
		return err
	}
	id, ok := caller.OriginatingInstanceID.(string)
	if !ok {
		return errors.New("store: invalid originating instance")
	}
	if id == "" {
		return nil
	}
	instance, err := r.queries.GetAgentInstance(ctx, id)
	if err != nil {
		return err
	}
	if instance.ArchivedAt.Valid {
		return nil
	}
	reports := &InstanceReportRepo{repoBase: r.repoBase}
	_, err = reports.AppendReport(
		ctx,
		id,
		agentinstance.ReportInput{
			SourceKey: "turn:" + turnID,
			Kind:      "failure",
			Payload: []byte(
				`{"reason":"context_exhausted","detail":"The provider refused the prompt because its context limit was reached."}`,
			),
		},
	)
	return err
}
