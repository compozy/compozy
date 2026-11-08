package globaldb

import (
	"context"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"strings"

	"github.com/compozy/compozy/internal/agentinstance"
	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
)

func (r *InstanceReportRepo) projectReportToolCalls(
	ctx context.Context,
	rows []sqlcgen.AgentInstanceReport,
) ([]agentinstance.Report, error) {
	reports, err := instanceReports(rows)
	if err != nil {
		return nil, err
	}
	for i := range reports {
		report := &reports[i]
		occurrence, ok := strings.CutPrefix(report.SourceKey, "occ:")
		if !ok {
			continue
		}
		summary, err := r.readQueries.CheckInToolCallSummary(ctx, occurrence)
		if err != nil {
			return nil, err
		}
		var payload map[string]jsontext.Value
		if err = json.Unmarshal(report.Payload, &payload); err != nil {
			return nil, err
		}
		payload["tool_calls"], err = json.Marshal(
			agentinstance.CheckInToolSummary{
				Total:       summary.Total,
				WritesDone:  summary.WritesDone,
				WritesMaybe: summary.WritesMaybe,
				Denied:      summary.Denied,
			},
		)
		if err != nil {
			return nil, err
		}
		report.Payload, err = json.Marshal(payload)
		if err != nil {
			return nil, err
		}
	}
	return reports, nil
}
