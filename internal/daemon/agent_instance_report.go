package daemon

import (
	"context"

	"github.com/compozy/compozy/internal/agentinstance"
	"github.com/compozy/compozy/internal/session"
)

type instanceReportSessionWriter interface {
	PublishInstanceReport(context.Context, string, session.InstanceReportEvent) (int64, error)
}

type instanceReportEventWriter struct{ sessions instanceReportSessionWriter }

var _ agentinstance.ReportEventWriter = instanceReportEventWriter{}

func (w instanceReportEventWriter) Publish(
	ctx context.Context,
	sessionID string,
	report agentinstance.Report,
) (int64, error) {
	return w.sessions.PublishInstanceReport(
		ctx,
		sessionID,
		session.InstanceReportEvent{
			Republished: report.EventSessionID != "",
			ID:          report.ID,
			InstanceID:  report.InstanceID,
			Kind:        report.Kind,
			Payload:     report.Payload,
			CreatedAt:   report.CreatedAt,
		},
	)
}
