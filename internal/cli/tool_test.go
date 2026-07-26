package cli

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/compozy/agh/internal/api/contract"
	toolspkg "github.com/compozy/agh/internal/tools"
)

func TestToolCommandsRenderJSON(t *testing.T) {
	t.Parallel()

	t.Run("Should render tool list json with scoped query", func(t *testing.T) {
		t.Parallel()

		client := &stubClient{
			listToolsFn: func(_ context.Context, query ToolQuery) (ToolsResponseRecord, error) {
				if query.WorkspaceID != "ws-1" || query.SessionID != "sess-1" || query.AgentName != "coder" {
					t.Fatalf("ListTools query = %#v, want scoped query", query)
				}
				return sampleToolsResponse(), nil
			},
		}
		stdout, _, err := executeRootCommand(
			t,
			newTestDeps(t, client),
			"tool",
			"list",
			"--workspace",
			"ws-1",
			"--session",
			"sess-1",
			"--agent",
			"coder",
			"-o",
			"json",
		)
		if err != nil {
			t.Fatalf("tool list error = %v", err)
		}

		var response ToolsResponseRecord
		decodeJSONOutput(t, stdout, &response)
		if got, want := len(response.Tools), 2; got != want {
			t.Fatalf("tool count = %d, want %d", got, want)
		}
		if got, want := response.Tools[0].Descriptor.ToolID, toolspkg.ToolIDSkillView; got != want {
			t.Fatalf("tool id = %q, want %q", got, want)
		}
		if got, want := response.Tools[1].Availability.ReasonCodes[0], toolspkg.ReasonMCPAuthRequired; got != want {
			t.Fatalf("reason = %q, want %q", got, want)
		}
	})

	t.Run("Should render tool search json with request body", func(t *testing.T) {
		t.Parallel()

		client := &stubClient{
			searchToolsFn: func(_ context.Context, request ToolSearchRequest) (ToolsResponseRecord, error) {
				if request.Query != "skill" ||
					request.Limit != 1 ||
					request.WorkspaceID != "ws-1" ||
					request.SessionID != "sess-1" ||
					request.AgentName != "coder" {
					t.Fatalf("SearchTools request = %#v, want scoped search", request)
				}
				return ToolsResponseRecord{Tools: sampleToolsResponse().Tools[:1]}, nil
			},
		}
		stdout, _, err := executeRootCommand(
			t,
			newTestDeps(t, client),
			"tool",
			"search",
			"skill",
			"--limit",
			"1",
			"--workspace",
			"ws-1",
			"--session",
			"sess-1",
			"--agent",
			"coder",
			"-o",
			"json",
		)
		if err != nil {
			t.Fatalf("tool search error = %v", err)
		}

		var response ToolsResponseRecord
		decodeJSONOutput(t, stdout, &response)
		if got, want := len(response.Tools), 1; got != want {
			t.Fatalf("search tool count = %d, want %d", got, want)
		}
	})

	t.Run("Should render tool info json", func(t *testing.T) {
		t.Parallel()

		client := &stubClient{
			getToolFn: func(_ context.Context, id string, query ToolQuery) (ToolResponseRecord, error) {
				if id != toolspkg.ToolIDSkillView.String() || query.WorkspaceID != "ws-1" {
					t.Fatalf("GetTool(%q, %#v), want skill view in ws-1", id, query)
				}
				return ToolResponseRecord{Tool: sampleToolsResponse().Tools[0]}, nil
			},
		}
		stdout, _, err := executeRootCommand(
			t,
			newTestDeps(t, client),
			"tool",
			"info",
			toolspkg.ToolIDSkillView.String(),
			"--workspace",
			"ws-1",
			"-o",
			"json",
		)
		if err != nil {
			t.Fatalf("tool info error = %v", err)
		}

		var response ToolResponseRecord
		decodeJSONOutput(t, stdout, &response)
		if got, want := response.Tool.Descriptor.ToolID, toolspkg.ToolIDSkillView; got != want {
			t.Fatalf("tool id = %q, want %q", got, want)
		}
	})
}

func TestToolArtifactReadCommand(t *testing.T) {
	t.Parallel()

	artifactURI := toolspkg.ToolArtifactURIPrefix + "art_" + strings.Repeat("a", 64)
	content := []byte(`{"version":"agh.tool-result/v1","tail":"D6-TAIL"}`)
	page := ToolArtifactPageRecord{
		Artifact: toolspkg.ArtifactRef{
			URI:      artifactURI,
			Name:     toolspkg.ToolArtifactName,
			MIMEType: toolspkg.ToolArtifactMIMEType,
			Bytes:    int64(len(content)),
			SHA256:   strings.Repeat("a", 64),
		},
		Offset:     2,
		Bytes:      int64(len(content)),
		TotalBytes: 100,
		DataBase64: base64.StdEncoding.EncodeToString(content),
		NextOffset: 2 + int64(len(content)),
		EOF:        false,
	}
	newClient := func(t *testing.T) *stubClient {
		t.Helper()
		return &stubClient{
			readToolArtifactFn: func(
				_ context.Context,
				workspaceID string,
				gotURI string,
				offset int64,
				limit int64,
			) (ToolArtifactPageRecord, error) {
				if workspaceID != "ws-1" || gotURI != artifactURI || offset != 2 || limit != 64 {
					t.Fatalf(
						"ReadToolArtifact(%q, %q, %d, %d), want ws-1 URI offset=2 limit=64",
						workspaceID,
						gotURI,
						offset,
						limit,
					)
				}
				return page, nil
			},
		}
	}

	t.Run("Should preserve the structured page contract", func(t *testing.T) {
		t.Parallel()

		stdout, _, err := executeRootCommand(
			t,
			newTestDeps(t, newClient(t)),
			"tool",
			"artifact",
			"read",
			artifactURI,
			"--workspace",
			"ws-1",
			"--offset",
			"2",
			"--limit",
			"64",
			"-o",
			"json",
		)
		if err != nil {
			t.Fatalf("tool artifact read error = %v", err)
		}
		var got ToolArtifactPageRecord
		decodeJSONOutput(t, stdout, &got)
		if got != page {
			t.Fatalf("tool artifact page = %#v, want %#v", got, page)
		}
	})

	t.Run("Should decode exact page bytes in human mode", func(t *testing.T) {
		t.Parallel()

		stdout, _, err := executeRootCommand(
			t,
			newTestDeps(t, newClient(t)),
			"tool",
			"artifact",
			"read",
			artifactURI,
			"--workspace",
			"ws-1",
			"--offset",
			"2",
			"--limit",
			"64",
		)
		if err != nil {
			t.Fatalf("tool artifact read error = %v", err)
		}
		if stdout != string(content) {
			t.Fatalf("tool artifact stdout = %q, want exact %q", stdout, content)
		}
	})

	t.Run("Should reject unsupported ranges before calling the daemon", func(t *testing.T) {
		t.Parallel()

		client := &stubClient{
			readToolArtifactFn: func(
				context.Context,
				string,
				string,
				int64,
				int64,
			) (ToolArtifactPageRecord, error) {
				t.Fatal("ReadToolArtifact should not be called for an invalid range")
				return ToolArtifactPageRecord{}, nil
			},
		}
		stdout, _, err := executeRootCommand(
			t,
			newTestDeps(t, client),
			"tool",
			"artifact",
			"read",
			artifactURI,
			"--workspace",
			"ws-1",
			"--limit",
			"65537",
			"-o",
			"json",
		)
		if err == nil {
			t.Fatal("tool artifact invalid range error = nil, want structured error")
		}
		assertToolError(t, stdout, toolspkg.ErrorCodeInvalidInput, toolspkg.ReasonSchemaInvalid)
	})
}

func TestToolInvokeCommandInputs(t *testing.T) {
	t.Parallel()

	t.Run("Should invoke with inline json and redact sensitive result fields", func(t *testing.T) {
		t.Parallel()

		client := &stubClient{
			invokeToolFn: func(_ context.Context, id string, request ToolInvokeRequest) (ToolInvokeResponseRecord, error) {
				if id != toolspkg.ToolIDToolInfo.String() {
					t.Fatalf("InvokeTool id = %q, want %q", id, toolspkg.ToolIDToolInfo)
				}
				if got, want := string(request.Input), `{"tool_id":"agh__skill_view"}`; got != want {
					t.Fatalf("InvokeTool input = %s, want %s", got, want)
				}
				if len(request.SensitiveInputFields) != 1 || request.SensitiveInputFields[0] != "token" {
					t.Fatalf("SensitiveInputFields = %#v, want token", request.SensitiveInputFields)
				}
				return ToolInvokeResponseRecord{
					ToolID:     toolspkg.ToolIDToolInfo,
					Status:     "completed",
					DurationMS: 9,
					Result: toolspkg.ToolResult{
						Preview:    "token=super-secret",
						Structured: json.RawMessage(`{"token":"super-secret","visible":"ok"}`),
						DurationMS: 9,
					},
					Events: []contract.ToolCallEventPayload{},
				}, nil
			},
		}
		stdout, _, err := executeRootCommand(
			t,
			newTestDeps(t, client),
			"tool",
			"invoke",
			toolspkg.ToolIDToolInfo.String(),
			"--input",
			`{"tool_id":"agh__skill_view"}`,
			"--sensitive-input-field",
			"token",
			"-o",
			"json",
		)
		if err != nil {
			t.Fatalf("tool invoke inline error = %v", err)
		}
		if strings.Contains(stdout, "super-secret") {
			t.Fatalf("tool invoke output leaked secret: %s", stdout)
		}

		var response ToolInvokeResponseRecord
		decodeJSONOutput(t, stdout, &response)
		if got, want := compactJSON(response.Result.Structured), `{"token":"[REDACTED]","visible":"ok"}`; got != want {
			t.Fatalf("structured result = %s, want %s", got, want)
		}
	})

	t.Run("Should invoke with json input file", func(t *testing.T) {
		t.Parallel()

		inputPath := filepath.Join(t.TempDir(), "input.json")
		if err := os.WriteFile(inputPath, []byte("{\n  \"tool_id\": \"agh__skill_view\"\n}\n"), 0o600); err != nil {
			t.Fatalf("os.WriteFile(input) error = %v", err)
		}
		client := &stubClient{
			invokeToolFn: func(_ context.Context, _ string, request ToolInvokeRequest) (ToolInvokeResponseRecord, error) {
				if got, want := string(request.Input), `{"tool_id":"agh__skill_view"}`; got != want {
					t.Fatalf("InvokeTool input = %s, want %s", got, want)
				}
				return sampleInvokeResponse(), nil
			},
		}
		stdout, _, err := executeRootCommand(
			t,
			newTestDeps(t, client),
			"tool",
			"invoke",
			toolspkg.ToolIDToolInfo.String(),
			"--input-file",
			inputPath,
			"-o",
			"json",
		)
		if err != nil {
			t.Fatalf("tool invoke file error = %v", err)
		}
		var response ToolInvokeResponseRecord
		decodeJSONOutput(t, stdout, &response)
		if response.Status != "completed" {
			t.Fatalf("status = %q, want completed", response.Status)
		}
	})

	t.Run("Should invoke with json stdin", func(t *testing.T) {
		t.Parallel()

		client := &stubClient{
			invokeToolFn: func(_ context.Context, _ string, request ToolInvokeRequest) (ToolInvokeResponseRecord, error) {
				if got, want := string(request.Input), `{"tool_id":"agh__skill_view"}`; got != want {
					t.Fatalf("InvokeTool input = %s, want %s", got, want)
				}
				return sampleInvokeResponse(), nil
			},
		}
		stdout, _, err := executeRootCommandWithInput(
			t,
			newTestDeps(t, client),
			`{"tool_id":"agh__skill_view"}`,
			"tool",
			"invoke",
			toolspkg.ToolIDToolInfo.String(),
			"-o",
			"json",
		)
		if err != nil {
			t.Fatalf("tool invoke stdin error = %v", err)
		}
		var response ToolInvokeResponseRecord
		decodeJSONOutput(t, stdout, &response)
		if response.Status != "completed" {
			t.Fatalf("status = %q, want completed", response.Status)
		}
	})

	t.Run("Should invoke with dash input file from stdin", func(t *testing.T) {
		t.Parallel()

		client := &stubClient{
			invokeToolFn: func(_ context.Context, _ string, request ToolInvokeRequest) (ToolInvokeResponseRecord, error) {
				if got, want := string(request.Input), `{"tool_id":"agh__skill_view"}`; got != want {
					t.Fatalf("InvokeTool input = %s, want %s", got, want)
				}
				return sampleInvokeResponse(), nil
			},
		}
		stdout, _, err := executeRootCommandWithInput(
			t,
			newTestDeps(t, client),
			`{"tool_id":"agh__skill_view"}`,
			"tool",
			"invoke",
			toolspkg.ToolIDToolInfo.String(),
			"--input-file",
			"-",
			"-o",
			"json",
		)
		if err != nil {
			t.Fatalf("tool invoke dash stdin error = %v", err)
		}
		var response ToolInvokeResponseRecord
		decodeJSONOutput(t, stdout, &response)
		if response.Status != "completed" {
			t.Fatalf("status = %q, want completed", response.Status)
		}
	})

	t.Run("Should invoke with empty object when no input is provided", func(t *testing.T) {
		t.Parallel()

		client := &stubClient{
			invokeToolFn: func(_ context.Context, _ string, request ToolInvokeRequest) (ToolInvokeResponseRecord, error) {
				if got, want := string(request.Input), `{}`; got != want {
					t.Fatalf("InvokeTool input = %s, want %s", got, want)
				}
				return sampleInvokeResponse(), nil
			},
		}
		stdout, _, err := executeRootCommand(
			t,
			newTestDeps(t, client),
			"tool",
			"invoke",
			toolspkg.ToolIDToolInfo.String(),
			"-o",
			"json",
		)
		if err != nil {
			t.Fatalf("tool invoke default input error = %v", err)
		}
		var response ToolInvokeResponseRecord
		decodeJSONOutput(t, stdout, &response)
		if response.Status != "completed" {
			t.Fatalf("status = %q, want completed", response.Status)
		}
	})

	t.Run("Should reject invalid json before invoking client", func(t *testing.T) {
		t.Parallel()

		var invoked bool
		client := &stubClient{
			invokeToolFn: func(context.Context, string, ToolInvokeRequest) (ToolInvokeResponseRecord, error) {
				invoked = true
				return ToolInvokeResponseRecord{}, nil
			},
		}
		stdout, _, err := executeRootCommand(
			t,
			newTestDeps(t, client),
			"tool",
			"invoke",
			toolspkg.ToolIDToolInfo.String(),
			"--input",
			`{"token":"super-secret"`,
			"-o",
			"json",
		)
		if err == nil {
			t.Fatal("tool invoke invalid json error = nil, want structured error")
		}
		if invoked {
			t.Fatal("InvokeTool was called for invalid JSON input")
		}
		if strings.Contains(stdout, "super-secret") {
			t.Fatalf("invalid input error leaked raw input: %s", stdout)
		}
		assertToolError(t, stdout, toolspkg.ErrorCodeInvalidInput, toolspkg.ReasonSchemaInvalid)
	})

	t.Run("Should reject conflicting input sources before invoking client", func(t *testing.T) {
		t.Parallel()

		var invoked bool
		client := &stubClient{
			invokeToolFn: func(context.Context, string, ToolInvokeRequest) (ToolInvokeResponseRecord, error) {
				invoked = true
				return ToolInvokeResponseRecord{}, nil
			},
		}
		stdout, _, err := executeRootCommand(
			t,
			newTestDeps(t, client),
			"tool",
			"invoke",
			toolspkg.ToolIDToolInfo.String(),
			"--input",
			`{"tool_id":"agh__skill_view"}`,
			"--input-file",
			"input.json",
			"-o",
			"json",
		)
		if err == nil {
			t.Fatal("tool invoke conflicting inputs error = nil, want structured error")
		}
		if invoked {
			t.Fatal("InvokeTool was called for conflicting input sources")
		}
		assertToolError(t, stdout, toolspkg.ErrorCodeInvalidInput, toolspkg.ReasonSchemaInvalid)
	})

	t.Run("Should reject unreadable input files before invoking client", func(t *testing.T) {
		t.Parallel()

		var invoked bool
		client := &stubClient{
			invokeToolFn: func(context.Context, string, ToolInvokeRequest) (ToolInvokeResponseRecord, error) {
				invoked = true
				return ToolInvokeResponseRecord{}, nil
			},
		}
		stdout, _, err := executeRootCommand(
			t,
			newTestDeps(t, client),
			"tool",
			"invoke",
			toolspkg.ToolIDToolInfo.String(),
			"--input-file",
			filepath.Join(t.TempDir(), "missing.json"),
			"-o",
			"json",
		)
		if err == nil {
			t.Fatal("tool invoke unreadable file error = nil, want structured error")
		}
		if invoked {
			t.Fatal("InvokeTool was called for unreadable input file")
		}
		assertToolError(t, stdout, toolspkg.ErrorCodeInvalidInput, toolspkg.ReasonSchemaInvalid)
	})

	t.Run("Should reject empty inline input before invoking client", func(t *testing.T) {
		t.Parallel()

		var invoked bool
		client := &stubClient{
			invokeToolFn: func(context.Context, string, ToolInvokeRequest) (ToolInvokeResponseRecord, error) {
				invoked = true
				return ToolInvokeResponseRecord{}, nil
			},
		}
		stdout, _, err := executeRootCommand(
			t,
			newTestDeps(t, client),
			"tool",
			"invoke",
			toolspkg.ToolIDToolInfo.String(),
			"--input",
			"   ",
			"-o",
			"json",
		)
		if err == nil {
			t.Fatal("tool invoke empty inline input error = nil, want structured error")
		}
		if invoked {
			t.Fatal("InvokeTool was called for empty inline input")
		}
		assertToolError(t, stdout, toolspkg.ErrorCodeInvalidInput, toolspkg.ReasonSchemaInvalid)
	})
}

func TestToolsetsCommandsRenderJSON(t *testing.T) {
	t.Parallel()

	t.Run("Should render toolsets list json", func(t *testing.T) {
		t.Parallel()

		client := &stubClient{
			listToolsetsFn: func(_ context.Context, query ToolQuery) (ToolsetsResponseRecord, error) {
				if query.WorkspaceID != "ws-1" {
					t.Fatalf("ListToolsets query = %#v, want ws-1", query)
				}
				return sampleToolsetsResponse(), nil
			},
		}
		stdout, _, err := executeRootCommand(
			t,
			newTestDeps(t, client),
			"toolsets",
			"list",
			"--workspace",
			"ws-1",
			"-o",
			"json",
		)
		if err != nil {
			t.Fatalf("toolsets list error = %v", err)
		}
		var response ToolsetsResponseRecord
		decodeJSONOutput(t, stdout, &response)
		if got, want := len(response.Toolsets), 2; got != want {
			t.Fatalf("toolset count = %d, want %d", got, want)
		}
		if got, want := response.Toolsets[1].Status, "degraded"; got != want {
			t.Fatalf("status = %q, want %q", got, want)
		}
	})

	t.Run("Should render toolsets info json", func(t *testing.T) {
		t.Parallel()

		client := &stubClient{
			getToolsetFn: func(_ context.Context, id string, query ToolQuery) (ToolsetResponseRecord, error) {
				if id != toolspkg.ToolsetIDCatalog.String() || query.SessionID != "sess-1" {
					t.Fatalf("GetToolset(%q, %#v), want catalog in sess-1", id, query)
				}
				return ToolsetResponseRecord{Toolset: sampleToolsetsResponse().Toolsets[0]}, nil
			},
		}
		stdout, _, err := executeRootCommand(
			t,
			newTestDeps(t, client),
			"toolsets",
			"info",
			toolspkg.ToolsetIDCatalog.String(),
			"--session",
			"sess-1",
			"-o",
			"json",
		)
		if err != nil {
			t.Fatalf("toolsets info error = %v", err)
		}
		var response ToolsetResponseRecord
		decodeJSONOutput(t, stdout, &response)
		if got, want := response.Toolset.ID, toolspkg.ToolsetIDCatalog; got != want {
			t.Fatalf("toolset id = %q, want %q", got, want)
		}
	})
}

func TestToolCommandsRenderStructuredErrors(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name       string
		statusCode int
		payload    ToolErrorResponseRecord
		wantCode   toolspkg.ErrorCode
		wantReason toolspkg.ReasonCode
	}{
		{
			name:       "Should render denied errors",
			statusCode: 403,
			payload: toolErrorResponse(
				toolspkg.ErrorCodeDenied,
				toolspkg.ReasonPolicyDenied,
				"tool denied token=super-secret",
				"policy",
			),
			wantCode:   toolspkg.ErrorCodeDenied,
			wantReason: toolspkg.ReasonPolicyDenied,
		},
		{
			name:       "Should render unavailable errors",
			statusCode: 422,
			payload: toolErrorResponse(
				toolspkg.ErrorCodeUnavailable,
				toolspkg.ReasonBackendUnhealthy,
				"tool unavailable",
				"availability",
			),
			wantCode:   toolspkg.ErrorCodeUnavailable,
			wantReason: toolspkg.ReasonBackendUnhealthy,
		},
		{
			name:       "Should render auth required diagnostics",
			statusCode: 422,
			payload: toolErrorResponse(
				toolspkg.ErrorCodeUnavailable,
				toolspkg.ReasonMCPAuthRequired,
				"mcp login required",
				"auth",
			),
			wantCode:   toolspkg.ErrorCodeUnavailable,
			wantReason: toolspkg.ReasonMCPAuthRequired,
		},
		{
			name:       "Should render conflicted errors",
			statusCode: 409,
			payload: toolErrorResponse(
				toolspkg.ErrorCodeConflict,
				toolspkg.ReasonConflictedID,
				"tool id conflict",
				"registry",
			),
			wantCode:   toolspkg.ErrorCodeConflict,
			wantReason: toolspkg.ReasonConflictedID,
		},
		{
			name:       "Should render approval required errors",
			statusCode: 202,
			payload: toolErrorResponse(
				toolspkg.ErrorCodeApprovalRequired,
				toolspkg.ReasonApprovalTokenMissing,
				"tool approval token is required",
				"approval",
			),
			wantCode:   toolspkg.ErrorCodeApprovalRequired,
			wantReason: toolspkg.ReasonApprovalTokenMissing,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			client := &stubClient{
				getToolFn: func(context.Context, string, ToolQuery) (ToolResponseRecord, error) {
					return ToolResponseRecord{}, newToolAPIError(tc.statusCode, "status", tc.payload)
				},
			}
			stdout, _, err := executeRootCommand(
				t,
				newTestDeps(t, client),
				"tool",
				"info",
				toolspkg.ToolIDSkillView.String(),
				"-o",
				"json",
			)
			if err == nil {
				t.Fatal("tool info error = nil, want structured error")
			}
			if strings.Contains(stdout, "super-secret") || strings.Contains(stdout, "approval-token-secret") {
				t.Fatalf("structured error leaked sensitive data: %s", stdout)
			}
			assertToolError(t, stdout, tc.wantCode, tc.wantReason)
		})
	}

	t.Run("Should preserve a safe partial result when artifact persistence fails", func(t *testing.T) {
		t.Parallel()

		partial := toolspkg.ToolResult{
			Content: []toolspkg.ToolContent{{Type: "text", Text: "bounded preview"}},
			Preview: "token=super-secret",
		}
		payload := toolErrorResponse(
			toolspkg.ErrorCodeResultPersistenceFailed,
			toolspkg.ReasonBackendUnhealthy,
			"tool result could not be retained",
			"result",
		)
		payload.Error.PartialResult = &partial
		client := &stubClient{
			invokeToolFn: func(context.Context, string, ToolInvokeRequest) (ToolInvokeResponseRecord, error) {
				return ToolInvokeResponseRecord{}, newToolAPIError(507, "Insufficient Storage", payload)
			},
		}
		stdout, _, err := executeRootCommand(
			t,
			newTestDeps(t, client),
			"tool",
			"invoke",
			toolspkg.ToolIDSkillView.String(),
			"-o",
			"json",
		)
		if err == nil {
			t.Fatal("tool invoke persistence error = nil, want structured error")
		}
		if strings.Contains(stdout, "super-secret") || !strings.Contains(stdout, "bounded preview") {
			t.Fatalf("tool partial result output = %s, want safe bounded preview", stdout)
		}
		var got ToolErrorResponseRecord
		decodeJSONOutput(t, stdout, &got)
		if got.Error.Code != toolspkg.ErrorCodeResultPersistenceFailed || got.Error.PartialResult == nil {
			t.Fatalf("tool partial error = %#v, want persistence code and partial result", got.Error)
		}
	})

	t.Run("Should reject invalid canonical ids before client calls", func(t *testing.T) {
		t.Parallel()

		client := &stubClient{
			getToolFn: func(context.Context, string, ToolQuery) (ToolResponseRecord, error) {
				t.Fatal("GetTool should not be called for invalid canonical id")
				return ToolResponseRecord{}, nil
			},
		}
		stdout, _, err := executeRootCommand(t, newTestDeps(t, client), "tool", "info", "bad.id", "-o", "json")
		if err == nil {
			t.Fatal("tool info invalid id error = nil, want structured error")
		}
		assertToolError(t, stdout, toolspkg.ErrorCodeInvalidInput, toolspkg.ReasonIDInvalidFormat)
	})

	t.Run("Should reject invalid toolset ids before client calls", func(t *testing.T) {
		t.Parallel()

		client := &stubClient{
			getToolsetFn: func(context.Context, string, ToolQuery) (ToolsetResponseRecord, error) {
				t.Fatal("GetToolset should not be called for invalid canonical id")
				return ToolsetResponseRecord{}, nil
			},
		}
		stdout, _, err := executeRootCommand(t, newTestDeps(t, client), "toolsets", "info", "bad.id", "-o", "json")
		if err == nil {
			t.Fatal("toolsets info invalid id error = nil, want structured error")
		}
		assertToolError(t, stdout, toolspkg.ErrorCodeInvalidInput, toolspkg.ReasonIDInvalidFormat)
	})
}

func TestToolCommandPreservesMCPAuthSurface(t *testing.T) {
	t.Parallel()

	t.Run("Should keep auth commands under mcp auth only", func(t *testing.T) {
		t.Parallel()

		root := newRootCommand(commandDeps{})
		toolCmd, _, err := root.Find([]string{"tool"})
		if err != nil {
			t.Fatalf("find tool command: %v", err)
		}
		for _, sub := range toolCmd.Commands() {
			if sub.Name() == "auth" || sub.Name() == "login" || sub.Name() == "logout" || sub.Name() == "status" {
				t.Fatalf("tool command duplicated MCP auth subcommand %q", sub.Name())
			}
			if sub.Name() == "mcp" && !sub.Hidden {
				t.Fatal("tool mcp should remain hidden internal transport")
			}
		}
		mcpAuthStatus, _, err := root.Find([]string{"mcp", "auth", "status"})
		if err != nil {
			t.Fatalf("find mcp auth status command: %v", err)
		}
		if mcpAuthStatus.Name() != "status" {
			t.Fatalf("mcp auth status command = %q, want status", mcpAuthStatus.Name())
		}
	})
}

func TestToolRenderingAndValidationHelpers(t *testing.T) {
	t.Parallel()

	t.Run("Should render human and toon bundles", func(t *testing.T) {
		t.Parallel()

		toolsResponse := sampleToolsResponse()
		listBundle := toolListBundle(toolsResponse)
		listHuman, err := listBundle.human()
		if err != nil {
			t.Fatalf("toolListBundle.human() error = %v", err)
		}
		if !strings.Contains(listHuman, "agh__skill_view") || !strings.Contains(listHuman, "mcp_auth_required") {
			t.Fatalf("tool list human = %q, want tool ids and reasons", listHuman)
		}
		listToon, err := listBundle.toon()
		if err != nil {
			t.Fatalf("toolListBundle.toon() error = %v", err)
		}
		if !strings.Contains(listToon, "tools[2]") {
			t.Fatalf("tool list toon = %q, want table", listToon)
		}

		infoResponse := ToolResponseRecord{Tool: toolsResponse.Tools[0]}
		infoResponse.Tool.Descriptor.OutputSchema = json.RawMessage(`{"type":"object"}`)
		infoBundle := toolInfoBundle(&infoResponse)
		infoHuman, err := infoBundle.human()
		if err != nil {
			t.Fatalf("toolInfoBundle.human() error = %v", err)
		}
		if !strings.Contains(infoHuman, "Output Schema") {
			t.Fatalf("tool info human = %q, want output schema", infoHuman)
		}
		infoToon, err := infoBundle.toon()
		if err != nil {
			t.Fatalf("toolInfoBundle.toon() error = %v", err)
		}
		if !strings.Contains(infoToon, "tool{tool_id") {
			t.Fatalf("tool info toon = %q, want object", infoToon)
		}

		invokeResponse := sampleInvokeResponse()
		invokeResponse.Result.Redactions = []toolspkg.Redaction{
			{Path: "$.token", Reason: toolspkg.ReasonSecretMetadata},
		}
		invokeBundle := toolInvokeBundle(invokeResponse)
		invokeHuman, err := invokeBundle.human()
		if err != nil {
			t.Fatalf("toolInvokeBundle.human() error = %v", err)
		}
		if !strings.Contains(invokeHuman, "Redactions") {
			t.Fatalf("tool invoke human = %q, want redaction count", invokeHuman)
		}
		invokeToon, err := invokeBundle.toon()
		if err != nil {
			t.Fatalf("toolInvokeBundle.toon() error = %v", err)
		}
		if !strings.Contains(invokeToon, "tool_invocation") {
			t.Fatalf("tool invoke toon = %q, want object", invokeToon)
		}

		toolsetsResponse := sampleToolsetsResponse()
		toolsetsBundle := toolsetListBundle(toolsetsResponse)
		toolsetsHuman, err := toolsetsBundle.human()
		if err != nil {
			t.Fatalf("toolsetListBundle.human() error = %v", err)
		}
		if !strings.Contains(toolsetsHuman, "mcp_auth_required") {
			t.Fatalf("toolsets human = %q, want reason", toolsetsHuman)
		}
		toolsetsToon, err := toolsetsBundle.toon()
		if err != nil {
			t.Fatalf("toolsetListBundle.toon() error = %v", err)
		}
		if !strings.Contains(toolsetsToon, "toolsets[2]") {
			t.Fatalf("toolsets toon = %q, want table", toolsetsToon)
		}

		toolsetInfo := toolsetInfoBundle(ToolsetResponseRecord{Toolset: toolsetsResponse.Toolsets[0]})
		toolsetHuman, err := toolsetInfo.human()
		if err != nil {
			t.Fatalf("toolsetInfoBundle.human() error = %v", err)
		}
		if !strings.Contains(toolsetHuman, "Expanded Tools") {
			t.Fatalf("toolset info human = %q, want expanded tools", toolsetHuman)
		}
		toolsetToon, err := toolsetInfo.toon()
		if err != nil {
			t.Fatalf("toolsetInfoBundle.toon() error = %v", err)
		}
		if !strings.Contains(toolsetToon, "toolset{id") {
			t.Fatalf("toolset info toon = %q, want object", toolsetToon)
		}
	})

	t.Run("Should classify tool availability states", func(t *testing.T) {
		t.Parallel()

		testCases := []struct {
			name         string
			availability contract.ToolAvailabilityPayload
			want         string
		}{
			{
				name:         "Should report conflicted",
				availability: contract.ToolAvailabilityPayload{Conflicted: true},
				want:         "conflicted",
			},
			{
				name:         "Should report unregistered",
				availability: contract.ToolAvailabilityPayload{},
				want:         "unregistered",
			},
			{
				name: "Should report disabled",
				availability: contract.ToolAvailabilityPayload{
					Registered: true,
				},
				want: "disabled",
			},
			{
				name: "Should report unavailable",
				availability: contract.ToolAvailabilityPayload{
					Registered: true,
					Enabled:    true,
				},
				want: "unavailable",
			},
			{
				name: "Should report auth required",
				availability: contract.ToolAvailabilityPayload{
					Registered: true,
					Enabled:    true,
					Available:  true,
				},
				want: "auth-required",
			},
			{
				name: "Should report not executable",
				availability: contract.ToolAvailabilityPayload{
					Registered: true,
					Enabled:    true,
					Available:  true,
					Authorized: true,
				},
				want: "not-executable",
			},
			{
				name: "Should report available",
				availability: contract.ToolAvailabilityPayload{
					Registered: true,
					Enabled:    true,
					Available:  true,
					Authorized: true,
					Executable: true,
				},
				want: "available",
			},
		}
		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				if got := toolAvailabilitySummary(tc.availability); got != tc.want {
					t.Fatalf("toolAvailabilitySummary() = %q, want %q", got, tc.want)
				}
			})
		}
	})

	t.Run("Should cover helper edge cases", func(t *testing.T) {
		t.Parallel()

		if got := toolSourceSummary(contract.ToolSourceRefPayload{Kind: toolspkg.SourceBuiltin}); got != "builtin" {
			t.Fatalf("toolSourceSummary(no owner) = %q, want builtin", got)
		}
		if got := joinReasons(
			[]toolspkg.ReasonCode{toolspkg.ReasonPolicyDenied, toolspkg.ReasonPolicyDenied},
			[]toolspkg.ReasonCode{toolspkg.ReasonSchemaInvalid},
		); got != "policy_denied,schema_invalid" {
			t.Fatalf("joinReasons() = %q, want deduped reasons", got)
		}
		if got := strings.Join(
			toolIDsToStrings([]toolspkg.ToolID{toolspkg.ToolIDSkillView}),
			",",
		); got != "agh__skill_view" {
			t.Fatalf("toolIDsToStrings() = %q, want skill view", got)
		}
		if got := strings.Join(
			toolsetIDsToStrings([]toolspkg.ToolsetID{toolspkg.ToolsetIDCatalog}),
			",",
		); got != "agh__catalog" {
			t.Fatalf("toolsetIDsToStrings() = %q, want catalog", got)
		}
		if got := formatBool(false); got != "false" {
			t.Fatalf("formatBool(false) = %q, want false", got)
		}

		commandErr := toolValidationCommandError(
			toolspkg.ToolIDSkillView,
			"tool input is invalid",
			toolspkg.NewValidationError("input", toolspkg.ReasonSchemaInvalid, "bad"),
		)
		if !strings.Contains(commandErr.Error(), "tool input is invalid") {
			t.Fatalf("toolCommandError.Error() = %q, want message", commandErr.Error())
		}
		if got := (*toolCommandError)(nil).Error(); got != nilToolErrorString {
			t.Fatalf("nil toolCommandError.Error() = %q, want <nil>", got)
		}
		fallbackCommandErr := &toolCommandError{
			response: toolErrorResponse(
				toolspkg.ErrorCodeUnavailable,
				toolspkg.ReasonBackendUnhealthy,
				"backend token=super-secret unavailable",
				"registry",
			),
		}
		if got := fallbackCommandErr.Error(); strings.Contains(got, "super-secret") ||
			!strings.Contains(got, string(toolspkg.ErrorCodeUnavailable)) {
			t.Fatalf("fallback toolCommandError.Error() = %q, want redacted unavailable error", got)
		}
		toolErr := toolspkg.NewToolError(
			toolspkg.ErrorCodeDenied,
			toolspkg.ToolIDSkillView,
			"denied",
			toolspkg.ErrToolDenied,
			toolspkg.ReasonPolicyDenied,
		)
		if response, ok := toolErrorResponseForError(toolErr); !ok ||
			response.Error.Code != toolspkg.ErrorCodeDenied {
			t.Fatalf("toolErrorResponseForError(tool) = %#v, %v", response, ok)
		}
		validationErr := toolspkg.NewValidationError("tool_id", toolspkg.ReasonIDInvalidFormat, "bad")
		if response, ok := toolErrorResponseForError(validationErr); !ok ||
			response.Error.ReasonCodes[0] != toolspkg.ReasonIDInvalidFormat {
			t.Fatalf("toolErrorResponseForError(validation) = %#v, %v", response, ok)
		}
		if _, ok := toolErrorResponseForError(os.ErrNotExist); ok {
			t.Fatal("toolErrorResponseForError(os.ErrNotExist) ok = true, want false")
		}
		if err := writeToolCommandError(nil, nil); err != nil {
			t.Fatalf("writeToolCommandError(nil) error = %v, want nil", err)
		}
	})

	t.Run("Should redact invoke metadata fields", func(t *testing.T) {
		t.Parallel()

		response := ToolInvokeResponseRecord{
			ToolID: toolspkg.ToolIDSkillView,
			Status: "completed",
			Result: toolspkg.ToolResult{
				Preview: "authorization=Bearer abc",
				Structured: json.RawMessage(
					`{"password":"super-secret","visible":"ok","completion_tokens":9,"totalTokens":7,"accessToken":"super-secret","apiKey":"super-secret"}`,
				),
				Metadata: map[string]json.RawMessage{
					"access_token": json.RawMessage(`"super-secret"`),
					"refreshToken": json.RawMessage(`"super-secret"`),
					"token_count":  json.RawMessage(`42`),
					"safe":         json.RawMessage(`{"nestedToken":"super-secret","visible":"ok"}`),
				},
				Content: []toolspkg.ToolContent{
					{
						Type: "json",
						Text: "token=super-secret",
						Data: json.RawMessage(`{"secret":"super-secret","visible":"ok"}`),
						Metadata: map[string]json.RawMessage{
							"apiKey": json.RawMessage(`"super-secret"`),
						},
					},
				},
			},
		}
		sanitized := sanitizeToolInvokeResponse(response)
		encoded := string(mustJSON(t, sanitized))
		if strings.Contains(encoded, "super-secret") || strings.Contains(encoded, "Bearer abc") {
			t.Fatalf("sanitizeToolInvokeResponse leaked secret material: %s", encoded)
		}
		if !strings.Contains(encoded, "completion_tokens") || !strings.Contains(encoded, "token_count") ||
			!strings.Contains(encoded, "totalTokens") {
			t.Fatalf("sanitizeToolInvokeResponse removed benign token metrics: %s", encoded)
		}
	})

	t.Run("Should cover tool api and raw json redaction edge cases", func(t *testing.T) {
		t.Parallel()

		if got := (*toolAPIError)(nil).Error(); got != nilToolErrorString {
			t.Fatalf("nil toolAPIError.Error() = %q, want <nil>", got)
		}
		if got := (*toolAPIError)(nil).Response(); got.Error.Code != "" {
			t.Fatalf("nil toolAPIError.Response() = %#v, want empty", got)
		}
		apiErr := newToolAPIError(503, "", ToolErrorResponseRecord{})
		if got := apiErr.Error(); got != "tool_error: HTTP 503" {
			t.Fatalf("fallback toolAPIError.Error() = %q, want HTTP fallback", got)
		}
		if got := string(redactToolRawJSON(nil)); got != "" {
			t.Fatalf("redactToolRawJSON(nil) = %q, want empty", got)
		}
		if got := string(
			redactToolRawJSON(json.RawMessage(`"token=super-secret"`)),
		); strings.Contains(
			got,
			"super-secret",
		) {
			t.Fatalf("redactToolRawJSON(string) leaked secret: %s", got)
		}
		if got := string(
			redactToolRawJSON(json.RawMessage(`{"items":[{"access_token":"super-secret"}]}`)),
		); strings.Contains(
			got,
			"super-secret",
		) {
			t.Fatalf("redactToolRawJSON(array) leaked secret: %s", got)
		}
		invalid := json.RawMessage(`{"token":"super-secret"} trailing`)
		if got := string(redactToolRawJSON(invalid)); got != string(invalid) {
			t.Fatalf("redactToolRawJSON(invalid) = %q, want original invalid payload", got)
		}
	})
}

func executeRootCommandWithInput(
	t *testing.T,
	deps commandDeps,
	stdin string,
	args ...string,
) (string, string, error) {
	t.Helper()

	cmd := newRootCommand(deps)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetIn(strings.NewReader(stdin))
	cmd.SetArgs(args)

	err := cmd.ExecuteContext(t.Context())
	return stdout.String(), stderr.String(), err
}

func decodeJSONOutput(t *testing.T, stdout string, target any) {
	t.Helper()

	if err := json.Unmarshal([]byte(stdout), target); err != nil {
		t.Fatalf("json.Unmarshal(%s) error = %v", stdout, err)
	}
}

func assertToolError(
	t *testing.T,
	stdout string,
	wantCode toolspkg.ErrorCode,
	wantReason toolspkg.ReasonCode,
) {
	t.Helper()

	var response ToolErrorResponseRecord
	decodeJSONOutput(t, stdout, &response)
	if got := response.Error.Code; got != wantCode {
		t.Fatalf("error code = %q, want %q; stdout=%s", got, wantCode, stdout)
	}
	if len(response.Error.ReasonCodes) == 0 || response.Error.ReasonCodes[0] != wantReason {
		t.Fatalf("reason codes = %#v, want first %q; stdout=%s", response.Error.ReasonCodes, wantReason, stdout)
	}
	if len(response.Error.Details) > 0 {
		t.Fatalf("details = %#v, want redacted/omitted", response.Error.Details)
	}
}

func sampleToolsResponse() ToolsResponseRecord {
	return ToolsResponseRecord{Tools: []ToolRecord{
		{
			Descriptor: contract.ToolDescriptorPayload{
				ToolID:       toolspkg.ToolIDSkillView,
				Backend:      contract.ToolBackendRefPayload{Kind: toolspkg.BackendNativeGo, NativeName: "skill_view"},
				DisplayTitle: "Skill View",
				Description:  "Read one skill.",
				InputSchema:  json.RawMessage(`{"type":"object"}`),
				Source: contract.ToolSourceRefPayload{
					Kind:  toolspkg.SourceBuiltin,
					Owner: toolspkg.BuiltinSourceOwner,
				},
				Visibility: toolspkg.VisibilityModel,
				Risk:       toolspkg.RiskRead,
				ReadOnly:   true,
			},
			Availability: contract.ToolAvailabilityPayload{
				Registered: true,
				Enabled:    true,
				Available:  true,
				Authorized: true,
				Executable: true,
			},
			Decision: contract.ToolPolicyDecisionPayload{
				VisibleToOperator: true,
				VisibleToSession:  true,
				Callable:          true,
			},
		},
		{
			Descriptor: contract.ToolDescriptorPayload{
				ToolID: "mcp__github__create_issue",
				Backend: contract.ToolBackendRefPayload{
					Kind:      toolspkg.BackendMCP,
					MCPServer: "github",
					MCPTool:   "create_issue",
				},
				Description: "Create an issue.",
				InputSchema: json.RawMessage(`{"type":"object"}`),
				Source: contract.ToolSourceRefPayload{
					Kind:          toolspkg.SourceMCP,
					Owner:         "github",
					RawServerName: "github",
					RawToolName:   "create_issue",
				},
				Visibility: toolspkg.VisibilityModel,
				Risk:       toolspkg.RiskMutating,
			},
			Availability: contract.ToolAvailabilityPayload{
				Registered:  true,
				Enabled:     true,
				Available:   false,
				Authorized:  false,
				Executable:  true,
				ReasonCodes: []toolspkg.ReasonCode{toolspkg.ReasonMCPAuthRequired},
			},
			Decision: contract.ToolPolicyDecisionPayload{
				VisibleToOperator: true,
				VisibleToSession:  false,
				Callable:          false,
				ReasonCodes:       []toolspkg.ReasonCode{toolspkg.ReasonMCPAuthRequired},
			},
		},
	}}
}

func sampleToolsetsResponse() ToolsetsResponseRecord {
	return ToolsetsResponseRecord{Toolsets: []ToolsetRecord{
		{
			ID:            toolspkg.ToolsetIDCatalog,
			Tools:         []string{"agh__skill_*"},
			Toolsets:      []toolspkg.ToolsetID{toolspkg.ToolsetIDBootstrap},
			ExpandedTools: []toolspkg.ToolID{toolspkg.ToolIDToolList, toolspkg.ToolIDSkillView},
			Status:        "expanded",
		},
		{
			ID:            "mcp__github_read",
			Tools:         []string{"mcp__github__*"},
			ExpandedTools: []toolspkg.ToolID{"mcp__github__search"},
			Status:        "degraded",
			ReasonCodes:   []toolspkg.ReasonCode{toolspkg.ReasonMCPAuthRequired},
		},
	}}
}

func sampleInvokeResponse() ToolInvokeResponseRecord {
	return ToolInvokeResponseRecord{
		ToolID:     toolspkg.ToolIDToolInfo,
		Status:     "completed",
		DurationMS: 4,
		Result: toolspkg.ToolResult{
			Preview:    "agh__skill_view",
			Structured: json.RawMessage(`{"ok":true}`),
			DurationMS: 4,
		},
		Events: []contract.ToolCallEventPayload{},
	}
}

func toolErrorResponse(
	code toolspkg.ErrorCode,
	reason toolspkg.ReasonCode,
	message string,
	layer string,
) ToolErrorResponseRecord {
	return ToolErrorResponseRecord{
		Error: contract.ToolErrorPayload{
			Code:        code,
			Message:     message,
			ToolID:      toolspkg.ToolIDSkillView,
			ReasonCodes: []toolspkg.ReasonCode{reason},
			Layer:       layer,
			Details: map[string]json.RawMessage{
				"approval_token": json.RawMessage(`"approval-token-secret"`),
			},
		},
	}
}
