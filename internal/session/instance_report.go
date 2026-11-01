package session

import (
	"context"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"errors"
	"time"

	"github.com/compozy/compozy/internal/store"
)

type InstanceReportEvent struct {
	ID         string         `json:"report_id"`
	InstanceID string         `json:"instance_id"`
	Kind       string         `json:"kind"`
	Payload    jsontext.Value `json:"payload"`
	CreatedAt  time.Time      `json:"created_at"`
}

func (m *Manager) PublishInstanceReport(
	ctx context.Context,
	sessionID string,
	report InstanceReportEvent,
) (int64, error) {
	if report.ID == "" || report.InstanceID == "" || sessionID == "" ||
		report.CreatedAt.IsZero() || !report.Payload.IsValid() {
		return 0, errors.New("session: invalid instance report event")
	}
	info, err := m.Status(ctx, sessionID)
	if err != nil {
		return 0, err
	}
	content, err := json.Marshal(report)
	if err != nil {
		return 0, err
	}
	persisted, inserted, err := m.appendDurableSessionEvent(ctx, sessionID, store.SessionEvent{
		ID: "report:" + report.ID, SessionID: sessionID, TurnID: "report:" + report.ID,
		Type: "session.report", AgentName: info.AgentName,
		Content: string(content), Timestamp: report.CreatedAt.UTC(),
	})
	if err != nil {
		return 0, err
	}
	if inserted {
		m.publishSessionEventByID(ctx, sessionID, persisted)
	}
	return persisted.Sequence, nil
}
