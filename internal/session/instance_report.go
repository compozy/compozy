package session

import (
	"context"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/acp"

	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/transcript"
)

type InstanceReportEvent struct {
	NotificationPayload jsontext.Value `json:"-"`
	WritesDone          int64          `json:"-"`
	WritesMaybe         int64          `json:"-"`
	Republished         bool           `json:"-"`
	ID                  string         `json:"report_id"`
	InstanceID          string         `json:"instance_id"`
	Kind                string         `json:"kind"`
	Payload             jsontext.Value `json:"payload"`
	CreatedAt           time.Time      `json:"created_at"`
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
	title, body, err := instanceReportNotification(report)
	if err != nil {
		return 0, err
	}
	if title != "" && !report.Republished {
		muted, err := m.attentionWorkspaceMuted(ctx, info.ProfileID, info.WorkspaceID)
		if err != nil {
			return 0, err
		}
		if !muted {
			notification = &OperatorNotification{
				NotificationID: "report:" + report.ID,
				SessionID:      sessionID,
				ProfileID:      info.ProfileID,
				WorkspaceID:    info.WorkspaceID,
				Title:          title,
				Body:           body,
				At:             report.CreatedAt,
			}
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
			m.publishSessionCatalogEventCount(
				CatalogEvent{
					Name:                 CatalogEventNameOperatorNotification,
					ProfileID:            info.ProfileID,
					WorkspaceID:          info.WorkspaceID,
					SessionID:            sessionID,
					OperatorNotification: notification,
				},
			)
		}
		m.PublishInstanceCatalogChanged(info.ProfileID, info.WorkspaceID, report.InstanceID)
	}
	return persisted.Sequence, nil
}

func instanceReportNotification(report InstanceReportEvent) (string, string, error) {
	if len(report.NotificationPayload) > 0 {
		report.Payload = report.NotificationPayload
	}
	if report.Kind == "proposal" {
		var payload struct {
			Proposals []jsontext.Value `json:"proposals"`
			Picks     []struct {
				Actor string `json:"actor"`
			} `json:"picks"`
		}
		if err := json.Unmarshal(report.Payload, &payload); err != nil {
			return "", "", err
		}
		if len(payload.Proposals) > 0 && len(payload.Proposals) == len(payload.Picks) {
			all := true
			for _, pick := range payload.Picks {
				all = all && pick.Actor == "auto_accept"
			}
			if all {
				return "Your agent acted on a check-in", "Review the check-in report.", nil
			}
		}
	}
	if report.Kind == "activity" {
		title := fmt.Sprintf("Your agent made %d changes", report.WritesDone)
		switch report.WritesDone {
		case 1:
			title = "Your agent made 1 change"
		case 0:
			title = "Your agent may have changed something"
		}
		return title, fmt.Sprintf(
			"%d calls may have changed something. Review the check-in report.",
			report.WritesMaybe,
		), nil
	}
	if report.Kind == "proposal" {
		return "New agent proposal", "Review the proposed work in the agent thread.", nil
	}
	if report.Kind != "failure" {
		return "", "", nil
	}
	var failure struct {
		Reason string `json:"reason"`
	}
	if err := json.Unmarshal(report.Payload, &failure); err != nil {
		return "", "", err
	}
	switch failure.Reason {
	case "fatigue":
		return "Proactive checks paused", "Consecutive checks failed. Review the agent report before resuming.", nil
	case "unattended":
		return "Agent paused", "The unattended turn limit was reached. Review the agent report before resuming.", nil
	default:
		return "", "", nil
	}
}

func (m *Manager) reportExhaustedContext(ctx context.Context, sessionID string, event acp.AgentEvent) error {
	if event.Type != acp.EventTypeError || strings.TrimSpace(event.TurnID) == "" {
		return nil
	}
	message := strings.ToLower(event.Error)
	if event.Failure != nil {
		message += " " + strings.ToLower(event.Failure.Summary)
	}
	exhausted := false
	for _, phrase := range []string{
		"context_length_exceeded", "context window exceeded", "context window is full",
		"maximum context length", "prompt is too long", "prompt too long",
	} {
		exhausted = exhausted || strings.Contains(message, phrase)
	}
	if !exhausted {
		return nil
	}
	writer, ok := m.promptAdmissionStore.(interface {
		ReportContextExhaustion(context.Context, string, string) error
	})
	if !ok {
		return nil
	}
	return writer.ReportContextExhaustion(ctx, sessionID, event.TurnID)
}
