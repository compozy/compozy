package globaldb

import (
	"context"
	"encoding/json/v2"
	"time"

	"github.com/compozy/compozy/internal/heartbeat"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
)

type instanceProposalPayload struct {
	Proposals []heartbeat.TriageProposal `json:"proposals"`
}

func deduplicateInstanceProposals(ctx context.Context, q *sqlcgen.Queries, req *heartbeat.FinishRequest) error {
	if req.Result != "reported" || req.Report == nil || req.Report.Kind != "proposal" {
		return nil
	}
	validation := *req.Report
	validation.SourceKey = "occ:" + req.OccurrenceID
	if err := validation.Validate(); err != nil {
		return err
	}
	var incoming instanceProposalPayload
	if err := json.Unmarshal(req.Report.Payload, &incoming); err != nil {
		return err
	}
	seen := map[string]bool{}
	cutoff := store.FormatTimestamp(req.Now.Add(-24 * time.Hour))
	query := sqlcgen.ListAgentInstanceReportsParams{
		InstanceID:     req.InstanceID,
		ReportState:    "",
		AfterCreatedAt: store.FormatTimestamp(req.Now.Add(-24 * time.Hour)),
		PageLimit:      100,
	}
	for {
		rows, err := q.ListAgentInstanceReports(ctx, query)
		if err != nil {
			return err
		}
		for _, row := range rows {
			if row.Kind != "proposal" || row.CreatedAt <= cutoff {
				continue
			}
			var previous instanceProposalPayload
			if err := json.Unmarshal([]byte(row.Payload), &previous); err != nil {
				return err
			}
			for _, proposal := range previous.Proposals {
				seen[heartbeat.ProposalFingerprint(proposal)] = true
			}
		}
		if len(rows) < int(query.PageLimit) {
			break
		}
		last := rows[len(rows)-1]
		query.AfterCreatedAt, query.AfterID = last.CreatedAt, last.ID
	}
	retained := make([]heartbeat.TriageProposal, 0, len(incoming.Proposals))
	for _, proposal := range incoming.Proposals {
		key := heartbeat.ProposalFingerprint(proposal)
		if !seen[key] {
			retained = append(retained, proposal)
			seen[key] = true
		}
	}
	if len(retained) == 0 {
		req.Result, req.Reason, req.Report = "quiet", "duplicate_suppressed", nil
		return nil
	}
	payload, err := json.Marshal(instanceProposalPayload{Proposals: retained})
	if err != nil {
		return err
	}
	report := *req.Report
	report.Payload = payload
	req.Report = &report
	return nil
}
