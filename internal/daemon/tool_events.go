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
	writer store.EventSummaryStore
	now    func() time.Time
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
	return s.writer.WriteEventSummary(context.WithoutCancel(ctx), store.EventSummary{
		ProfileID:   store.DefaultProfileID,
		Type:        eventType,
		WorkspaceID: event.WorkspaceID,
		SessionID:   event.SessionID,
		AgentName:   event.AgentName,
		Outcome:     string(eventspkg.OutcomeFor(eventType)),
		Content:     content,
		Summary:     fmt.Sprintf("%s %s", event.ToolID, event.Kind),
		Timestamp:   timestamp,
	})
}
