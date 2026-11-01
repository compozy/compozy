package session

import (
	"context"

	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/transcript"
)

const reportEvidenceCodeKey = "code"

type indeterminateReportReader interface {
	IndeterminatePromptReports(context.Context, string) ([]store.IndeterminatePromptReport, error)
}

func (m *Manager) publishIndeterminateReportNotes(ctx context.Context) error {
	reader, ok := m.promptAdmissionStore.(indeterminateReportReader)
	if !ok {
		return nil
	}
	after := ""
	for {
		reports, err := reader.IndeterminatePromptReports(ctx, after)
		if err != nil {
			return err
		}
		for _, report := range reports {
			info, err := m.Status(ctx, report.SessionID)
			if err != nil {
				return err
			}
			marker, err := transcript.NewMarker(transcript.MarkerPromptDropped,
				"Report delivery could not be confirmed. This report will not be resent.", report.At,
				map[string]any{"report_id": report.ID, reportEvidenceCodeKey: "report_dispatch_indeterminate"})
			if err != nil {
				return err
			}
			event, err := marker.AgentEvent(report.SessionID, report.TurnID)
			if err != nil {
				return err
			}
			payload, err := marshalAgentEventPreservingAuthoredText(event, "")
			if err != nil {
				return err
			}
			persisted, inserted, err := m.appendDurableSessionEvent(ctx, report.SessionID, store.SessionEvent{
				ID: "report-indeterminate:" + report.ID, SessionID: report.SessionID, TurnID: report.TurnID,
				Type: event.Type, AgentName: info.AgentName, Content: payload, Timestamp: report.At,
			})
			if err != nil {
				return err
			}
			if inserted {
				m.publishSessionEventByID(ctx, report.SessionID, persisted)
			}
			after = report.ID
		}
		if len(reports) < 100 {
			return nil
		}
	}
}
