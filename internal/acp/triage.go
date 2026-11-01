package acp

import (
	"errors"
	"fmt"
	"strings"

	acpsdk "github.com/coder/acp-go-sdk"
	compozyconfig "github.com/compozy/compozy/internal/config"
)

const ExecutionPolicyTriage = "triage"

var ErrTriageUnsupported = errors.New("triage_unsupported")

func triageReadOnlyMode(provider string, modes []string) string {
	for _, mode := range modes {
		switch strings.ToLower(strings.TrimSpace(mode)) {
		case "read-only", "read_only", "readonly":
			return mode
		}
	}
	// These providers advertise planning as their read-only execution mode.
	// Keep the alias provider-scoped: an arbitrary provider's "plan" is not a guarantee.
	switch compozyconfig.CanonicalProviderName(provider) {
	case "claude", "opencode":
		for _, mode := range modes {
			if mode == "plan" {
				return mode
			}
		}
	}
	return ""
}

func constrainTriageStart(opts StartOpts) (StartOpts, error) {
	if opts.ExecutionPolicy == "" {
		return opts, nil
	}
	if opts.ExecutionPolicy != ExecutionPolicyTriage {
		return StartOpts{}, fmt.Errorf("unknown execution policy %q", opts.ExecutionPolicy)
	}
	if opts.Inspection || opts.ProviderName == "" {
		return StartOpts{}, ErrTriageUnsupported
	}
	if len(opts.ACPOptions) != 0 {
		return StartOpts{}, ErrTriageUnsupported
	}
	opts.Permissions = compozyconfig.PermissionModeApproveReads
	opts.MCPServers = nil
	opts.ActivateMCPServers = nil
	return opts, nil
}

func (p *AgentProcess) triagePermission(request acpsdk.RequestPermissionRequest) acpsdk.RequestPermissionResponse {
	decision := decisionRejectOnce
	if request.ToolCall.Kind != nil && *request.ToolCall.Kind == acpsdk.ToolKindRead {
		if _, err := p.permissions.resolvePathList(request.ToolCall.Locations); err == nil {
			decision = decisionAllowOnce
		}
	}
	if decision == decisionRejectOnce {
		p.emitTriageDenial(string(request.ToolCall.ToolCallId))
	}
	outcome, _ := selectPermissionOutcome(request.Options, decision)
	return acpsdk.RequestPermissionResponse{Outcome: outcome}
}

func (p *AgentProcess) emitTriageDenial(toolCallID string) {
	turnID := p.activeTurnID()
	requestID := p.nextPermissionRequestID(turnID, acpsdk.RequestPermissionRequest{ToolCall: acpsdk.ToolCallUpdate{ToolCallId: acpsdk.ToolCallId(toolCallID)}})
	p.emitPermissionEvent(p.SessionID, turnID, requestID, "Read-only triage", toolCallID, "", decisionRejectOnce, "triage", nil)
}
