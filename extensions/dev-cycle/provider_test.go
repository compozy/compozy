package devcycle

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/subprocess"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

func TestRPCServerShouldCallImportTasksTool(t *testing.T) {
	t.Run("Should return structured import tasks payload over tools call", func(t *testing.T) {
		t.Parallel()

		tasksDir := t.TempDir()
		writeImportTasksManifest(t, tasksDir, compozyTaskManifestVersion, []string{
			"    - from: task_01",
			"      to: task_03",
		})
		writeImportTaskFile(t, tasksDir, "task_01.md", "pending", "First task", "# First\n")
		writeImportTaskFile(t, tasksDir, "task_02.md", "done", "Second task", "# Second\n")
		writeImportTaskFile(t, tasksDir, "task_03.md", "pending", "Third task", "# Third\n")

		response := runToolRPC(t, toolImportTasks, json.RawMessage(fmt.Sprintf(
			`{"pattern":%q}`,
			filepath.Join(tasksDir, "task_*.md"),
		)), nil)
		if response.Error != nil {
			t.Fatalf("tools/call error = %#v, want result", response.Error)
		}
		var output importTasksOutput
		if err := json.Unmarshal(response.Result.Result.Structured, &output); err != nil {
			t.Fatalf("Unmarshal(structured) error = %v", err)
		}
		if output.Count != 2 || len(output.Tasks) != 2 {
			t.Fatalf("structured output = %#v, want two pending tasks", output)
		}
		got := []string{output.Tasks[0].ID, output.Tasks[1].ID}
		want := []string{"task_01", "task_03"}
		if !slices.Equal(got, want) {
			t.Fatalf("task IDs = %#v, want %#v", got, want)
		}
		if response.Result.Result.Bytes <= 0 || response.Result.Result.Preview == "" {
			t.Fatalf("tool result metadata = %#v, want byte count and preview", response.Result.Result)
		}
	})

	t.Run("Should surface missing pattern validation over tools call", func(t *testing.T) {
		t.Parallel()

		response := runToolRPC(t, toolImportTasks, json.RawMessage(`{}`), nil)
		if response.Error == nil {
			t.Fatal("tools/call error = nil, want missing pattern error")
		}
		if response.Error.Code != -32010 || response.Error.Message != "The task import pattern is required." {
			t.Fatalf("tools/call error = %#v, want safe missing-pattern validation", response.Error)
		}
		assertInvalidInputToolError(t, response.Error.Data)
	})

	t.Run("Should surface safe recovery when the task set is missing", func(t *testing.T) {
		t.Parallel()

		workspaceDir := t.TempDir()
		pattern := filepath.Join(workspaceDir, ".compozy", "tasks", "helix-v1-launch", "task_*.md")
		response := runToolRPC(
			t,
			toolImportTasks,
			json.RawMessage(fmt.Sprintf(`{"pattern":%q}`, pattern)),
			nil,
		)
		if response.Error == nil {
			t.Fatal("tools/call error = nil, want missing task set error")
		}
		var toolErr toolspkg.ToolError
		if err := json.Unmarshal(response.Error.Data, &toolErr); err != nil {
			t.Fatalf("Unmarshal(tools/call error data) error = %v", err)
		}
		if toolErr.Operator == nil || strings.Contains(toolErr.Operator.Cause, workspaceDir) {
			t.Fatalf("tools/call error = %#v, want safe path-free operator detail", toolErr)
		}
		if got, want := toolErr.Operator.Cause,
			"No task set matched .compozy/tasks/helix-v1-launch/task_*.md."; got != want {
			t.Fatalf("operator cause = %q, want %q", got, want)
		}
	})

	t.Run("Should reject malformed import task input over tools call", func(t *testing.T) {
		t.Parallel()

		response := runToolRPC(t, toolImportTasks, json.RawMessage(`"not-an-object"`), nil)
		if response.Error == nil || !strings.Contains(response.Error.Message, "decode tool input") {
			t.Fatalf("tools/call error = %#v, want decode validation", response.Error)
		}
	})
}

func TestRPCServerShouldCallReviewArtifactTools(t *testing.T) {
	t.Run("Should return structured field errors without creating a round", func(t *testing.T) {
		t.Parallel()

		workspace := newReviewArtifactWorkspace(t)
		response := runToolRPC(
			t,
			toolWriteArtifacts,
			json.RawMessage(`{"issues":[{"title":"Finding","body":"Evidence","severity":"high"}]}`),
			reviewArtifactScope(workspace),
		)
		if response.Error == nil || !strings.Contains(response.Error.Message, "task_name") {
			t.Fatalf("tools/call error = %#v, want task_name validation", response.Error)
		}
		assertInvalidInputToolError(t, response.Error.Data)
		entries, err := filepath.Glob(filepath.Join(workspace, reviewTasksDir, "*"))
		if err != nil {
			t.Fatalf("Glob(review tasks) error = %v", err)
		}
		if len(entries) != 0 {
			t.Fatalf("review task entries = %#v, want none", entries)
		}
	})
}

func TestRPCServerShouldServeLifecycleMethods(t *testing.T) {
	t.Run("Should initialize with only implemented tool methods", func(t *testing.T) {
		t.Parallel()

		response := runProviderRPC(t, rpcMethodInitialize, subprocess.InitializeRequest{
			ProtocolVersion: "2026-01-01",
			Extension:       subprocess.InitializeExtension{Name: Name, Version: "0.3.1"},
			Capabilities:    subprocess.InitializeCapabilities{Provides: []string{"tool.provider"}},
			Methods: subprocess.InitializeMethods{ExtensionServices: []string{
				rpcMethodToolsCall, rpcMethodProvideTools, "watch/poll",
			}},
		})
		if response.Error != nil {
			t.Fatalf("initialize error = %#v, want result", response.Error)
		}
		var result subprocess.InitializeResponse
		if err := json.Unmarshal(response.Result, &result); err != nil {
			t.Fatalf("Unmarshal(initialize result) error = %v", err)
		}
		want := []string{rpcMethodHealthCheck, rpcMethodProvideTools, rpcMethodShutdown, rpcMethodToolsCall}
		if !slices.Equal(result.ImplementedMethods, want) {
			t.Fatalf("implemented methods = %#v, want %#v", result.ImplementedMethods, want)
		}
		if len(result.WatchSourceKinds) != 0 {
			t.Fatalf("watch source kinds = %#v, want none", result.WatchSourceKinds)
		}
	})

	t.Run("Should provide exactly the surviving runtime descriptors", func(t *testing.T) {
		t.Parallel()

		response := runProviderRPC(t, rpcMethodProvideTools, struct{}{})
		if response.Error != nil {
			t.Fatalf("provide_tools error = %#v, want result", response.Error)
		}
		var result toolspkg.ExtensionProvideToolsResponse
		if err := json.Unmarshal(response.Result, &result); err != nil {
			t.Fatalf("Unmarshal(provide_tools result) error = %v", err)
		}
		handlers := make([]string, 0, len(result.Tools))
		for _, descriptor := range result.Tools {
			handlers = append(handlers, descriptor.Handler)
			if descriptor.InputSchemaDigest == "" || descriptor.OutputSchemaDigest == "" {
				t.Fatalf("descriptor missing schema digests: %#v", descriptor)
			}
			if descriptor.Handler != toolImportTasks &&
				(descriptor.ReadOnly || descriptor.Risk != toolspkg.RiskMutating) {
				t.Fatalf("review descriptor = %#v, want mutating risk", descriptor)
			}
		}
		if want := []string{toolImportTasks, toolWriteArtifacts, toolFinalizeRound}; !slices.Equal(handlers, want) {
			t.Fatalf("descriptor handlers = %#v, want %#v", handlers, want)
		}
	})

	t.Run("Should report healthy and acknowledge shutdown", func(t *testing.T) {
		t.Parallel()

		health := runProviderRPC(t, rpcMethodHealthCheck, struct{}{})
		if health.Error != nil {
			t.Fatalf("health_check error = %#v, want result", health.Error)
		}
		var healthResult subprocess.HealthCheckResponse
		if err := json.Unmarshal(health.Result, &healthResult); err != nil {
			t.Fatalf("Unmarshal(health_check result) error = %v", err)
		}
		if !healthResult.Healthy {
			t.Fatal("health_check healthy = false, want true")
		}
		shutdown := runProviderRPC(t, rpcMethodShutdown, subprocess.ShutdownRequest{Reason: "test"})
		if shutdown.Error != nil {
			t.Fatalf("shutdown error = %#v, want result", shutdown.Error)
		}
	})

	t.Run("Should reject malformed initialize params and unknown methods", func(t *testing.T) {
		t.Parallel()

		malformed := runProviderRPC(t, rpcMethodInitialize, "not-an-object")
		if malformed.Error == nil || malformed.Error.Code != -32602 {
			t.Fatalf("initialize error = %#v, want invalid params", malformed.Error)
		}
		unknown := runProviderRPC(t, "unknown/method", struct{}{})
		if unknown.Error == nil || unknown.Error.Code != -32601 {
			t.Fatalf("unknown method error = %#v, want method not found", unknown.Error)
		}
	})

	t.Run("Should reject unsupported tool handlers", func(t *testing.T) {
		t.Parallel()

		response := runToolRPC(t, "missing_handler", json.RawMessage(`{}`), nil)
		if response.Error == nil || !strings.Contains(response.Error.Message, "unsupported tool handler") {
			t.Fatalf("tools/call error = %#v, want unsupported handler", response.Error)
		}
	})
}

func TestRPCServerShouldValidateProviderIOAndToolResultMetadata(t *testing.T) {
	t.Run("Should require provider streams", func(t *testing.T) {
		t.Parallel()

		var stdout bytes.Buffer
		var nilCtx context.Context
		if err := RunProvider(nilCtx, strings.NewReader(""), &stdout); err == nil ||
			!strings.Contains(err.Error(), "context is required") {
			t.Fatalf("RunProvider(nil context) error = %v", err)
		}
		if err := RunProvider(context.Background(), strings.NewReader(""), nil); err == nil ||
			!strings.Contains(err.Error(), "stdout is required") {
			t.Fatalf("RunProvider(nil stdout) error = %v", err)
		}
		if err := RunProvider(context.Background(), nil, &stdout); err == nil ||
			!strings.Contains(err.Error(), "stdin is required") {
			t.Fatalf("RunProvider(nil stdin) error = %v", err)
		}
	})

	t.Run("Should truncate long tool result previews", func(t *testing.T) {
		t.Parallel()

		result, err := toolResult(map[string]string{"body": strings.Repeat("x", 400)}, time.Now())
		if err != nil {
			t.Fatalf("toolResult() error = %v", err)
		}
		if result.Bytes <= int64(len(result.Preview)) || !strings.HasSuffix(result.Preview, "...") {
			t.Fatalf("toolResult() = %#v, want truncated preview metadata", result)
		}
	})
}

type rpcRawResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  json.RawMessage `json:"result"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcToolCallResponse struct {
	JSONRPC string                             `json:"jsonrpc"`
	ID      json.RawMessage                    `json:"id"`
	Result  toolspkg.ExtensionToolCallResponse `json:"result"`
	Error   *rpcError                          `json:"error,omitempty"`
}

func runToolRPC(
	t *testing.T,
	handler string,
	input json.RawMessage,
	workspace *toolspkg.ExtensionToolWorkspaceScope,
) rpcToolCallResponse {
	t.Helper()
	toolID, err := runtimeToolID(handler)
	if err != nil {
		t.Fatalf("runtimeToolID(%q) error = %v", handler, err)
	}
	request := struct {
		JSONRPC string                            `json:"jsonrpc"`
		ID      int                               `json:"id"`
		Method  string                            `json:"method"`
		Params  toolspkg.ExtensionToolCallRequest `json:"params"`
	}{
		JSONRPC: jsonRPCVersion,
		ID:      1,
		Method:  rpcMethodToolsCall,
		Params: toolspkg.ExtensionToolCallRequest{
			ToolID: toolID, Handler: handler, TrustedWorkspace: workspace, Input: input,
		},
	}
	line, err := json.Marshal(request)
	if err != nil {
		t.Fatalf("Marshal(rpc request) error = %v", err)
	}
	rawResponse := runProviderRPCLine(t, string(line))
	var response rpcToolCallResponse
	if err := json.Unmarshal(rawResponse, &response); err != nil {
		t.Fatalf("Unmarshal(rpc response %q) error = %v", string(rawResponse), err)
	}
	return response
}

func runProviderRPC(t *testing.T, method string, params any) rpcRawResponse {
	t.Helper()
	request := struct {
		JSONRPC string `json:"jsonrpc"`
		ID      int    `json:"id"`
		Method  string `json:"method"`
		Params  any    `json:"params"`
	}{JSONRPC: jsonRPCVersion, ID: 1, Method: method, Params: params}
	line, err := json.Marshal(request)
	if err != nil {
		t.Fatalf("Marshal(rpc request) error = %v", err)
	}
	rawResponse := runProviderRPCLine(t, string(line))
	var response rpcRawResponse
	if err := json.Unmarshal(rawResponse, &response); err != nil {
		t.Fatalf("Unmarshal(rpc response %q) error = %v", string(rawResponse), err)
	}
	return response
}

func runProviderRPCLine(t *testing.T, line string) []byte {
	t.Helper()
	var stdout bytes.Buffer
	if err := RunProvider(context.Background(), strings.NewReader(line+"\n"), &stdout); err != nil {
		t.Fatalf("RunProvider() error = %v", err)
	}
	return bytes.TrimSpace(stdout.Bytes())
}

func assertInvalidInputToolError(t *testing.T, raw json.RawMessage) {
	t.Helper()
	var toolErr toolspkg.ToolError
	if err := json.Unmarshal(raw, &toolErr); err != nil {
		t.Fatalf("Unmarshal(tool error) error = %v", err)
	}
	if toolErr.Code != toolspkg.ErrorCodeInvalidInput || toolErr.Operator == nil ||
		toolErr.Operator.Cause == "" || toolErr.Operator.Recovery == "" {
		t.Fatalf("tool error = %#v, want operator-safe invalid input", toolErr)
	}
}
