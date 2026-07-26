package extensionpkg

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	extensionprotocol "github.com/compozy/agh/internal/extensionprotocol"
	"github.com/compozy/agh/internal/subprocess"
	toolspkg "github.com/compozy/agh/internal/tools"
)

func TestManagerCallToolTracksInvocationForExactCallLifetime(t *testing.T) {
	t.Parallel()

	process := newFakeProcess(123)
	tracker := &extensionToolCallTrackerStub{}
	manager := NewManager(nil, WithExtensionToolCallTracker(tracker))
	manager.extensions["question-tool"] = &managedExtension{
		info: ExtensionInfo{Capabilities: CapabilitiesConfig{
			Provides: []string{extensionprotocol.CapabilityToolProvider},
		}},
		process: process,
		active:  true,
		initialize: &subprocess.InitializeResponse{
			ImplementedMethods: []string{string(extensionprotocol.ExtensionServiceMethodToolsCall)},
		},
	}

	process.callFn = func(_ context.Context, method string, params any, result any) error {
		if method != string(extensionprotocol.ExtensionServiceMethodToolsCall) {
			t.Fatalf("process method = %q, want tools/call", method)
		}
		request, ok := params.(toolspkg.ExtensionToolCallRequest)
		if !ok {
			t.Fatalf("process params = %T, want ExtensionToolCallRequest", params)
		}
		if request.SessionID != "sess-1" || request.InvocationID != "inv-1" {
			t.Fatalf("tracked tool request = %#v", request)
		}
		response := result.(*toolspkg.ExtensionToolCallResponse)
		response.Result = toolspkg.ToolResult{Content: []toolspkg.ToolContent{{Type: "text", Text: "done"}}}
		return nil
	}

	result, err := manager.CallTool(t.Context(), "question-tool", toolspkg.ExtensionToolCallRequest{
		ToolID:    toolspkg.ToolID("ext__question_tool__ask"),
		Handler:   "ask",
		SessionID: "sess-1",
		Input:     json.RawMessage(`{}`),
	})
	if err != nil {
		t.Fatalf("CallTool() error = %v", err)
	}
	if len(result.Content) != 1 || result.Content[0].Text != "done" {
		t.Fatalf("CallTool() result = %#v", result)
	}
	if tracker.finished != 1 {
		t.Fatalf("tracker cleanup count = %d, want 1", tracker.finished)
	}

	process.callFn = func(context.Context, string, any, any) error {
		return errors.New("extension failed")
	}
	_, err = manager.CallTool(t.Context(), "question-tool", toolspkg.ExtensionToolCallRequest{
		ToolID:    toolspkg.ToolID("ext__question_tool__ask"),
		Handler:   "ask",
		SessionID: "sess-1",
		Input:     json.RawMessage(`{}`),
	})
	if err == nil {
		t.Fatal("CallTool() error = nil, want extension failure")
	}
	if tracker.finished != 2 {
		t.Fatalf("tracker cleanup count after error = %d, want 2", tracker.finished)
	}
}

type extensionToolCallTrackerStub struct {
	finished int
}

func (s *extensionToolCallTrackerStub) BeginExtensionToolCall(
	ctx context.Context,
	extensionName string,
	sessionID string,
) (string, func(), error) {
	if ctx == nil || ctx.Err() != nil {
		return "", nil, errors.New("unexpected tracking context")
	}
	if extensionName != "question-tool" || sessionID != "sess-1" {
		return "", nil, errors.New("unexpected tracking scope")
	}
	return "inv-1", func() { s.finished++ }, nil
}
