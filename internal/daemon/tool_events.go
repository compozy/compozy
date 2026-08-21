package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	eventspkg "github.com/compozy/compozy/internal/events"
	"github.com/compozy/compozy/internal/store"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

type daemonToolEventSink struct {
	writer            store.EventSummaryStore
	now               func() time.Time
	profileForSession func(context.Context, string) (string, error)
}

var _ toolspkg.ToolEventSink = (*daemonToolEventSink)(nil)

func (s *daemonToolEventSink) EmitToolEvent(ctx context.Context, event toolspkg.ToolCallEvent) error {
	if s == nil || s.writer == nil {
		return nil
	}
	if ctx == nil {
		return errors.New("daemon: tool event context is required")
	}
	content, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("daemon: marshal tool event summary: %w", err)
	}
	eventType := strings.TrimSpace(string(event.Kind))
	if eventType == "" {
		return errors.New("daemon: tool event type is required")
	}
	timestamp := time.Now().UTC()
	if s.now != nil {
		timestamp = s.now().UTC()
	}
	profileID := store.DefaultProfileID
	if sessionID := strings.TrimSpace(event.SessionID); sessionID != "" && s.profileForSession != nil {
		profileID, err = s.profileForSession(ctx, sessionID)
		if err != nil {
			return fmt.Errorf("daemon: resolve tool event session profile: %w", err)
		}
		profileID = strings.TrimSpace(profileID)
		if profileID == "" {
			return errors.New("daemon: tool event session profile is required")
		}
	}
	return s.writer.WriteEventSummary(context.WithoutCancel(ctx), daemonEventSummary(store.EventSummary{
		ProfileID:   profileID,
		Type:        eventType,
		WorkspaceID: event.WorkspaceID,
		SessionID:   event.SessionID,
		AgentName:   event.AgentName,
		Outcome:     string(eventspkg.OutcomeFor(eventType)),
		Summary:     fmt.Sprintf("%s %s", event.ToolID, event.Kind),
		Timestamp:   timestamp,
	}, content))
}
