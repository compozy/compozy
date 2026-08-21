package cli

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/agentidentity"
	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/loop/dsl"
	"github.com/compozy/compozy/internal/network/participation"
	"github.com/compozy/compozy/internal/session"
	"github.com/spf13/cobra"
)

func TestLoopCommandShouldMapCLIVerbsToClient(t *testing.T) {
	t.Parallel()

	t.Run("Should map request response identity flags without positional arguments", func(t *testing.T) {
		t.Parallel()

		var capturedWorkspaceID, capturedRunID, capturedNodeID string
		var capturedRequest contract.RespondLoopRequest
		deps := newTestDeps(t, &stubClient{
			getWorkspaceFn: resolveTestLoopWorkspace(t),
			respondLoopRequestFn: func(
				_ context.Context,
				workspaceID string,
				runID string,
				nodeID string,
				request contract.RespondLoopRequest,
				_ agentidentity.Credentials,
			) (contract.RespondLoopRequestResponse, error) {
				capturedWorkspaceID = workspaceID
				capturedRunID = runID
				capturedNodeID = nodeID
				capturedRequest = request
				return contract.RespondLoopRequestResponse{
					OK: true, RunID: runID, NodeID: nodeID, State: "answered",
				}, nil
			},
		})

		_, _, err := executeRootCommand(
			t,
			deps,
			"loop", "respond",
			"--workspace", "alpha",
			"--run-id", "run-request",
			"--generation", "7",
			"--node", "ask",
			"--item", "3",
			"--decision", "respond",
			"--payload", `{"answer":"yes"}`,
		)
		if err != nil {
			t.Fatalf("executeRootCommand(loop respond) error = %v", err)
		}
		if capturedWorkspaceID != "ws-alpha" || capturedRunID != "run-request" || capturedNodeID != "ask" ||
			capturedRequest.Generation != 7 || capturedRequest.ItemIndex != 3 ||
			capturedRequest.Decision != "respond" ||
			string(capturedRequest.Payload) != `{"answer":"yes"}` {
			t.Fatalf(
				"RespondLoopRequest target/request = %q/%q/%q/%#v",
				capturedWorkspaceID,
				capturedRunID,
				capturedNodeID,
				capturedRequest,
			)
		}
	})

	t.Run("Should reject a positional run argument for request responses", func(t *testing.T) {
		t.Parallel()

		deps := newTestDeps(t, &stubClient{})
		_, _, err := executeRootCommand(t, deps, "loop", "respond", "run-request")
		if err == nil || !strings.Contains(err.Error(), `unknown command "run-request"`) {
			t.Fatalf("executeRootCommand(loop respond positional) error = %v, want no-args rejection", err)
		}
	})

	t.Run("Should preserve editor and temporary-file cleanup failures", func(t *testing.T) {
		t.Parallel()

		cleanupErr := errors.New("remove temporary loop definition")
		var temporaryPath string
		deps := commandDeps{
			removeFile: func(path string) error {
				temporaryPath = path
				return cleanupErr
			},
		}
		definition := testLoopDefinition("release", 7)
		client := &stubClient{
			getLoopFn: func(context.Context, string, string) (contract.LoopResponse, error) {
				return testLoopResponse(t, definition), nil
			},
		}
		cmd := &cobra.Command{Use: "loop-edit-test"}
		cmd.SetContext(t.Context())
		cmd.SetOut(io.Discard)
		cmd.SetErr(io.Discard)

		_, err := editLoopDefinition(
			cmd,
			deps,
			client,
			"workspace-1",
			"release",
			filepath.Join(t.TempDir(), "missing-editor"),
		)
		if temporaryPath == "" {
			t.Fatal("temporary path = empty, want cleanup attempt")
		}
		t.Cleanup(func() {
			if removeErr := os.Remove(temporaryPath); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
				t.Errorf("os.Remove(%q) error = %v", temporaryPath, removeErr)
			}
		})
		if !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("editLoopDefinition() error = %v, want editor error %v", err, os.ErrNotExist)
		}
		if !errors.Is(err, cleanupErr) {
			t.Fatalf("editLoopDefinition() error = %v, want cleanup error %v", err, cleanupErr)
		}
	})

	t.Run("Should render the persisted run URL through every output contract", func(t *testing.T) {
		t.Parallel()

		const webURL = "http://127.0.0.1:43127/loop-runs/run-123"
		deps := newTestDeps(t, &stubClient{
			getWorkspaceFn: resolveTestLoopWorkspace(t),
			getLoopFn:      loopRunDefinitionResponse(t),
			runLoopFn: func(
				context.Context,
				string,
				string,
				contract.RunLoopRequest,
				bool,
				agentidentity.Credentials,
			) (contract.RunLoopResponse, error) {
				return contract.RunLoopResponse{
					Run:    &contract.LoopRunPayload{ID: "run-123"},
					WebURL: webURL,
				}, nil
			},
		})

		human, _, err := executeRootCommand(
			t,
			deps,
			"loop", "run", "--workspace", "alpha", "--name", "release",
		)
		if err != nil {
			t.Fatalf("executeRootCommand(loop run human) error = %v", err)
		}
		lines := strings.Split(strings.TrimSpace(human), "\n")
		if got := lines[len(lines)-1]; got != webURL {
			t.Fatalf("loop run final human line = %q, want %q; output=%q", got, webURL, human)
		}

		jsonOutput, _, err := executeRootCommand(
			t,
			deps,
			"loop", "run", "--workspace", "alpha", "--name", "release", "-o", "json",
		)
		if err != nil {
			t.Fatalf("executeRootCommand(loop run json) error = %v", err)
		}
		var decoded contract.RunLoopResponse
		if err := json.Unmarshal([]byte(jsonOutput), &decoded); err != nil {
			t.Fatalf("json.Unmarshal(loop run response) error = %v", err)
		}
		if decoded.WebURL != webURL {
			t.Fatalf("loop run JSON web_url = %q, want %q", decoded.WebURL, webURL)
		}

		toonOutput, _, err := executeRootCommand(
			t,
			deps,
			"loop", "run", "--workspace", "alpha", "--name", "release", "-o", "toon",
		)
		if err != nil {
			t.Fatalf("executeRootCommand(loop run toon) error = %v", err)
		}
		if !strings.Contains(toonOutput, webURL) || !strings.Contains(toonOutput, "web_url") {
			t.Fatalf("loop run TOON = %q, want web_url %q", toonOutput, webURL)
		}
	})

	t.Run("Should run dry with parsed inputs", func(t *testing.T) {
		t.Parallel()

		var capturedRequest contract.RunLoopRequest
		var capturedDry bool
		var capturedCredentials agentidentity.Credentials
		deps := newTestDeps(t, &stubClient{
			getWorkspaceFn: resolveTestLoopWorkspace(t),
			getLoopFn:      loopRunDefinitionResponse(t),
			getSessionFn: func(context.Context, string) (SessionRecord, error) {
				return SessionRecord{
					ID:          "sess-author",
					AgentName:   "coder",
					WorkspaceID: "ws-alpha",
					State:       session.StateActive,
				}, nil
			},
			runLoopFn: func(
				_ context.Context,
				workspaceID string,
				name string,
				request contract.RunLoopRequest,
				dry bool,
				credentials agentidentity.Credentials,
			) (contract.RunLoopResponse, error) {
				if workspaceID != "ws-alpha" || name != "release" {
					t.Fatalf("RunLoop target = %s/%s, want ws-alpha/release", workspaceID, name)
				}
				capturedRequest = request
				capturedDry = dry
				capturedCredentials = credentials
				return contract.RunLoopResponse{
					DryRun: &contract.LoopPlanPayload{
						LoopName:       "release",
						ResolvedInputs: request.Inputs,
						InputOrigins: map[string]contract.LoopInputOrigin{
							"target":  contract.LoopInputOriginRun,
							"enabled": contract.LoopInputOriginRun,
							"retries": contract.LoopInputOriginRun,
						},
					},
				}, nil
			},
		})
		deps.getenv = testAgentIdentityEnv("sess-author", "coder")

		stdout, _, err := executeRootCommand(
			t,
			deps,
			"loop", "run",
			"--workspace", "alpha",
			"--name", "release",
			"--input", "target=prod",
			"--input", "enabled=true",
			"--input", "retries=3",
			"--runtime", "worker=codex/gpt-5.4@high",
			"--runtime", "id=task_06:codex/openai/gpt-5.4@medium",
			"--runtime", "type=feature:-/claude-sonnet-4-5",
			"--network", "live",
			"--network-channel-strategy", "named",
			"--network-channel", "builders",
			"--network-bounds", `{"max_wakes":3}`,
			"--dry-run",
			"-o", "json",
		)
		if err != nil {
			t.Fatalf("executeRootCommand(loop run) error = %v", err)
		}

		if !capturedDry {
			t.Fatal("RunLoop dry = false, want true")
		}
		if capturedCredentials.SessionID != "sess-author" || capturedCredentials.AgentName != "coder" {
			t.Fatalf("RunLoop credentials = %#v, want sess-author/coder", capturedCredentials)
		}
		if capturedRequest.Inputs["target"] != "prod" || capturedRequest.Inputs["enabled"] != true {
			t.Fatalf("RunLoop inputs = %#v, want parsed string/bool", capturedRequest.Inputs)
		}
		if got, ok := capturedRequest.Inputs["retries"].(json.Number); !ok || got.String() != "3" {
			t.Fatalf("RunLoop retries = %#v, want json.Number(3)", capturedRequest.Inputs["retries"])
		}
		overrides := capturedRequest.ConfigOverrides
		if overrides == nil || overrides.RuntimeDefaults == nil ||
			overrides.RuntimeDefaults.Worker.Provider != "codex" ||
			overrides.RuntimeDefaults.Worker.Model != "gpt-5.4" ||
			overrides.RuntimeDefaults.Worker.Reasoning != "high" {
			t.Fatalf("RunLoop runtime defaults = %#v, want codex/gpt-5.4@high", overrides)
		}
		if len(overrides.RuntimeRules) != 2 ||
			overrides.RuntimeRules[0].Match.ID != "task_06" ||
			overrides.RuntimeRules[0].Runtime.Provider != "codex" ||
			overrides.RuntimeRules[0].Runtime.Model != "openai/gpt-5.4" ||
			overrides.RuntimeRules[0].Runtime.Reasoning != "medium" ||
			overrides.RuntimeRules[1].Match.Type != "feature" ||
			overrides.RuntimeRules[1].Runtime.Provider != "" ||
			overrides.RuntimeRules[1].Runtime.Model != "claude-sonnet-4-5" {
			t.Fatalf("RunLoop runtime rules = %#v, want ordered ID then type rules", overrides.RuntimeRules)
		}
		participationRequest := capturedRequest.NetworkParticipation
		if participationRequest == nil ||
			participationRequest.Mode == nil || *participationRequest.Mode != participation.ModeLive ||
			participationRequest.ChannelStrategy == nil ||
			*participationRequest.ChannelStrategy != participation.StrategyNamed ||
			participationRequest.ChannelID == nil || *participationRequest.ChannelID != "builders" ||
			participationRequest.Bounds == nil || participationRequest.Bounds.MaxWakes == nil ||
			*participationRequest.Bounds.MaxWakes != 3 {
			t.Fatalf("RunLoop network participation = %#v, want bounded Live builders request", participationRequest)
		}
		var response contract.RunLoopResponse
		if err := json.Unmarshal([]byte(stdout), &response); err != nil {
			t.Fatalf("json.Unmarshal(loop run response) error = %v", err)
		}
		if response.DryRun == nil || response.DryRun.LoopName != "release" {
			t.Fatalf("response.DryRun = %#v, want release preview", response.DryRun)
		}
		if response.WebURL != "" {
			t.Fatalf("dry-run web_url = %q, want omitted", response.WebURL)
		}

		human, _, err := executeRootCommand(
			t,
			deps,
			"loop", "run",
			"--workspace", "alpha",
			"--name", "release",
			"--input", "target=prod",
			"--input", "enabled=true",
			"--input", "retries=3",
			"--dry-run",
		)
		if err != nil {
			t.Fatalf("executeRootCommand(loop run dry human) error = %v", err)
		}
		if !strings.Contains(human, `Input enabled=true (origin: run)`) ||
			!strings.Contains(human, `Input target="prod" (origin: run)`) ||
			strings.Contains(human, "/loop-runs/") {
			t.Fatalf("loop run dry human = %q, want effective origins without a link", human)
		}

		toonOutput, _, err := executeRootCommand(
			t,
			deps,
			"loop", "run",
			"--workspace", "alpha",
			"--name", "release",
			"--input", "target=prod",
			"--input", "enabled=true",
			"--input", "retries=3",
			"--dry-run",
			"-o", "toon",
		)
		if err != nil {
			t.Fatalf("executeRootCommand(loop run dry toon) error = %v", err)
		}
		if !strings.Contains(toonOutput, "input_origins") || strings.Contains(toonOutput, "web_url") {
			t.Fatalf("loop run dry TOON = %q, want input origins without web_url", toonOutput)
		}
	})

	t.Run("Should publish file with expected version", func(t *testing.T) {
		t.Parallel()

		definition := testLoopDefinition("release", 7)
		definitionPath := writeTestLoopDefinition(t, definition)
		var capturedRequest contract.PatchLoopRequest
		deps := newTestDeps(t, &stubClient{
			getWorkspaceFn: resolveTestLoopWorkspace(t),
			patchLoopFn: func(
				_ context.Context,
				workspaceID string,
				name string,
				request contract.PatchLoopRequest,
				_ agentidentity.Credentials,
			) (contract.LoopResponse, error) {
				if workspaceID != "ws-alpha" || name != "release" {
					t.Fatalf("PatchLoop target = %s/%s, want ws-alpha/release", workspaceID, name)
				}
				capturedRequest = request
				return testLoopResponse(t, definition), nil
			},
		})

		stdout, _, err := executeRootCommand(
			t,
			deps,
			"loop", "create",
			"--workspace", "alpha",
			"--file", definitionPath,
			"--expected-version", "7",
			"-o", "json",
		)
		if err != nil {
			t.Fatalf("executeRootCommand(loop create --expected-version) error = %v", err)
		}

		if capturedRequest.ExpectedVersion == nil || *capturedRequest.ExpectedVersion != 7 {
			t.Fatalf("ExpectedVersion = %#v, want 7", capturedRequest.ExpectedVersion)
		}
		if capturedRequest.Definition.Meta.Name != "release" {
			t.Fatalf("Definition.Meta.Name = %q, want release", capturedRequest.Definition.Meta.Name)
		}
		var response contract.LoopResponse
		if err := json.Unmarshal([]byte(stdout), &response); err != nil {
			t.Fatalf("json.Unmarshal(loop create response) error = %v", err)
		}
		if response.Loop.Version != 7 {
			t.Fatalf("response.Loop.Version = %d, want 7", response.Loop.Version)
		}
	})

	t.Run("Should configure set flags", func(t *testing.T) {
		t.Parallel()

		var capturedRequest contract.PutLoopConfigRequest
		deps := newTestDeps(t, &stubClient{
			getWorkspaceFn: resolveTestLoopWorkspace(t),
			putLoopConfigFn: func(
				_ context.Context,
				workspaceID string,
				name string,
				request contract.PutLoopConfigRequest,
				_ agentidentity.Credentials,
			) (contract.LoopConfigResponse, error) {
				if workspaceID != "ws-alpha" || name != "release" {
					t.Fatalf("PutLoopConfig target = %s/%s, want ws-alpha/release", workspaceID, name)
				}
				capturedRequest = request
				return contract.LoopConfigResponse{Config: &request.Config}, nil
			},
		})

		if _, _, err := executeRootCommand(
			t,
			deps,
			"loop", "configure",
			"--workspace", "alpha",
			"--name", "release",
			"--set", "iteration_cap=9",
			"--set", "human_gate_enabled=true",
			"-o", "json",
		); err != nil {
			t.Fatalf("executeRootCommand(loop configure) error = %v", err)
		}

		if capturedRequest.Config.IterationCap == nil || *capturedRequest.Config.IterationCap != 9 {
			t.Fatalf("IterationCap = %#v, want 9", capturedRequest.Config.IterationCap)
		}
		if capturedRequest.Config.HumanGateEnabled == nil || !*capturedRequest.Config.HumanGateEnabled {
			t.Fatalf("HumanGateEnabled = %#v, want true", capturedRequest.Config.HumanGateEnabled)
		}
	})

	for _, testCase := range []struct {
		name           string
		fileName       string
		body           string
		wantChecksJSON string
	}{
		{
			name:     "JSON",
			fileName: "loop-config.json",
			body:     `{"iteration_cap":9,"no_progress_window":4,"gate_max_revisions":2}`,
		},
		{
			name:     "YAML",
			fileName: "loop-config.yaml",
			body: "iteration_cap: 9\nno_progress_window: 4\ngate_max_revisions: 2\n" +
				"enabled_checks_json:\n  project_check:\n    enabled: false\n    command: go test ./internal/loop\n",
			wantChecksJSON: `{"project_check":{"command":"go test ./internal/loop","enabled":false}}`,
		},
	} {
		t.Run("Should load snake case loop config from "+testCase.name, func(t *testing.T) {
			t.Parallel()

			configPath := filepath.Join(t.TempDir(), testCase.fileName)
			if err := os.WriteFile(configPath, []byte(testCase.body), 0o600); err != nil {
				t.Fatalf("os.WriteFile(loop config) error = %v", err)
			}

			var capturedRun contract.RunLoopRequest
			var capturedPut contract.PutLoopConfigRequest
			deps := newTestDeps(t, &stubClient{
				getWorkspaceFn: resolveTestLoopWorkspace(t),
				getLoopFn:      loopRunDefinitionResponse(t),
				runLoopFn: func(
					_ context.Context,
					_ string,
					_ string,
					request contract.RunLoopRequest,
					_ bool,
					_ agentidentity.Credentials,
				) (contract.RunLoopResponse, error) {
					capturedRun = request
					return contract.RunLoopResponse{Run: &contract.LoopRunPayload{ID: "run-1"}}, nil
				},
				putLoopConfigFn: func(
					_ context.Context,
					_ string,
					_ string,
					request contract.PutLoopConfigRequest,
					_ agentidentity.Credentials,
				) (contract.LoopConfigResponse, error) {
					capturedPut = request
					return contract.LoopConfigResponse{Config: &request.Config}, nil
				},
			})

			if _, _, err := executeRootCommand(
				t,
				deps,
				"loop", "run",
				"--workspace", "alpha",
				"--name", "release",
				"--config-file", configPath,
				"-o", "json",
			); err != nil {
				t.Fatalf("executeRootCommand(loop run --config-file) error = %v", err)
			}
			if _, _, err := executeRootCommand(
				t,
				deps,
				"loop", "configure",
				"--workspace", "alpha",
				"--name", "release",
				"--file", configPath,
				"-o", "json",
			); err != nil {
				t.Fatalf("executeRootCommand(loop configure --file) error = %v", err)
			}

			assertConfig := func(label string, cfg *contract.LoopConfig) {
				t.Helper()
				if cfg == nil {
					t.Fatalf("%s config = nil", label)
				}
				if cfg.IterationCap == nil || *cfg.IterationCap != 9 {
					t.Fatalf("%s IterationCap = %#v, want 9", label, cfg.IterationCap)
				}
				if cfg.NoProgressWindow == nil || *cfg.NoProgressWindow != 4 {
					t.Fatalf("%s NoProgressWindow = %#v, want 4", label, cfg.NoProgressWindow)
				}
				if cfg.GateMaxRevisions == nil || *cfg.GateMaxRevisions != 2 {
					t.Fatalf("%s GateMaxRevisions = %#v, want 2", label, cfg.GateMaxRevisions)
				}
				if testCase.wantChecksJSON != "" {
					var gotChecks, wantChecks any
					if err := json.Unmarshal(cfg.EnabledChecks, &gotChecks); err != nil {
						t.Fatalf("json.Unmarshal(%s EnabledChecks) error = %v", label, err)
					}
					if err := json.Unmarshal([]byte(testCase.wantChecksJSON), &wantChecks); err != nil {
						t.Fatalf("json.Unmarshal(want EnabledChecks) error = %v", err)
					}
					if !reflect.DeepEqual(gotChecks, wantChecks) {
						t.Fatalf("%s EnabledChecks = %s, want %s", label, cfg.EnabledChecks, testCase.wantChecksJSON)
					}
				}
			}
			assertConfig("run", capturedRun.ConfigOverrides)
			assertConfig("configure", &capturedPut.Config)
		})
	}

	t.Run("Should reject unknown configuration fields before daemon mutation", func(t *testing.T) {
		t.Parallel()

		configPath := filepath.Join(t.TempDir(), "loop-config.yaml")
		if err := os.WriteFile(configPath, []byte("iteration_cap: 9\nunknown_field: true\n"), 0o600); err != nil {
			t.Fatalf("os.WriteFile(loop config) error = %v", err)
		}
		putCalls := 0
		deps := newTestDeps(t, &stubClient{
			getWorkspaceFn: resolveTestLoopWorkspace(t),
			putLoopConfigFn: func(
				context.Context,
				string,
				string,
				contract.PutLoopConfigRequest,
				agentidentity.Credentials,
			) (contract.LoopConfigResponse, error) {
				putCalls++
				return contract.LoopConfigResponse{}, nil
			},
		})

		for _, args := range [][]string{
			{"loop", "configure", "--workspace", "alpha", "--name", "release", "--set", "unknown_field=true"},
			{"loop", "configure", "--workspace", "alpha", "--name", "release", "--file", configPath},
		} {
			_, _, err := executeRootCommand(t, deps, args...)
			assertErrorContains(t, err, "unknown_field")
		}
		if putCalls != 0 {
			t.Fatalf("PutLoopConfig calls = %d, want 0", putCalls)
		}
	})

	t.Run("Should approve gate decision", func(t *testing.T) {
		t.Parallel()

		var capturedRequest contract.ApproveLoopRunRequest
		var capturedCredentials agentidentity.Credentials
		deps := newTestDeps(t, &stubClient{
			getWorkspaceFn: resolveTestLoopWorkspace(t),
			getSessionFn: func(context.Context, string) (SessionRecord, error) {
				return SessionRecord{
					ID:          "sess-author",
					AgentName:   "coder",
					WorkspaceID: "ws-alpha",
					State:       session.StateActive,
				}, nil
			},
			approveLoopRunFn: func(
				_ context.Context,
				workspaceID string,
				runID string,
				request contract.ApproveLoopRunRequest,
				credentials agentidentity.Credentials,
			) error {
				if workspaceID != "ws-alpha" || runID != "looprun-1" {
					t.Fatalf("ApproveLoopRun target = %s/%s, want ws-alpha/looprun-1", workspaceID, runID)
				}
				capturedRequest = request
				capturedCredentials = credentials
				return nil
			},
		})
		deps.getenv = testAgentIdentityEnv("sess-author", "coder")

		if _, _, err := executeRootCommand(
			t,
			deps,
			"loop", "approve",
			"looprun-1",
			"--workspace", "alpha",
			"--gate", "human-review",
			"-o", "json",
		); err != nil {
			t.Fatalf("executeRootCommand(loop approve) error = %v", err)
		}

		if capturedRequest.GateID != "human-review" ||
			capturedRequest.Decision != contract.LoopGateDecisionApprove {
			t.Fatalf("ApproveLoopRun request = %#v, want human-review/approve", capturedRequest)
		}
		if capturedCredentials.SessionID != "sess-author" || capturedCredentials.AgentName != "coder" {
			t.Fatalf("ApproveLoopRun credentials = %#v, want sess-author/coder", capturedCredentials)
		}
	})

	t.Run("Should pass origin filters to Loop run discovery", func(t *testing.T) {
		t.Parallel()

		var captured LoopRunListQuery
		deps := newTestDeps(t, &stubClient{
			getWorkspaceFn: resolveTestLoopWorkspace(t),
			listLoopRunsFn: func(
				_ context.Context,
				workspaceID string,
				query LoopRunListQuery,
			) (contract.LoopRunsResponse, error) {
				if workspaceID != "ws-alpha" {
					t.Fatalf("ListLoopRuns workspace = %q, want ws-alpha", workspaceID)
				}
				captured = query
				return contract.LoopRunsResponse{Runs: []contract.LoopRunPayload{}}, nil
			},
		})

		if _, _, err := executeRootCommand(
			t,
			deps,
			"loop", "runs",
			"--workspace", "alpha",
			"--origin", "session",
			"--origin-session", "session-origin",
			"--cursor", "cursor-next",
			"--limit", "7",
			"-o", "json",
		); err != nil {
			t.Fatalf("executeRootCommand(loop runs origin) error = %v", err)
		}
		if captured.Origin != "session" || captured.OriginSession != "session-origin" ||
			captured.Cursor != "cursor-next" || captured.Limit != 7 {
			t.Fatalf("ListLoopRuns query = %#v", captured)
		}
		values := loopRunValues(captured)
		if values.Get("origin") != "session" || values.Get("origin_session") != "session-origin" ||
			values.Get("cursor") != "cursor-next" {
			t.Fatalf("loopRunValues() = %v", values)
		}
	})

	t.Run("Should send the documented default Loop run page size", func(t *testing.T) {
		t.Parallel()
		var captured LoopRunListQuery
		deps := newTestDeps(t, &stubClient{
			getWorkspaceFn: resolveTestLoopWorkspace(t),
			listLoopRunsFn: func(
				_ context.Context,
				_ string,
				query LoopRunListQuery,
			) (contract.LoopRunsResponse, error) {
				captured = query
				return contract.LoopRunsResponse{}, nil
			},
		})
		if _, _, err := executeRootCommand(t, deps, "loop", "runs", "--workspace", "alpha", "-o", "json"); err != nil {
			t.Fatalf("executeRootCommand(loop runs default limit) error = %v", err)
		}
		if captured.Limit != 50 || loopRunValues(captured).Get("limit") != "50" {
			t.Fatalf("default Loop run query = %#v, want limit 50", captured)
		}
	})

	t.Run("Should render Loop status detail with best and generation history fields", func(t *testing.T) {
		t.Parallel()

		bestGeneration := int64(2)
		bestScore := 0.87
		verdictScore := 0.87
		rank := 0
		response := contract.LoopRunResponse{
			Run: contract.LoopRunPayload{
				ID:             "run-detail",
				WorkspaceID:    "ws-alpha",
				LoopName:       "release",
				Status:         contract.LoopRunStatusRunning,
				Generation:     2,
				BestGeneration: &bestGeneration,
				BestScore:      &bestScore,
			},
			Generations: []contract.LoopGenerationPayload{{
				Generation:       2,
				ParentGeneration: 1,
				Origin:           contract.LoopGenerationOriginGateRevise,
				Verdicts: []contract.LoopGateVerdictPayload{{
					GateID:         "quality",
					Outcome:        contract.LoopGateVerdictRejected,
					Score:          &verdictScore,
					RouteCauseRank: &rank,
				}},
				Outputs: []contract.LoopGenerationOutput{},
			}},
		}
		deps := newTestDeps(t, &stubClient{
			getWorkspaceFn: resolveTestLoopWorkspace(t),
			getLoopRunFn: func(context.Context, string, string) (contract.LoopRunResponse, error) {
				return response, nil
			},
		})

		stdout, _, err := executeRootCommand(
			t,
			deps,
			"loop", "status", "--workspace", "alpha", "--run-id", "run-detail", "-o", "json",
		)
		if err != nil {
			t.Fatalf("executeRootCommand(loop status json) error = %v", err)
		}
		var decoded contract.LoopRunResponse
		if err := json.Unmarshal([]byte(stdout), &decoded); err != nil {
			t.Fatalf("json.Unmarshal(loop status) error = %v", err)
		}
		if decoded.Run.BestGeneration == nil || *decoded.Run.BestGeneration != bestGeneration ||
			decoded.Run.BestScore == nil || *decoded.Run.BestScore != bestScore {
			t.Fatalf(
				"loop status best = %#v/%#v, want %d/%.2f",
				decoded.Run.BestGeneration,
				decoded.Run.BestScore,
				bestGeneration,
				bestScore,
			)
		}
		if len(decoded.Generations) != 1 || decoded.Generations[0].ParentGeneration != 1 ||
			decoded.Generations[0].Origin != contract.LoopGenerationOriginGateRevise ||
			len(decoded.Generations[0].Verdicts) != 1 {
			t.Fatalf("loop status generations = %#v, want provenance and verdict detail", decoded.Generations)
		}

		response.Run.BestGeneration = nil
		response.Run.BestScore = nil
		stdout, _, err = executeRootCommand(
			t,
			deps,
			"loop", "status", "--workspace", "alpha", "--run-id", "run-unset", "-o", "json",
		)
		if err != nil {
			t.Fatalf("executeRootCommand(loop status unset json) error = %v", err)
		}
		var raw struct {
			Run map[string]any `json:"run"`
		}
		if err := json.Unmarshal([]byte(stdout), &raw); err != nil {
			t.Fatalf("json.Unmarshal(loop status unset) error = %v", err)
		}
		if _, exists := raw.Run["best_generation"]; exists {
			t.Fatalf("loop status unset best_generation = %#v, want omitted", raw.Run)
		}
		if _, exists := raw.Run["best_score"]; exists {
			t.Fatalf("loop status unset best_score = %#v, want omitted", raw.Run)
		}
	})

	t.Run("Should render one summary-only Loop run per JSONL row", func(t *testing.T) {
		t.Parallel()

		bestGeneration := int64(3)
		bestScore := 0.93
		deps := newTestDeps(t, &stubClient{
			getWorkspaceFn: resolveTestLoopWorkspace(t),
			listLoopRunsFn: func(context.Context, string, LoopRunListQuery) (contract.LoopRunsResponse, error) {
				return contract.LoopRunsResponse{Runs: []contract.LoopRunPayload{{
					ID:             "run-summary",
					WorkspaceID:    "ws-alpha",
					LoopName:       "release",
					Status:         contract.LoopRunStatusDone,
					Generation:     3,
					BestGeneration: &bestGeneration,
					BestScore:      &bestScore,
				}}}, nil
			},
		})
		stdout, _, err := executeRootCommand(
			t,
			deps,
			"loop", "runs", "--workspace", "alpha", "-o", "jsonl",
		)
		if err != nil {
			t.Fatalf("executeRootCommand(loop runs jsonl) error = %v", err)
		}
		lines := strings.Split(strings.TrimSpace(stdout), "\n")
		if len(lines) != 1 {
			t.Fatalf("loop runs JSONL lines = %d, want one summary; output=%q", len(lines), stdout)
		}
		var row map[string]any
		if err := json.Unmarshal([]byte(lines[0]), &row); err != nil {
			t.Fatalf("json.Unmarshal(loop runs JSONL) error = %v", err)
		}
		if row["best_generation"] != float64(bestGeneration) || row["best_score"] != bestScore {
			t.Fatalf("loop runs JSONL best = %#v, want generation %d score %.2f", row, bestGeneration, bestScore)
		}
		for _, forbidden := range []string{"generations", "runs", "aggregates"} {
			if _, exists := row[forbidden]; exists {
				t.Fatalf("loop runs JSONL row contains %q: %#v", forbidden, row)
			}
		}
	})

	t.Run("Should render Goal turn pages as HTTP JSON and one turn per JSONL line", func(t *testing.T) {
		t.Parallel()

		resultStatus := contract.GoalTurnResultCompleted
		endedAt := fixedTestNow.Add(time.Second)
		nextAfterSeq := int64(12)
		turn := contract.GoalTurn{
			Seq: 12, Generation: 2, NodeID: "goal", ItemIndex: 1, Turn: 3,
			PromptAttempt: 0, SessionID: "session-bound", BindingHandle: "goal:handle",
			BindingEpoch: 4, PromptID: "prompt-12", ResultStatus: &resultStatus,
			BlockingIssues: []contract.GoalBlockingIssue{}, ActorKind: "agent_session",
			ActorID: "session-bound", StartedAt: fixedTestNow, EndedAt: &endedAt,
		}
		var captured GoalTurnListQuery
		deps := newTestDeps(t, &stubClient{
			getWorkspaceFn: resolveTestLoopWorkspace(t),
			listGoalTurnsFn: func(
				_ context.Context,
				workspaceID string,
				runID string,
				query GoalTurnListQuery,
			) (contract.GoalTurnPage, error) {
				if workspaceID != "ws-alpha" || runID != "run-goal" {
					t.Fatalf("ListGoalTurns target = %s/%s", workspaceID, runID)
				}
				captured = query
				return contract.GoalTurnPage{Turns: []contract.GoalTurn{turn}, NextAfterSeq: &nextAfterSeq}, nil
			},
		})

		args := []string{
			"loop", "turns", "--workspace", "alpha", "--run", "run-goal",
			"--node", "goal", "--item", "1", "--after-seq", "8", "--limit", "4",
		}
		stdout, _, err := executeRootCommand(t, deps, append(args, "-o", "json")...)
		if err != nil {
			t.Fatalf("executeRootCommand(loop turns json) error = %v", err)
		}
		if captured.NodeID != "goal" || captured.ItemIndex == nil || *captured.ItemIndex != 1 ||
			captured.AfterSeq != 8 || captured.Limit != 4 {
			t.Fatalf("ListGoalTurns query = %#v", captured)
		}
		values := goalTurnValues(captured)
		if values.Get("node") != "goal" || values.Get("item") != "1" ||
			values.Get("after_seq") != "8" || values.Get("limit") != "4" {
			t.Fatalf("goalTurnValues() = %v", values)
		}
		var page contract.GoalTurnPage
		if err := json.Unmarshal([]byte(stdout), &page); err != nil {
			t.Fatalf("json.Unmarshal(loop turns page) error = %v", err)
		}
		if len(page.Turns) != 1 || page.Turns[0].PromptID != "prompt-12" ||
			page.NextAfterSeq == nil || *page.NextAfterSeq != 12 {
			t.Fatalf("loop turns page = %#v", page)
		}

		stdout, _, err = executeRootCommand(t, deps, append(args, "-o", "jsonl")...)
		if err != nil {
			t.Fatalf("executeRootCommand(loop turns jsonl) error = %v", err)
		}
		lines := strings.Split(strings.TrimSpace(stdout), "\n")
		if len(lines) != 1 {
			t.Fatalf("loop turns jsonl lines = %d, output=%q", len(lines), stdout)
		}
		var decodedTurn contract.GoalTurn
		if err := json.Unmarshal([]byte(lines[0]), &decodedTurn); err != nil {
			t.Fatalf("json.Unmarshal(loop turn jsonl) error = %v", err)
		}
		if decodedTurn.PromptID != "prompt-12" || decodedTurn.Seq != 12 {
			t.Fatalf("loop turn jsonl = %#v", decodedTurn)
		}
	})

	t.Run("Should reject an item filter without a node", func(t *testing.T) {
		t.Parallel()

		_, _, err := executeRootCommand(
			t,
			newTestDeps(t, &stubClient{getWorkspaceFn: resolveTestLoopWorkspace(t)}),
			"loop", "turns", "--workspace", "alpha", "--run", "run-goal", "--item", "1",
		)
		if err == nil || !strings.Contains(err.Error(), "--item requires --node") {
			t.Fatalf("executeRootCommand(loop turns item without node) error = %v", err)
		}
	})

	t.Run("Should expose Loop lifecycle verbs and paginated node inventory as structured output", func(t *testing.T) {
		t.Parallel()

		calls := make(map[string]int)
		mutation := contract.LoopMutationResponse{
			OK: true, RunID: "looprun-1", NodeID: "worker", Status: "running",
		}
		deps := newTestDeps(t, &stubClient{
			getWorkspaceFn: resolveTestLoopWorkspace(t),
			cancelLoopRunFn: func(
				_ context.Context, workspaceID string, runID string, _ agentidentity.Credentials,
			) (contract.LoopMutationResponse, error) {
				if workspaceID != "ws-alpha" || runID != "looprun-1" {
					t.Fatalf("CancelLoopRun target = %s/%s", workspaceID, runID)
				}
				calls[loopCancelKey]++
				response := mutation
				response.NodeID = ""
				return response, nil
			},
			killLoopRunFn: func(
				_ context.Context, workspaceID string, runID string, _ agentidentity.Credentials,
			) (contract.LoopMutationResponse, error) {
				if workspaceID != "ws-alpha" || runID != "looprun-1" {
					t.Fatalf("KillLoopRun target = %s/%s", workspaceID, runID)
				}
				calls[loopKillKey]++
				response := mutation
				response.NodeID = ""
				return response, nil
			},
			pauseLoopNodeFn: func(
				_ context.Context, workspaceID string, runID string, nodeID string,
				request contract.LoopNodePauseRequest, _ agentidentity.Credentials,
			) (contract.LoopMutationResponse, error) {
				if workspaceID != "ws-alpha" || runID != "looprun-1" || nodeID != "worker" ||
					request.Mode != "cancel" || request.Reason != "repair" {
					t.Fatalf("PauseLoopNode request = %s/%s/%s %#v", workspaceID, runID, nodeID, request)
				}
				calls[loopPauseKey]++
				return mutation, nil
			},
			resumeLoopNodeFn: func(
				_ context.Context, workspaceID string, runID string, nodeID string,
				request contract.LoopNodeResumeRequest, _ agentidentity.Credentials,
			) (contract.LoopMutationResponse, error) {
				if workspaceID != "ws-alpha" || runID != "looprun-1" || nodeID != "worker" ||
					request.Mode != "immediate" || request.ItemIndex == nil || *request.ItemIndex != 2 ||
					string(request.Payload) != `{"approved":true}` {
					t.Fatalf("ResumeLoopNode request = %s/%s/%s %#v", workspaceID, runID, nodeID, request)
				}
				calls[loopResumeKey]++
				return mutation, nil
			},
			cancelLoopNodeFn:  loopNodeMutationStubForTest(t, calls, loopCancelKey, mutation),
			killLoopNodeFn:    loopNodeMutationStubForTest(t, calls, loopKillKey, mutation),
			requeueLoopNodeFn: loopNodeMutationStubForTest(t, calls, loopRequeueKey, mutation),
			listLoopNodesFn: func(
				_ context.Context, workspaceID string, query LoopNodeListQuery,
			) (contract.LoopNodeInventoryResponse, error) {
				if workspaceID != "ws-alpha" || query.State != "waiting" || query.LoopName != "release" ||
					query.RunID != "looprun-1" || query.Cursor != "cursor-1" || query.Limit != 2 {
					t.Fatalf("ListLoopNodes query = %s %#v", workspaceID, query)
				}
				calls[loopNodesKey]++
				return contract.LoopNodeInventoryResponse{
					Items: []contract.LoopNodeInventoryItem{
						{
							State: contract.LoopNodeInventoryWaiting, LoopRunID: "looprun-1",
							LoopName: "release", Generation: 2, NodeID: "a", StateAt: fixedTestNow,
						},
						{
							State: contract.LoopNodeInventoryWaiting, LoopRunID: "looprun-1",
							LoopName: "release", Generation: 2, NodeID: "b", ItemIndex: 1,
							StateAt: fixedTestNow.Add(time.Second),
						},
					},
					NextCursor: "cursor-2",
				}, nil
			},
		})

		for _, args := range [][]string{
			{"loop", "cancel", "--workspace", "alpha", "--run-id", "looprun-1", "-o", "json"},
			{"loop", "kill", "--workspace", "alpha", "--run-id", "looprun-1", "-o", "json"},
			{"loop", "node", "pause", "--workspace", "alpha", "--run-id", "looprun-1", "--node", "worker", "--mode", "cancel", "--reason", "repair", "-o", "json"},
			{"loop", "node", "resume", "--workspace", "alpha", "--run-id", "looprun-1", "--node", "worker", "--mode", "immediate", "--item", "2", "--payload", `{"approved":true}`, "-o", "json"},
			{"loop", "node", "cancel", "--workspace", "alpha", "--run-id", "looprun-1", "--node", "worker", "--reason", "repair", "-o", "json"},
			{"loop", "node", "kill", "--workspace", "alpha", "--run-id", "looprun-1", "--node", "worker", "--reason", "repair", "-o", "json"},
			{"loop", "node", "requeue", "--workspace", "alpha", "--run-id", "looprun-1", "--node", "worker", "--reason", "repair", "-o", "json"},
		} {
			stdout, _, err := executeRootCommand(t, deps, args...)
			if err != nil {
				t.Fatalf("executeRootCommand(%v) error = %v", args, err)
			}
			var decoded contract.LoopMutationResponse
			err = json.Unmarshal([]byte(stdout), &decoded)
			if err != nil || !decoded.OK || decoded.RunID != "looprun-1" {
				t.Fatalf("lifecycle output for %v = %q, decode error = %v", args, stdout, err)
			}
		}

		inventoryArgs := []string{
			"loop", "nodes", "--workspace", "alpha", "--state", "waiting", "--loop", "release",
			"--run-id", "looprun-1", "--cursor", "cursor-1", "--limit", "2",
		}
		stdout, _, err := executeRootCommand(t, deps, append(inventoryArgs, "-o", "json")...)
		if err != nil {
			t.Fatalf("executeRootCommand(loop nodes json) error = %v", err)
		}
		var page contract.LoopNodeInventoryResponse
		err = json.Unmarshal([]byte(stdout), &page)
		if err != nil || len(page.Items) != 2 || page.NextCursor != "cursor-2" {
			t.Fatalf("loop nodes JSON = %q, page=%#v, error=%v", stdout, page, err)
		}
		stdout, _, err = executeRootCommand(t, deps, append(inventoryArgs, "-o", "jsonl")...)
		if err != nil {
			t.Fatalf("executeRootCommand(loop nodes jsonl) error = %v", err)
		}
		lines := strings.Split(strings.TrimSpace(stdout), "\n")
		if len(lines) != 2 {
			t.Fatalf("loop nodes JSONL lines = %d, output=%q", len(lines), stdout)
		}
		for index, line := range lines {
			var item contract.LoopNodeInventoryItem
			if err := json.Unmarshal([]byte(line), &item); err != nil || item.NodeID != string(rune('a'+index)) {
				t.Fatalf("loop nodes JSONL row %d = %q, item=%#v, error=%v", index, line, item, err)
			}
		}
		if calls[loopCancelKey] != 2 || calls[loopKillKey] != 2 || calls[loopPauseKey] != 1 ||
			calls[loopResumeKey] != 1 || calls[loopRequeueKey] != 1 || calls[loopNodesKey] != 2 {
			t.Fatalf("Loop lifecycle calls = %#v", calls)
		}
	})

	t.Run("Should reject the deleted Loop stop command", func(t *testing.T) {
		t.Parallel()

		_, _, err := executeRootCommand(t, newTestDeps(t, &stubClient{}), "loop", "stop")
		if err == nil || !strings.Contains(err.Error(), "unknown command") {
			t.Fatalf("executeRootCommand(loop stop) error = %v, want unknown command", err)
		}
	})

	t.Run("Should reject positional arguments", func(t *testing.T) {
		t.Parallel()

		_, _, err := executeRootCommand(
			t,
			newTestDeps(t, &stubClient{}),
			"loop", "run",
			"--workspace", "alpha",
			"--name", "release",
			"release",
		)
		if err == nil || (!strings.Contains(err.Error(), "accepts 0 arg(s)") &&
			!strings.Contains(err.Error(), "unknown command")) {
			t.Fatalf("loop run positional error = %v, want positional rejection", err)
		}
	})
}

func TestLoopRunInputsShouldNormalizeRuntimeAndRespectPromptMode(t *testing.T) {
	t.Parallel()

	t.Run("Should normalize only declared runtime expressions", func(t *testing.T) {
		t.Parallel()

		definition := testLoopDefinition("release", 1)
		definition.Inputs["runtime"] = dsl.Input{Type: dsl.InputTypeRuntime}
		values, err := normalizeLoopRunInputs(definition, map[string]any{
			"target": "openai/gpt-5@high", "runtime": "openai/gpt-5@high",
		})
		if err != nil {
			t.Fatalf("normalizeLoopRunInputs() error = %v", err)
		}
		if values["target"] != "openai/gpt-5@high" {
			t.Fatalf("target = %#v, want unchanged string", values["target"])
		}
		want := map[string]any{"provider": "openai", "model": "gpt-5", "reasoning": "high"}
		if !reflect.DeepEqual(values["runtime"], want) {
			t.Fatalf("runtime = %#v, want %#v", values["runtime"], want)
		}
	})

	t.Run("Should select a required enum in a human terminal", func(t *testing.T) {
		t.Parallel()

		definition := testLoopDefinition("release", 1)
		definition.Inputs["target"] = dsl.Input{
			Type: dsl.InputTypeString, Required: true, Enum: []string{"dev", "prod"},
		}
		cmd := &cobra.Command{Use: "loop-input-prompt-test"}
		cmd.PersistentFlags().String(outputFlagName, string(OutputHuman), "output")
		cmd.PersistentFlags().Bool(jsonFlagName, false, "json")
		cmd.SetIn(strings.NewReader("2\n"))
		cmd.SetOut(io.Discard)
		cmd.SetErr(io.Discard)
		values, err := promptForMissingLoopInputs(
			cmd,
			commandDeps{inputIsTerminal: func(io.Reader) bool { return true }},
			&stubClient{},
			"ws-alpha",
			definition,
			map[string]any{},
			false,
		)
		if err != nil {
			t.Fatalf("promptForMissingLoopInputs() error = %v", err)
		}
		if values["target"] != "prod" {
			t.Fatalf("target = %#v, want selected prod", values["target"])
		}
	})

	t.Run("Should leave missing values untouched when prompting is disabled", func(t *testing.T) {
		t.Parallel()

		definition := testLoopDefinition("release", 1)
		values, err := promptForMissingLoopInputs(
			&cobra.Command{Use: "loop-no-prompt-test"},
			commandDeps{inputIsTerminal: func(io.Reader) bool { return true }},
			&stubClient{},
			"ws-alpha",
			definition,
			map[string]any{},
			true,
		)
		if err != nil {
			t.Fatalf("promptForMissingLoopInputs() error = %v", err)
		}
		if len(values) != 0 {
			t.Fatalf("values = %#v, want missing values untouched", values)
		}
	})

	t.Run("Should treat configured defaults as satisfied without copying them into run inputs", func(t *testing.T) {
		t.Parallel()

		definition := testLoopDefinition("release", 1)
		definition.Inputs["target"] = dsl.Input{Type: dsl.InputTypeString, Required: true}
		calls := 0
		client := &stubClient{getLoopInputDefaultsFn: func(
			_ context.Context,
			workspaceID string,
			name string,
			scope contract.LoopInputDefaultsScope,
		) (contract.LoopInputDefaultsResponse, error) {
			calls++
			if workspaceID != "ws-alpha" || name != "release" {
				t.Fatalf("GetLoopInputDefaults target = %s/%s", workspaceID, name)
			}
			values := map[string]any{}
			if scope == contract.LoopInputDefaultsScopeGlobal {
				values["target"] = "configured"
			}
			return contract.LoopInputDefaultsResponse{LoopName: name, Scope: scope, Values: values}, nil
		}}
		cmd := &cobra.Command{Use: "loop-input-default-prompt-test"}
		cmd.PersistentFlags().String(outputFlagName, string(OutputHuman), "output")
		cmd.PersistentFlags().Bool(jsonFlagName, false, "json")
		cmd.SetIn(strings.NewReader(""))
		cmd.SetErr(io.Discard)
		values, err := promptForMissingLoopInputs(
			cmd,
			commandDeps{inputIsTerminal: func(io.Reader) bool { return true }},
			client,
			"ws-alpha",
			definition,
			map[string]any{},
			false,
		)
		if err != nil {
			t.Fatalf("promptForMissingLoopInputs() error = %v", err)
		}
		if calls != 2 || len(values) != 0 {
			t.Fatalf("configured default calls/values = %d/%#v, want 2 and daemon-resolved input", calls, values)
		}
	})

	t.Run("Should preserve prompted strings and sanitize authored terminal choices", func(t *testing.T) {
		t.Parallel()

		got, err := parsePromptedLoopInput(dsl.Input{Type: dsl.InputTypeString}, "null")
		if err != nil || got != "null" {
			t.Fatalf("parsePromptedLoopInput(string) = %#v, %v, want literal null", got, err)
		}
		var output bytes.Buffer
		selected, err := promptLoopInputValue(
			&output,
			bufio.NewReader(strings.NewReader("1\n")),
			"target\x1b",
			dsl.Input{Type: dsl.InputTypeString},
			[]string{"prod\x1b[31m\n"},
		)
		if err != nil {
			t.Fatalf("promptLoopInputValue() error = %v", err)
		}
		if selected != "prod\x1b[31m\n" {
			t.Fatalf("selected = %q, want exact authored value", selected)
		}
		if strings.ContainsAny(output.String(), "\x1b\r") {
			t.Fatalf("prompt output contains terminal controls: %q", output.String())
		}
	})
}

func loopNodeMutationStubForTest(
	t *testing.T,
	calls map[string]int,
	verb string,
	response contract.LoopMutationResponse,
) func(
	context.Context,
	string,
	string,
	string,
	contract.LoopNodeMutationRequest,
	agentidentity.Credentials,
) (contract.LoopMutationResponse, error) {
	t.Helper()
	return func(
		_ context.Context,
		workspaceID string,
		runID string,
		nodeID string,
		request contract.LoopNodeMutationRequest,
		_ agentidentity.Credentials,
	) (contract.LoopMutationResponse, error) {
		if workspaceID != "ws-alpha" || runID != "looprun-1" || nodeID != "worker" || request.Reason != "repair" {
			t.Fatalf("%s Loop node request = %s/%s/%s %#v", verb, workspaceID, runID, nodeID, request)
		}
		calls[verb]++
		return response, nil
	}
}

func TestLoopListShouldPreserveServerOwnedCatalogPages(t *testing.T) {
	t.Parallel()

	lastRunCreatedAt := time.Date(2026, 7, 10, 11, 0, 0, 0, time.UTC)
	response := contract.LoopsResponse{
		Loops: []contract.LoopCatalogEntryPayload{{
			Name:    "release",
			Source:  contract.LoopSourceWorkspace,
			Catalog: contract.LoopCatalogResourceSpec{Category: "delivery"},
			LastRun: &contract.LoopCatalogLastRunPayload{
				ID:        "run-release-latest",
				Status:    contract.LoopRunStatusRunning,
				CreatedAt: lastRunCreatedAt,
			},
			Aggregate30d:  contract.LoopCatalogAggregatePayload{Runs: 4, Succeeded: 2, Failed: 1},
			SuccessRate30: 0.5,
		}},
		Facets: contract.LoopCatalogFacetsPayload{
			Kinds:      map[string]int{"workspace": 7},
			Categories: map[string]int{"delivery": 5},
			Statuses:   map[string]int{"running": 3},
		},
		Page: contract.CountedCursorPagePayload{
			NextCursor: "cursor-next",
			HasMore:    true,
			Total:      7,
			Limit:      1,
		},
	}

	t.Run("Should forward filters and render the complete JSON envelope", func(t *testing.T) {
		t.Parallel()

		var captured LoopListQuery
		deps := newTestDeps(t, &stubClient{
			getWorkspaceFn: resolveTestLoopWorkspace(t),
			listLoopsFn: func(
				_ context.Context,
				workspaceID string,
				query LoopListQuery,
			) (contract.LoopsResponse, error) {
				if workspaceID != "ws-alpha" {
					t.Fatalf("ListLoops() workspace = %q, want ws-alpha", workspaceID)
				}
				captured = query
				return response, nil
			},
		})
		stdout, _, err := executeRootCommand(
			t,
			deps,
			"loop", "list", "--workspace", "alpha",
			"--query", "deploy", "--kind", "workspace", "--category", "delivery",
			"--status", "running", "--sort", "name", "--cursor", "cursor-prev", "--limit", "1",
			"-o", "json",
		)
		if err != nil {
			t.Fatalf("executeRootCommand(loop list) error = %v", err)
		}
		if captured.Search != "deploy" || captured.Kind != "workspace" || captured.Category != "delivery" ||
			captured.Status != "running" || captured.Sort != "name" ||
			captured.Cursor != "cursor-prev" || captured.Limit != 1 {
			t.Fatalf("ListLoops() query = %#v, want every server-owned filter", captured)
		}
		var decoded contract.LoopsResponse
		if err := json.Unmarshal([]byte(stdout), &decoded); err != nil {
			t.Fatalf("json.Unmarshal(loop list) error = %v", err)
		}
		if len(decoded.Loops) != 1 || decoded.Page.Total != 7 || decoded.Page.NextCursor != "cursor-next" ||
			decoded.Facets.Kinds["workspace"] != 7 {
			t.Fatalf("loop list JSON = %#v, want full page envelope", decoded)
		}
		lastRun := decoded.Loops[0].LastRun
		if lastRun == nil || lastRun.ID != "run-release-latest" ||
			lastRun.Status != contract.LoopRunStatusRunning || !lastRun.CreatedAt.Equal(lastRunCreatedAt) {
			t.Fatalf("loop list last_run = %#v, want lean run payload", lastRun)
		}
	})

	t.Run("Should append page and facets after JSONL items", func(t *testing.T) {
		t.Parallel()

		deps := newTestDeps(t, &stubClient{
			getWorkspaceFn: resolveTestLoopWorkspace(t),
			listLoopsFn: func(context.Context, string, LoopListQuery) (contract.LoopsResponse, error) {
				return response, nil
			},
		})
		stdout, _, err := executeRootCommand(
			t,
			deps,
			"loop", "list", "--workspace", "alpha", "-o", "jsonl",
		)
		if err != nil {
			t.Fatalf("executeRootCommand(loop list jsonl) error = %v", err)
		}
		lines := strings.Split(strings.TrimSpace(stdout), "\n")
		if len(lines) != 2 {
			t.Fatalf("loop list JSONL lines = %d, want item plus page; output=%q", len(lines), stdout)
		}
		var continuation struct {
			Type   string                            `json:"type"`
			Page   contract.CountedCursorPagePayload `json:"page"`
			Facets contract.LoopCatalogFacetsPayload `json:"facets"`
		}
		if err := json.Unmarshal([]byte(lines[1]), &continuation); err != nil {
			t.Fatalf("json.Unmarshal(loop page continuation) error = %v", err)
		}
		if continuation.Type != "page" || continuation.Page.NextCursor != "cursor-next" ||
			continuation.Facets.Statuses["running"] != 3 {
			t.Fatalf("loop page continuation = %#v", continuation)
		}
	})

	for _, output := range []string{"human", "toon"} {
		t.Run("Should expose continuation metadata in "+output, func(t *testing.T) {
			t.Parallel()

			deps := newTestDeps(t, &stubClient{
				getWorkspaceFn: resolveTestLoopWorkspace(t),
				listLoopsFn: func(context.Context, string, LoopListQuery) (contract.LoopsResponse, error) {
					return response, nil
				},
			})
			args := []string{"loop", "list", "--workspace", "alpha"}
			if output == "toon" {
				args = append(args, "-o", "toon")
			}
			stdout, _, err := executeRootCommand(t, deps, args...)
			if err != nil {
				t.Fatalf("executeRootCommand(loop list %s) error = %v", output, err)
			}
			if !strings.Contains(stdout, "cursor-next") || !strings.Contains(strings.ToLower(stdout), "total") {
				t.Fatalf("loop list %s output = %q, want continuation metadata", output, stdout)
			}
		})
	}
}

func testAgentIdentityEnv(sessionID string, agentName string) func(string) string {
	return func(key string) string {
		switch key {
		case agentidentity.EnvSessionID:
			return sessionID
		case agentidentity.EnvAgent:
			return agentName
		default:
			return ""
		}
	}
}

func resolveTestLoopWorkspace(t *testing.T) func(context.Context, string) (WorkspaceDetailRecord, error) {
	t.Helper()

	return func(_ context.Context, ref string) (WorkspaceDetailRecord, error) {
		if ref != "alpha" {
			t.Fatalf("GetWorkspace ref = %q, want alpha", ref)
		}
		return WorkspaceDetailRecord{Workspace: WorkspaceRecord{ID: "ws-alpha"}}, nil
	}
}

func testLoopDefinition(name string, version int) dsl.Definition {
	return dsl.Definition{
		APIVersion: dsl.APIVersion,
		Kind:       dsl.KindLoop,
		Meta: dsl.Meta{
			Name:        name,
			Version:     version,
			Description: "Release loop",
			Catalog: dsl.CatalogMeta{
				UseWhen:  "release coordination is needed",
				Keywords: []string{"release", "qa"},
				Category: "delivery",
			},
		},
		Inputs: map[string]dsl.Input{
			"target": {Type: dsl.InputTypeString, Required: true},
		},
		Contract: dsl.Contract{
			Goal:             "Ship a release",
			DefinitionOfDone: "Release is verified",
			IterationCap:     2,
			Budget:           dsl.Budget{OnExceeded: dsl.BudgetExceededHalt},
		},
		Graph: dsl.Graph{
			Nodes: []dsl.Node{{
				ID:       "target",
				Class:    dsl.NodeClassSource,
				Kind:     string(dsl.SourceInput),
				InputRef: "target",
			}},
		},
		DefinitionExtensionState: &dsl.DefinitionExtensionState{
			Start: []dsl.StartBinding{{Kind: dsl.StartCLI}},
		},
	}
}

func loopRunDefinitionResponse(t testing.TB) func(
	context.Context,
	string,
	string,
) (contract.LoopResponse, error) {
	t.Helper()
	return func(_ context.Context, _ string, name string) (contract.LoopResponse, error) {
		return testLoopResponse(t, testLoopDefinition(name, 1)), nil
	}
}

func testLoopResponse(t testing.TB, definition dsl.Definition) contract.LoopResponse {
	t.Helper()

	document, err := contract.NewLoopDefinitionDocument(definition)
	if err != nil {
		t.Fatalf("NewLoopDefinitionDocument() error = %v", err)
	}
	return contract.LoopResponse{
		Loop: contract.LoopDefinitionPayload{
			Name:        definition.Meta.Name,
			Version:     definition.Meta.Version,
			Description: definition.Meta.Description,
			Source:      contract.LoopSourceUser,
			Definition:  document,
		},
	}
}

func writeTestLoopDefinition(t *testing.T, definition dsl.Definition) string {
	t.Helper()

	body := []byte(`apiVersion: compozy.loop/v1
kind: Loop
meta:
  name: ` + definition.Meta.Name + `
  version: ` + strconv.Itoa(definition.Meta.Version) + `
  description: Release loop
  catalog:
    use_when: release coordination is needed
    keywords: [release, qa]
    category: delivery
inputs:
  target:
    type: string
    required: true
contract:
  goal: Ship a release
  definition_of_done: Release is verified
  iteration_cap: 2
  no_progress:
    window: 0
  budget:
    tokens: 0
    wall_clock_sec: 0
    on_exceeded: halt
graph:
  nodes:
    - id: target
      class: source
      kind: input
      input_ref: target
  edges: []
start:
  - kind: cli
`)
	path := filepath.Join(t.TempDir(), definition.Meta.Name+".yaml")
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatalf("os.WriteFile(%s) error = %v", path, err)
	}
	return path
}
