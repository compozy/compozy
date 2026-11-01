package session

import (
	"context"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"errors"
	"time"

	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/transcript"
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
	var notification *OperatorNotification
	title, body := "", ""
	if report.Kind == "proposal" {
		title, body = "New agent proposal", "Review the proposed work in the agent thread."
	}
	if report.Kind == "failure" {
		var failure struct {
			Reason string `json:"reason"`
		}
		if err := json.Unmarshal(report.Payload, &failure); err != nil {
			return 0, err
		}
		if failure.Reason == "fatigue" {
			title, body = "Proactive checks paused", "Consecutive checks failed. Review the agent report before resuming."
		}
	}
	if title != "" {
		if m.attentionWorkspaceMutes == nil {
			return 0, errors.New("session: attention workspace mute reader is required")
		}
		muted, err := m.attentionWorkspaceMutes.IsAttentionWorkspaceMuted(ctx, info.ProfileID, info.WorkspaceID)
		if err != nil {
			return 0, err
		}
		if !muted {
			notification = &OperatorNotification{NotificationID: "report:" + report.ID, SessionID: sessionID, ProfileID: info.ProfileID, WorkspaceID: info.WorkspaceID, Title: title, Body: body, At: report.CreatedAt}
		}
	}
	content, err := json.Marshal(report)
	if err != nil {
		return 0, err
	}
	persisted, inserted, err := m.appendDurableSessionEvent(ctx, sessionID, store.SessionEvent{
		ID: "report:" + report.ID, SessionID: sessionID, TurnID: "report:" + report.ID,
		Type: transcript.ReportEventType, AgentName: info.AgentName,
		Content: string(content), Timestamp: report.CreatedAt.UTC(),
	})
	if err != nil {
		return 0, err
	}
	if inserted {
		m.publishSessionEventByID(ctx, sessionID, persisted)
		if notification != nil {
			m.publishSessionCatalogEventCount(CatalogEvent{Name: CatalogEventNameOperatorNotification, ProfileID: info.ProfileID, WorkspaceID: info.WorkspaceID, SessionID: sessionID, OperatorNotification: notification})
		}
		m.PublishInstanceCatalogChanged(info.ProfileID, info.WorkspaceID, report.InstanceID)
	}
	return persisted.Sequence, nil
}
