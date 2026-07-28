package acp

import (
	"context"

	"errors"
	"fmt"

	"strings"
	"time"

	acpsdk "github.com/coder/acp-go-sdk"
)

func (p *AgentProcess) registerPendingPermission(
	turnID string,
	request acpsdk.RequestPermissionRequest,
) (string, *pendingPermission) {
	p.pendingPermissionMu.Lock()
	defer p.pendingPermissionMu.Unlock()

	if p.pendingPermissions == nil {
		p.pendingPermissions = make(map[string]*pendingPermission)
	}
	requestID := p.allocatePermissionRequestIDLocked(turnID, request)
	pending := &pendingPermission{
		requestID:          requestID,
		turnID:             strings.TrimSpace(turnID),
		response:           make(chan permissionDecision, 1),
		supportedDecisions: supportedPermissionDecisions(request.Options),
	}
	p.pendingPermissions[requestID] = pending
	return requestID, pending
}

func (p *AgentProcess) nextPermissionRequestID(turnID string, request acpsdk.RequestPermissionRequest) string {
	p.pendingPermissionMu.Lock()
	defer p.pendingPermissionMu.Unlock()

	if p.pendingPermissions == nil {
		p.pendingPermissions = make(map[string]*pendingPermission)
	}
	return p.allocatePermissionRequestIDLocked(turnID, request)
}

func (p *AgentProcess) allocatePermissionRequestIDLocked(
	turnID string,
	request acpsdk.RequestPermissionRequest,
) string {
	base := strings.TrimSpace(permissionRequestIDFromMeta(request.Meta))
	if base == "" {
		toolCallID := strings.TrimSpace(string(request.ToolCall.ToolCallId))
		if toolCallID != "" {
			base = toolCallID
			if strings.TrimSpace(turnID) != "" {
				base = strings.TrimSpace(turnID) + ":" + toolCallID
			}
		}
	}
	if base == "" {
		name := strings.TrimSpace(permissionRequestName(turnID, request))
		if name != "" {
			base = name
		}
	}
	if base == "" {
		base = EventTypePermission
	}

	requestID := base
	for {
		if _, exists := p.pendingPermissions[requestID]; !exists {
			return requestID
		}
		p.permissionRequestSeq++
		requestID = fmt.Sprintf("%s:%d", base, p.permissionRequestSeq)
	}
}

func (p *AgentProcess) clearPendingPermission(requestID string) {
	p.pendingPermissionMu.Lock()
	defer p.pendingPermissionMu.Unlock()

	if p.pendingPermissions == nil {
		return
	}
	delete(p.pendingPermissions, strings.TrimSpace(requestID))
}

// ResolvePermission delivers a user decision to a waiting permission request.
func (p *AgentProcess) ResolvePermission(req ApproveRequest) error {
	if err := req.Validate(); err != nil {
		return err
	}

	decision, err := parsePermissionDecision(req.Decision)
	if err != nil {
		return err
	}

	p.pendingPermissionMu.Lock()
	requestID, pending, err := p.lookupPendingPermissionLocked(req)
	if err != nil {
		p.pendingPermissionMu.Unlock()
		return err
	}
	if len(pending.supportedDecisions) > 0 {
		if _, ok := pending.supportedDecisions[decision]; !ok {
			p.pendingPermissionMu.Unlock()
			return fmt.Errorf("%w: %s", ErrPermissionDecisionUnsupported, decision)
		}
	}
	delete(p.pendingPermissions, requestID)
	p.pendingPermissionMu.Unlock()

	pending.response <- decision
	return nil
}

// RequestPermission reuses the ACP client-side permission path for daemon-originated tool approvals.
func (p *AgentProcess) RequestPermission(
	ctx context.Context,
	req RequestPermissionRequest,
) (RequestPermissionResponse, error) {
	if p == nil {
		return RequestPermissionResponse{}, errors.New("acp: agent process is required")
	}
	if ctx == nil {
		return RequestPermissionResponse{}, errors.New("acp: permission context is required")
	}
	select {
	case <-p.Done():
		return RequestPermissionResponse{}, errors.New("acp: agent process is stopped")
	default:
	}
	return p.handleRequestPermission(ctx, req)
}

func (p *AgentProcess) permissionTimeoutOrDefault() time.Duration {
	if p.permissionTimeout <= 0 {
		return 5 * time.Minute
	}
	return p.permissionTimeout
}

func (p *AgentProcess) lookupPendingPermissionLocked(req ApproveRequest) (string, *pendingPermission, error) {
	if p.pendingPermissions == nil {
		return "", nil, ErrPendingPermissionNotFound
	}

	if requestID := strings.TrimSpace(req.RequestID); requestID != "" {
		pending, ok := p.pendingPermissions[requestID]
		if !ok {
			return "", nil, fmt.Errorf("%w: %s", ErrPendingPermissionNotFound, requestID)
		}
		return requestID, pending, nil
	}

	turnID := strings.TrimSpace(req.TurnID)
	if turnID == "" {
		return "", nil, ErrPendingPermissionNotFound
	}

	var (
		matchedID string
		matched   *pendingPermission
	)
	for requestID, pending := range p.pendingPermissions {
		if pending == nil || pending.turnID != turnID {
			continue
		}
		if matched != nil {
			return "", nil, fmt.Errorf("%w: %s", ErrPendingPermissionConflict, turnID)
		}
		matchedID = requestID
		matched = pending
	}
	if matched == nil {
		return "", nil, fmt.Errorf("%w: %s", ErrPendingPermissionNotFound, turnID)
	}
	return matchedID, matched, nil
}
