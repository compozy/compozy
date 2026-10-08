package session

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/acp"
)

// ErrCompactionUnsupported reports an agent without a native compaction command.
var ErrCompactionUnsupported = errors.New("session: agent does not advertise a compaction command")

// CompactionRequestResult identifies the accepted maintenance turn.
type CompactionRequestResult struct {
	SessionID string
	PromptID  string
	Command   string
}

const sessionCompactionRequestedEvent = "session.compaction.requested"

type compactionRequestedByKey struct{}

// WithCompactionRequestedBy records the transport initiating a compaction request.
func WithCompactionRequestedBy(ctx context.Context, source string) context.Context {
	return context.WithValue(ctx, compactionRequestedByKey{}, source)
}

// RequestCompaction submits the advertised native command through exclusive prompt admission.
func (m *Manager) RequestCompaction(
	ctx context.Context,
	sessionID string,
) (CompactionRequestResult, <-chan acp.AgentEvent, error) {
	if ctx == nil {
		return CompactionRequestResult{}, nil, errors.New("session: prompt context is required")
	}
	target := strings.TrimSpace(sessionID)
	if target == "" {
		return CompactionRequestResult{}, nil, errors.New("session: session id is required")
	}
	if err := m.checkNewWorkAdmission(ctx); err != nil {
		return CompactionRequestResult{}, nil, err
	}
	session, err := m.lookupPromptSession(ctx, target)
	if err != nil {
		return CompactionRequestResult{}, nil, err
	}
	if session.Info().State != StateActive {
		return CompactionRequestResult{}, nil, fmt.Errorf(
			"%w: %s (%s)",
			ErrSessionNotActive,
			target,
			session.Info().State,
		)
	}
	if session.IsPrompting() {
		return CompactionRequestResult{}, nil, ErrPromptInProgress
	}
	command, ok := ResolveCompactionCommand(session.Info().AdvertisedCommands)
	if !ok {
		return CompactionRequestResult{}, nil, ErrCompactionUnsupported
	}
	req, err := m.parsePromptRequest(
		ctx,
		target,
		PromptOpts{Message: "/" + command, Delivery: PromptDeliveryMaintenance},
	)
	if err != nil {
		return CompactionRequestResult{}, nil, err
	}
	req.resumeStopped = false
	events, err := m.submitPromptRequest(ctx, req)
	if err != nil {
		return CompactionRequestResult{}, nil, err
	}
	return CompactionRequestResult{SessionID: target, PromptID: req.turnID, Command: command}, events, nil
}

func (m *Manager) recordCompactionRequest(ctx context.Context, session *Session, req promptRequest) error {
	source, ok := ctx.Value(compactionRequestedByKey{}).(string)
	if !ok {
		source = ""
	}
	switch source {
	case "cli", "http", "tool", "goal", "web":
	default:
		source = "http"
		if req.meta.Synthetic != nil && req.meta.Synthetic.Goal != nil {
			source = "goal"
		}
		if actingSessionID(ctx) != "" {
			source = "tool"
		}
	}
	raw, err := json.Marshal(struct {
		SessionID   string `json:"session_id"`
		Command     string `json:"command"`
		RequestedBy string `json:"requested_by"`
	}{SessionID: session.ID, Command: strings.TrimPrefix(req.message, "/"), RequestedBy: source})
	if err != nil {
		return err
	}
	event := m.normalizeEvent(
		session,
		req.turnID,
		acp.AgentEvent{Type: sessionCompactionRequestedEvent, Raw: raw, Timestamp: m.now()},
	)
	if err := m.recordEvent(ctx, session, event); err != nil {
		return err
	}
	m.notifyAgentEvent(ctx, session, event)
	return nil
}
