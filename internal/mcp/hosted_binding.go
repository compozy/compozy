package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/tools"
)

// Bind consumes a launch nonce after validating the Unix peer and expected binary.
func (s *HostedService) Bind(
	ctx context.Context,
	req HostedBindRequest,
	peer PeerInfo,
) (response HostedBindResponse, err error) {
	startedAt := time.Now()
	sessionID := strings.TrimSpace(req.SessionID)
	defer func() {
		s.logBindOutcome(sessionID, peer, len(response.Tools), time.Since(startedAt), err)
	}()

	if err := ctxErr(ctx); err != nil {
		return HostedBindResponse{}, err
	}
	if s == nil || !s.enabled {
		return HostedBindResponse{}, ErrHostedDisabled
	}
	if sessionID == "" {
		return HostedBindResponse{}, ErrHostedSessionRequired
	}
	nonce := strings.TrimSpace(req.Nonce)
	if nonce == "" {
		return HostedBindResponse{}, ErrHostedNonceRequired
	}
	if err := s.validatePeer(peer); err != nil {
		return HostedBindResponse{}, err
	}
	bindID, err := s.randomToken(hostedBindBytes)
	if err != nil {
		return HostedBindResponse{}, fmt.Errorf("mcp: mint hosted MCP bind id: %w", err)
	}
	record, err := s.bindLaunch(sessionID, nonce, bindID, peer)
	if err != nil {
		return HostedBindResponse{}, err
	}
	projection, err := s.bootstrapProjection(ctx, record)
	if err != nil {
		s.ReleaseBind(bindID)
		return HostedBindResponse{}, err
	}
	return HostedBindResponse{
		BindID: bindID,
		Scope:  record.scope(),
		Tools:  projection.Tools,
		Digest: projection.Digest,
	}, nil
}

func (s *HostedService) logBindOutcome(
	sessionID string,
	peer PeerInfo,
	toolCount int,
	duration time.Duration,
	err error,
) {
	if s == nil || s.logger == nil {
		return
	}
	attributes := []any{
		"session_id", sessionID,
		"peer_pid", peer.PID,
		"duration_ms", duration.Milliseconds(),
	}
	if err != nil {
		attributes = append(attributes, "reason", hostedBindFailureReason(err))
		s.logger.Warn("mcp: hosted MCP bind failed", attributes...)
		return
	}
	attributes = append(attributes, "tool_count", toolCount)
	s.logger.Info("mcp: hosted MCP bind established", attributes...)
}

func hostedBindFailureReason(err error) string {
	switch {
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return "request_canceled"
	case errors.Is(err, ErrHostedDisabled):
		return "disabled"
	case errors.Is(err, ErrHostedSessionRequired):
		return "session_required"
	case errors.Is(err, ErrHostedNonceRequired):
		return "nonce_required"
	case errors.Is(err, ErrHostedNonceInvalid):
		return "nonce_invalid"
	case errors.Is(err, ErrHostedNonceExpired):
		return "nonce_expired"
	case errors.Is(err, ErrHostedPeerInvalid):
		return "peer_invalid"
	case errors.Is(err, ErrHostedBinaryInvalid):
		return "binary_invalid"
	case errors.Is(err, ErrHostedRegistryRequired):
		return "registry_unavailable"
	default:
		if reason, ok := tools.ReasonOf(err); ok {
			return "registry_" + string(reason)
		}
		return "projection_failed"
	}
}

// Projection returns the current session-callable tool projection for one bind.
func (s *HostedService) Projection(
	ctx context.Context,
	bindID string,
	peer PeerInfo,
) (HostedProjectionResponse, error) {
	record, err := s.recordForBind(ctx, bindID, peer)
	if err != nil {
		return HostedProjectionResponse{}, err
	}
	return s.projection(ctx, record)
}

// Call routes a hosted MCP tool call through the registry dispatch pipeline.
func (s *HostedService) Call(ctx context.Context, req HostedCallRequest, peer PeerInfo) (HostedCallResponse, error) {
	record, err := s.recordForBind(ctx, req.BindID, peer)
	if err != nil {
		return HostedCallResponse{}, err
	}
	toolID := tools.ToolID(strings.TrimSpace(req.ToolName))
	if err := toolID.Validate(); err != nil {
		return HostedCallResponse{}, err
	}
	input := cloneRaw(req.Input)
	if len(input) == 0 {
		input = json.RawMessage(`{}`)
	}
	registry := s.currentRegistry()
	if registry == nil {
		return HostedCallResponse{}, ErrHostedRegistryRequired
	}
	call := tools.CallRequest{
		ToolID:        toolID,
		ToolCallID:    strings.TrimSpace(req.ToolCallID),
		SessionID:     record.sessionID,
		WorkspaceID:   record.workspaceID,
		AgentName:     record.agentName,
		CorrelationID: strings.TrimSpace(req.CorrelationID),
		Input:         input,
	}
	var result tools.ToolResult
	if bootstrap, ok := registry.(hostedBootstrapCallRegistry); ok {
		result, err = bootstrap.BootstrapSessionCall(ctx, record.scope(), call)
	} else {
		result, err = registry.Call(ctx, record.scope(), call)
	}
	if err != nil {
		return HostedCallResponse{}, err
	}
	return HostedCallResponse{Result: result}, nil
}

// ReleaseBind removes one active bind record.
func (s *HostedService) ReleaseBind(bindID string) {
	if s == nil {
		return
	}
	bindID = strings.TrimSpace(bindID)
	if bindID == "" {
		return
	}
	s.mu.Lock()
	delete(s.binds, bindID)
	s.mu.Unlock()
	s.projectionCache.remove(bindID)
}

// ReleaseBindForPeer removes a bind only after validating the requesting peer still owns it.
func (s *HostedService) ReleaseBindForPeer(ctx context.Context, bindID string, peer PeerInfo) error {
	record, err := s.recordForBind(ctx, bindID, peer)
	if err != nil {
		return err
	}
	s.mu.Lock()
	delete(s.binds, strings.TrimSpace(record.bindID))
	s.mu.Unlock()
	s.projectionCache.remove(strings.TrimSpace(record.bindID))
	return nil
}
