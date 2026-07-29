//go:build integration

package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"testing"

	toolspkg "github.com/compozy/compozy/internal/tools"
)

func TestExtensionCommandPolicyAndAvailabilityParityIntegration(t *testing.T) {
	descriptor := extensionCommandIntegrationDescriptor(t)
	handle := &extensionCommandIntegrationHandle{
		descriptor: descriptor,
		availability: toolspkg.Availability{
			Registered: true,
			Enabled:    true,
			Available:  true,
			Authorized: true,
			Executable: true,
		},
	}
	provider := &extensionCommandIntegrationProvider{descriptor: descriptor, handle: handle}
	registry, err := toolspkg.NewRegistry(
		toolspkg.WithProviders(provider),
		toolspkg.WithPolicyInputs(toolspkg.PolicyInputs{
			SystemPermissionMode: toolspkg.PermissionModeApproveReads,
			ExternalDefault:      toolspkg.ExternalDefaultAsk,
			ApprovalAvailable:    true,
		}, toolspkg.ToolsetCatalog{}),
		toolspkg.WithApprovalBridge(extensionCommandIntegrationApprovalBridge{}),
	)
	if err != nil {
		t.Fatalf("tools.NewRegistry() error = %v", err)
	}

	scope := toolspkg.Scope{
		Operator: true, WorkspaceID: "workspace-a", SessionID: "session-a", AgentName: "agent-a",
	}
	input := json.RawMessage(`{"message":"ship"}`)
	call := func(approvalToken string) (toolspkg.ToolResult, error) {
		return registry.Call(t.Context(), scope, toolspkg.CallRequest{
			ToolID: descriptor.ID, SessionID: scope.SessionID, WorkspaceID: scope.WorkspaceID,
			AgentName: scope.AgentName, ApprovalToken: approvalToken, Input: input,
		})
	}

	t.Run("Should gate command facade and raw invocation identically", func(t *testing.T) {
		_, execErr := call("")
		_, toolErr := call("")
		assertExtensionCommandApprovalError(t, execErr)
		assertExtensionCommandApprovalError(t, toolErr)
		if execErr.Error() != toolErr.Error() {
			t.Fatalf("approval errors differ: exec=%q tool=%q", execErr, toolErr)
		}
		if handle.calls != 0 {
			t.Fatalf("unapproved runtime calls = %d, want 0", handle.calls)
		}

		if _, err := call(extensionCommandIntegrationApprovalToken); err != nil {
			t.Fatalf("approved command facade call error = %v", err)
		}
		if _, err := call(extensionCommandIntegrationApprovalToken); err != nil {
			t.Fatalf("approved raw tool call error = %v", err)
		}
		if handle.calls != 2 {
			t.Fatalf("approved runtime calls = %d, want 2", handle.calls)
		}
	})

	t.Run("Should preserve unavailable backend reason codes", func(t *testing.T) {
		handle.availability = toolspkg.Availability{
			Registered:  true,
			Enabled:     true,
			Authorized:  true,
			ReasonCodes: []toolspkg.ReasonCode{toolspkg.ReasonBackendUnhealthy},
		}
		_, err := call(extensionCommandIntegrationApprovalToken)
		if !errors.Is(err, toolspkg.ErrToolUnavailable) {
			t.Fatalf("unavailable command error = %v, want ErrToolUnavailable", err)
		}
		var typed *toolspkg.ToolError
		if !errors.As(err, &typed) || !slices.Contains(typed.ReasonCodes, toolspkg.ReasonBackendUnhealthy) {
			t.Fatalf("unavailable command error = %#v, want backend_unhealthy", err)
		}
		if handle.calls != 2 {
			t.Fatalf("unavailable runtime calls = %d, want unchanged 2", handle.calls)
		}
	})
}

const extensionCommandIntegrationApprovalToken = "approved-command-token"

type extensionCommandIntegrationApprovalBridge struct{}

func (extensionCommandIntegrationApprovalBridge) RequestToolApproval(
	_ context.Context,
	_ toolspkg.Scope,
	request toolspkg.CallRequest,
	_ *toolspkg.ToolView,
) error {
	if request.ApprovalToken == extensionCommandIntegrationApprovalToken {
		return nil
	}
	return toolspkg.NewToolError(
		toolspkg.ErrorCodeApprovalRequired,
		request.ToolID,
		"tool approval is required",
		toolspkg.ErrToolApprovalRequired,
		toolspkg.ReasonApprovalRequired,
	)
}

type extensionCommandIntegrationProvider struct {
	descriptor toolspkg.Descriptor
	handle     *extensionCommandIntegrationHandle
}

func (p *extensionCommandIntegrationProvider) ID() toolspkg.SourceRef {
	return p.descriptor.Source
}

func (p *extensionCommandIntegrationProvider) List(
	context.Context,
	toolspkg.Scope,
) ([]toolspkg.Descriptor, error) {
	return []toolspkg.Descriptor{p.descriptor}, nil
}

func (p *extensionCommandIntegrationProvider) Resolve(
	_ context.Context,
	_ toolspkg.Scope,
	id toolspkg.ToolID,
) (toolspkg.Handle, bool, error) {
	if id != p.descriptor.ID {
		return nil, false, nil
	}
	return p.handle, true, nil
}

type extensionCommandIntegrationHandle struct {
	descriptor   toolspkg.Descriptor
	availability toolspkg.Availability
	calls        int
}

func (h *extensionCommandIntegrationHandle) Descriptor() toolspkg.Descriptor {
	return h.descriptor
}

func (h *extensionCommandIntegrationHandle) Availability(
	context.Context,
	toolspkg.Scope,
) toolspkg.Availability {
	return h.availability
}

func (h *extensionCommandIntegrationHandle) Call(
	context.Context,
	toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	h.calls++
	return toolspkg.ToolResult{Preview: "approved"}, nil
}

func extensionCommandIntegrationDescriptor(t *testing.T) toolspkg.Descriptor {
	t.Helper()
	descriptor, err := toolspkg.DescriptorWithSchemaDigests(toolspkg.Descriptor{
		ID: "ext__cmd_fixture__approved",
		Backend: toolspkg.BackendRef{
			Kind: toolspkg.BackendExtensionHost, ExtensionID: "cmd-fixture", Handler: "approved",
		},
		Description: "Run an approval-gated command",
		InputSchema: json.RawMessage(
			`{"type":"object","properties":{"message":{"type":"string"}},"required":["message"]}`,
		),
		Source:     toolspkg.SourceRef{Kind: toolspkg.SourceExtension, Owner: "cmd-fixture"},
		Visibility: toolspkg.VisibilityModel,
		Risk:       toolspkg.RiskRead, ReadOnly: true, RequiresInteraction: true, ConcurrencySafe: true,
	})
	if err != nil {
		t.Fatalf("tools.DescriptorWithSchemaDigests() error = %v", err)
	}
	return descriptor
}

func assertExtensionCommandApprovalError(t *testing.T, err error) {
	t.Helper()
	if !errors.Is(err, toolspkg.ErrToolApprovalRequired) {
		t.Fatalf("command approval error = %v, want ErrToolApprovalRequired", err)
	}
	var typed *toolspkg.ToolError
	if !errors.As(err, &typed) || typed.Code != toolspkg.ErrorCodeApprovalRequired ||
		!slices.Contains(typed.ReasonCodes, toolspkg.ReasonApprovalRequired) {
		t.Fatalf("command approval error = %#v, want approval_required contract", err)
	}
}
