package tools_test

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"testing"

	callspkg "github.com/compozy/compozy/internal/calls"
	toolspkg "github.com/compozy/compozy/internal/tools"
	"github.com/compozy/compozy/internal/tools/builtin"
)

func TestRuntimeRegistryDispatchCallsError(t *testing.T) {
	t.Parallel()

	t.Run("Should preserve a concrete calls error through dispatch", func(t *testing.T) {
		t.Parallel()
		descriptor := callDescriptor(t, toolspkg.ToolIDAgentCall)
		callErr := &callspkg.Error{
			Code:    callspkg.CodeChildrenCap,
			Message: "caller reached its child cap",
		}
		provider, err := toolspkg.NewNativeProvider(descriptor.Source, toolspkg.NativeTool{
			Descriptor: descriptor,
			Call: func(context.Context, toolspkg.Scope, toolspkg.CallRequest) (toolspkg.ToolResult, error) {
				return toolspkg.ToolResult{}, callErr
			},
		})
		if err != nil {
			t.Fatalf("NewNativeProvider() error = %v", err)
		}
		catalog, err := builtin.ToolsetCatalog()
		if err != nil {
			t.Fatalf("ToolsetCatalog() error = %v", err)
		}
		registry, err := toolspkg.NewRegistry(
			toolspkg.WithProviders(provider),
			toolspkg.WithPolicyInputs(toolspkg.PolicyInputs{
				SystemPermissionMode: toolspkg.PermissionModeApproveAll,
				ApprovalAvailable:    true,
			}, catalog),
		)
		if err != nil {
			t.Fatalf("NewRegistry() error = %v", err)
		}

		_, err = registry.Call(t.Context(), toolspkg.Scope{}, toolspkg.CallRequest{
			ToolID: descriptor.ID,
			Input:  json.RawMessage(`{"agent":"reviewer","prompt":"work"}`),
		})

		var toolErr *toolspkg.ToolError
		if !errors.As(err, &toolErr) || toolErr.Code != toolspkg.ErrorCode(callspkg.CodeChildrenCap) {
			t.Fatalf("RuntimeRegistry.Call() error = %#v, want %q", err, callspkg.CodeChildrenCap)
		}
		var preserved *callspkg.Error
		if !errors.As(err, &preserved) || preserved != callErr {
			t.Fatalf("RuntimeRegistry.Call() cause = %#v, want original calls error", err)
		}
		if slices.Contains(toolErr.ReasonCodes, toolspkg.ReasonBackendUnhealthy) {
			t.Fatalf("RuntimeRegistry.Call() reasons = %#v, must preserve calls error identity", toolErr.ReasonCodes)
		}
	})
}

func callDescriptor(t *testing.T, id toolspkg.ToolID) toolspkg.Descriptor {
	t.Helper()
	for _, descriptor := range builtin.NativeDescriptors() {
		if descriptor.ID == id {
			return descriptor
		}
	}
	t.Fatalf("builtin descriptor %q not found", id)
	return toolspkg.Descriptor{}
}
