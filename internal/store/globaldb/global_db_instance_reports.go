package globaldb

import (
	"context"
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

func (r *InstanceReportRepo) AppendReport(ctx context.Context, instanceID string, input agentinstance.ReportInput) (agentinstance.Report, error) {
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
	err = r.queries.InsertAgentInstanceReport(ctx, sqlcgen.InsertAgentInstanceReportParams{ID: id, InstanceID: instanceID, SourceKey: input.SourceKey, Kind: input.Kind, Payload: string(input.Payload), Fingerprint: input.Fingerprint, CreatedAt: store.FormatTimestamp(r.now())})
	if err != nil {
		return agentinstance.Report{}, fmt.Errorf("append instance report: %w", err)
	}
	row, err := r.queries.GetAgentInstanceReportBySource(ctx, sqlcgen.GetAgentInstanceReportBySourceParams{SourceKey: input.SourceKey, InstanceID: instanceID})
	if err != nil {
		return agentinstance.Report{}, fmt.Errorf("read appended instance report: %w", err)
	}
	return instanceReport(row)
}

func (r *InstanceReportRepo) ListReports(ctx context.Context, q agentinstance.ReportQuery) ([]agentinstance.Report, error) {
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
	rows, err := r.queries.ListAgentInstanceReports(ctx, sqlcgen.ListAgentInstanceReportsParams{InstanceID: q.InstanceID, ReportState: q.State, AfterCreatedAt: after, AfterID: q.AfterID, PageLimit: int64(q.Limit)})
	if err != nil {
		return nil, err
	}
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

func (r *InstanceReportRepo) UnpublishedReports(ctx context.Context, after string, limit int) ([]agentinstance.ReportPublication, error) {
	if err := r.checkReady(ctx, "list unpublished instance reports"); err != nil {
		return nil, err
	}
	if limit < 1 {
		return nil, errors.New("publication limit must be positive")
	}
	rows, err := r.queries.ListUnpublishedAgentInstanceReports(ctx, sqlcgen.ListUnpublishedAgentInstanceReportsParams{AfterID: after, PageLimit: int64(limit)})
	if err != nil {
		return nil, err
	}
	result := make([]agentinstance.ReportPublication, 0, len(rows))
	for _, row := range rows {
		report, err := instanceReport(sqlcgen.AgentInstanceReport{ID: row.ID, InstanceID: row.InstanceID, SourceKey: row.SourceKey, Kind: row.Kind, Payload: row.Payload, Fingerprint: row.Fingerprint, State: row.State, PromptAdmissionID: row.PromptAdmissionID, EventSessionID: row.EventSessionID, EventSeq: row.EventSeq, CreatedAt: row.CreatedAt, DeliveredAt: row.DeliveredAt})
		if err != nil {
			return nil, err
		}
		result = append(result, agentinstance.ReportPublication{Report: report, MainSessionID: row.MainSessionID.String})
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
	return r.queries.MarkAgentInstanceReportPublished(ctx, sqlcgen.MarkAgentInstanceReportPublishedParams{ID: id, SessionID: sessionID, EventSeq: seq})
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
	return agentinstance.Report{ID: row.ID, InstanceID: row.InstanceID, ReportInput: agentinstance.ReportInput{SourceKey: row.SourceKey, Kind: row.Kind, Payload: []byte(row.Payload), Fingerprint: row.Fingerprint}, State: row.State, PromptAdmissionID: row.PromptAdmissionID, EventSessionID: row.EventSessionID, EventSeq: row.EventSeq, CreatedAt: created, DeliveredAt: delivered}, nil
}
