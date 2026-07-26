package mcp

import (
	"context"

	"encoding/json"

	"fmt"

	"strings"

	"github.com/compozy/agh/internal/tools"
)

// Bind consumes a launch nonce after validating the Unix peer and expected binary.
func (s *HostedService) Bind(ctx context.Context, req HostedBindRequest, peer PeerInfo) (HostedBindResponse, error) {
	if err := ctxErr(ctx); err != nil {
		return HostedBindResponse{}, err
	}
	if s == nil || !s.enabled {
		return HostedBindResponse{}, ErrHostedDisabled
	}
	sessionID := strings.TrimSpace(req.SessionID)
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
	record, err := s.consumeLaunch(sessionID, nonce, bindID, peer)
	if err != nil {
		return HostedBindResponse{}, err
	}
	projection, err := s.projection(ctx, record)
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
	result, err := registry.Call(ctx, record.scope(), tools.CallRequest{
		ToolID:        toolID,
		ToolCallID:    strings.TrimSpace(req.ToolCallID),
		SessionID:     record.sessionID,
		WorkspaceID:   record.workspaceID,
		AgentName:     record.agentName,
		CorrelationID: strings.TrimSpace(req.CorrelationID),
		Input:         input,
	})
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
	return nil
}
