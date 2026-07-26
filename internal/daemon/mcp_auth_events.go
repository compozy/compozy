package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/compozy/agh/internal/diagnostics"
	eventspkg "github.com/compozy/agh/internal/events"
	mcpauth "github.com/compozy/agh/internal/mcp/auth"
	"github.com/compozy/agh/internal/store"
)

const defaultMCPAuthEventWriteTimeout = 5 * time.Second

type mcpAuthEventWriter interface {
	WriteEventSummary(context.Context, store.EventSummary) error
}

type daemonMCPAuthNotifier struct {
	writer mcpAuthEventWriter
	logger *slog.Logger
	now    func() time.Time
}

type mcpAuthEventPayload struct {
	ServerName  string `json:"server_name,omitempty"`
	Scope       string `json:"scope,omitempty"`
	WorkspaceID string `json:"workspace_id,omitempty"`
	Outcome     string `json:"outcome"`
	ErrorClass  string `json:"error_class,omitempty"`
}

var _ mcpauth.LifecycleNotifier = (*daemonMCPAuthNotifier)(nil)

func (n *daemonMCPAuthNotifier) NotifyMCPAuth(ctx context.Context, lifecycle mcpauth.LifecycleOutcome) {
	if n == nil || n.writer == nil {
		return
	}
	payload := mcpAuthEventPayload{
		ServerName:  lifecycle.Target.ServerName,
		Scope:       string(lifecycle.Target.Scope),
		WorkspaceID: lifecycle.Target.WorkspaceID,
		Outcome:     lifecycle.Outcome,
		ErrorClass:  lifecycle.ErrorClass,
	}
	content, err := json.Marshal(payload)
	if err != nil {
		n.logFailure(lifecycle.Action, "marshal", err)
		return
	}
	eventType, err := mcpAuthEventType(lifecycle.Action)
	if err != nil {
		n.logFailure(lifecycle.Action, "classify", err)
		return
	}
	eventOutcome := eventspkg.OutcomeSuccess
	if lifecycle.Outcome != "success" {
		eventOutcome = eventspkg.OutcomeFailure
	}
	summary := store.EventSummary{
		Type:      eventType,
		Outcome:   string(eventOutcome),
		Content:   content,
		Summary:   fmt.Sprintf("MCP auth %s %s", lifecycle.Action, lifecycle.Outcome),
		Timestamp: n.timestamp(),
	}
	if ctx == nil {
		ctx = context.Background()
	}
	writeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), defaultMCPAuthEventWriteTimeout)
	defer cancel()
	if err := n.writer.WriteEventSummary(writeCtx, summary); err != nil {
		n.logFailure(lifecycle.Action, "write", err)
	}
}

func mcpAuthEventType(action mcpauth.LifecycleAction) (string, error) {
	switch action {
	case mcpauth.LifecycleBegin:
		return eventspkg.MCPAuthBegin, nil
	case mcpauth.LifecycleExchange:
		return eventspkg.MCPAuthExchange, nil
	case mcpauth.LifecycleLogout:
		return eventspkg.MCPAuthLogout, nil
	default:
		return "", fmt.Errorf("unsupported MCP auth lifecycle action %q", action)
	}
}

func (n *daemonMCPAuthNotifier) timestamp() time.Time {
	if n != nil && n.now != nil {
		return n.now().UTC()
	}
	return time.Now().UTC()
}

func (n *daemonMCPAuthNotifier) logFailure(
	action mcpauth.LifecycleAction,
	operation string,
	err error,
) {
	if n == nil || n.logger == nil || err == nil {
		return
	}
	n.logger.Warn(
		"daemon.mcp_auth.event_failed",
		"action", strings.TrimSpace(string(action)),
		"operation", strings.TrimSpace(operation),
		"error", diagnostics.RedactAndBound(err.Error(), 1024),
	)
}
