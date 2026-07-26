package extensionpkg

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/compozy/agh/internal/subprocess"
	toolspkg "github.com/compozy/agh/internal/tools"
)

func TestExtensionToolCallErrorShouldRestoreStructuredOperatorFailure(t *testing.T) {
	t.Parallel()

	t.Run("Should restore a trusted tool error envelope from RPC data", func(t *testing.T) {
		t.Parallel()

		toolID := toolspkg.ToolID("ext__dev_cycle__import_tasks")
		remote := toolspkg.NewOperatorToolError(
			toolspkg.ErrorCodeInvalidInput,
			toolID,
			"No task set matched .compozy/tasks/launch/task_*.md.",
			toolspkg.ErrToolInvalidInput,
			"No task set matched .compozy/tasks/launch/task_*.md.",
			"Create the matching task set, then retry the run.",
			toolspkg.ReasonDependencyMissing,
		)
		data, err := json.Marshal(remote)
		if err != nil {
			t.Fatalf("Marshal(remote tool error) error = %v", err)
		}
		rpcErr := subprocess.NewRPCError(-32010, remote.Message, nil)
		rpcErr.Data = data

		restored := extensionToolCallError(toolID, rpcErr)
		var toolErr *toolspkg.ToolError
		if !errors.As(restored, &toolErr) {
			t.Fatalf("extensionToolCallError() type = %T, want *tools.ToolError", restored)
		}
		if toolErr.Code != toolspkg.ErrorCodeInvalidInput || toolErr.ToolID != toolID ||
			toolErr.Operator == nil || toolErr.Operator.Cause != remote.Operator.Cause ||
			toolErr.Operator.Recovery != remote.Operator.Recovery {
			t.Fatalf("extensionToolCallError() = %#v, want restored operator failure", toolErr)
		}
		if !errors.Is(restored, rpcErr) {
			t.Fatal("extensionToolCallError() did not preserve the RPC cause")
		}
	})

	t.Run("Should keep an RPC error generic when structured data is malformed", func(t *testing.T) {
		t.Parallel()

		rpcErr := subprocess.NewRPCError(-32010, "tool failed", nil)
		rpcErr.Data = json.RawMessage(`{"code":"not_a_tool_error","message":"unsafe"}`)

		if got := extensionToolCallError("ext__dev_cycle__import_tasks", rpcErr); got != rpcErr {
			t.Fatalf("extensionToolCallError(malformed) = %T %v, want original RPC error", got, got)
		}
	})
}
