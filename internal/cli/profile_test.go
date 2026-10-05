package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/compozy/compozy/internal/agentidentity"
	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/store"
	"github.com/spf13/cobra"
)

func TestProfileReadScopeQueryValues(t *testing.T) {
	t.Parallel()

	t.Run("Should transport only the explicit aggregate profile selection", func(t *testing.T) {
		t.Parallel()
		command := &cobra.Command{Use: "list"}
		recordProfileReadSelection(command, profileReadSelection{AllProfiles: true})
		original := url.Values{"limit": []string{"20"}}
		values := profileQueryValues(command.Context(), original)
		if values.Get("all_profiles") != "true" || values.Get("profile") != "" {
			t.Fatalf("profile query = %v, want aggregate only", values)
		}
		if original.Has("all_profiles") {
			t.Fatalf("profileQueryValues mutated caller query: %v", original)
		}
	})

	t.Run("Should transport the resolved scoped profile", func(t *testing.T) {
		t.Parallel()
		command := &cobra.Command{Use: "list"}
		recordProfileReadSelection(command, profileReadSelection{Profile: "marketing"})
		values := profileQueryValues(command.Context(), nil)
		if values.Get("profile") != "marketing" || values.Get("all_profiles") != "" {
			t.Fatalf("profile query = %v, want marketing only", values)
		}
	})
}

func TestAgentProfileSelectionUsesTheDaemonSessionOwner(t *testing.T) {
	t.Run("Should use the daemon session owner Profile", func(t *testing.T) {
		t.Parallel()

		root := &cobra.Command{Use: "compozy"}
		sessionCommand := &cobra.Command{Use: "session"}
		command := &cobra.Command{Use: "prompt"}
		root.AddCommand(sessionCommand)
		sessionCommand.AddCommand(command)
		command.SetContext(context.Background())
		command.Flags().String(profileFlagName, "", "")
		client := &profileTestDaemonClient{
			DaemonClient: &stubClient{getSessionFn: func(ctx context.Context, id string) (SessionRecord, error) {
				if id != "sess-engineering" {
					t.Fatalf("GetSession() id = %q, want sess-engineering", id)
				}
				if got := profileQueryValues(ctx, nil).Get("all_profiles"); got != "true" {
					t.Fatalf("GetSession() all_profiles = %q, want true", got)
				}
				return SessionRecord{
					ID: "sess-engineering", ProfileID: "profile-engineering", AgentName: "orchestrator",
					WorkspaceID: "ws-1", State: session.StateActive,
				}, nil
			}},
			profileClientAPI: &profileClientStub{profiles: []contract.Profile{
				{ID: "00000000000000000000000000", Name: "default", State: "active"},
				{ID: "profile-engineering", Name: "engineering", State: "active"},
			}},
		}
		deps := newTestDeps(t, client)
		deps.getenv = func(key string) string {
			switch key {
			case agentidentity.EnvSessionID:
				return "sess-engineering"
			case agentidentity.EnvAgent:
				return "orchestrator"
			default:
				return ""
			}
		}

		handled, err := prepareAgentProfileSelection(command, deps, client, client)
		if err != nil {
			t.Fatalf("prepareAgentProfileSelection() error = %v", err)
		}
		if !handled {
			t.Fatal("prepareAgentProfileSelection() handled = false")
		}
		if got := profileQueryValues(command.Context(), nil).Get(profileFlagName); got != "engineering" {
			t.Fatalf("profile query = %q, want engineering", got)
		}
	})
}

// Invariant: every executable session command inherits the calling agent's session profile.
// The canonical profile scope suite owns command-tree profile selection.
func TestSessionCommandUsesTheDaemonSessionOwnerProfile(t *testing.T) {
	t.Parallel()

	t.Run("Should inherit the caller Profile for session status", func(t *testing.T) {
		t.Parallel()

		client := &profileTestDaemonClient{
			DaemonClient: &stubClient{
				getSessionFn: func(ctx context.Context, id string) (SessionRecord, error) {
					if id != "sess-engineering" {
						t.Fatalf("GetSession() id = %q, want sess-engineering", id)
					}
					if got := profileQueryValues(ctx, nil).Get("all_profiles"); got != "true" {
						t.Fatalf("GetSession() all_profiles = %q, want true", got)
					}
					return SessionRecord{
						ID: "sess-engineering", ProfileID: "profile-engineering", AgentName: "orchestrator",
						WorkspaceID: "ws-1", State: session.StateActive,
					}, nil
				},
				getSessionStatusFn: func(ctx context.Context, id string) (SessionStatusRecord, error) {
					if got := profileQueryValues(ctx, nil).Get(profileFlagName); got != "engineering" {
						t.Fatalf("GetSessionStatus() profile = %q, want engineering", got)
					}
					return SessionStatusRecord{SessionID: id, State: contract.SessionHealthStatePrompting}, nil
				},
			},
			profileClientAPI: &profileClientStub{profiles: []contract.Profile{
				{ID: store.DefaultProfileID, Name: configDefaultKey, State: "active"},
				{ID: "profile-engineering", Name: "engineering", State: "active"},
			}},
		}
		deps := newTestDeps(t, client)
		deps.getenv = func(key string) string {
			switch key {
			case agentidentity.EnvSessionID:
				return "sess-engineering"
			case agentidentity.EnvAgent:
				return "orchestrator"
			default:
				return ""
			}
		}

		if _, _, err := executeRootCommand(t, deps, "session", "status", "sess-engineering", "-o", "json"); err != nil {
			t.Fatalf("session status error = %v", err)
		}
	})
}

func TestTaskExecutionCommandProfileSelection(t *testing.T) {
	t.Parallel()
	for _, args := range [][]string{
		{"publish"}, {"start"}, {"approve"}, {"reject"},
		{"pause", "--reason", "Operator review"}, {"resume"}, {"cancel"},
		{"block", "--kind", "needs_input", "--reason", "Operator review"},
		{"unblock", "--block", "block-review"}, {"blocks"}, {"recover"},
	} {
		action := args[0]
		t.Run("Should keep the selected profile when executing task "+action, func(t *testing.T) {
			t.Parallel()
			called := false
			checkSelection := func(ctx context.Context, id string) {
				t.Helper()
				called = true
				if id != "task-marketing" {
					t.Fatalf("task ID = %q, want task-marketing", id)
				}
				if got := profileQueryValues(ctx, nil).Get(profileFlagName); got != "marketing" {
					t.Fatalf("task %s profile = %q, want marketing", action, got)
				}
			}
			execute := func(ctx context.Context, id string, _ TaskExecutionRequest) (TaskExecutionRecord, error) {
				checkSelection(ctx, id)
				return sampleTaskExecutionRecord(), nil
			}
			client := &profileTestDaemonClient{
				DaemonClient: withWorkspaceResolution(&stubClient{
					publishTaskFn: execute,
					startTaskFn:   execute,
					approveTaskFn: execute,
					rejectTaskFn: func(ctx context.Context, id string) (TaskRecord, error) {
						checkSelection(ctx, id)
						return TaskRecord{ID: id}, nil
					},
					pauseTaskFn: func(ctx context.Context, id string, _ PauseTaskRequest) (TaskRecord, error) {
						checkSelection(ctx, id)
						return TaskRecord{ID: id}, nil
					},
					resumeTaskFn: func(ctx context.Context, id string, _ ResumeTaskRequest) (TaskRecord, error) {
						checkSelection(ctx, id)
						return TaskRecord{ID: id}, nil
					},
					cancelTaskFn: func(ctx context.Context, id string, _ CancelTaskRequest) (TaskRecord, error) {
						checkSelection(ctx, id)
						return TaskRecord{ID: id}, nil
					},
					blockTaskFn: func(ctx context.Context, id string, _ CreateTaskBlockRequest) (TaskBlockRecord, error) {
						checkSelection(ctx, id)
						return TaskBlockRecord{TaskID: id}, nil
					},
					clearTaskBlockFn: func(ctx context.Context, id, _ string, _ ClearTaskBlockRequest) (TaskBlockRecord, error) {
						checkSelection(ctx, id)
						return TaskBlockRecord{TaskID: id}, nil
					},
					listTaskBlocksFn: func(ctx context.Context, id string, _ bool) ([]TaskBlockRecord, error) {
						checkSelection(ctx, id)
						return []TaskBlockRecord{}, nil
					},
					recoverTaskFn: func(ctx context.Context, id string, _ RecoverTaskRequest) (TaskRecord, error) {
						checkSelection(ctx, id)
						return TaskRecord{ID: id}, nil
					},
				}),
				profileClientAPI: &profileClientStub{profiles: []contract.Profile{
					{ID: store.DefaultProfileID, Name: configDefaultKey, State: "active"},
					{ID: "profile-marketing", Name: "marketing", State: "active"},
				}},
			}
			commandArgs := append(
				[]string{"task", action, "task-marketing", "--profile", "marketing", "-o", "json"},
				args[1:]...)
			_, _, err := executeRootCommand(t, newTestDeps(t, client), commandArgs...)
			if err != nil {
				t.Fatalf("task %s error = %v", action, err)
			}
			if !called {
				t.Fatal("task action did not reach the client")
			}
		})
	}
}

func TestTaskOperatorCommandProfileSelection(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name   string
		args   []string
		method string
		path   string
	}{
		{"Should scope task updates", []string{"update", "task-marketing", "--title", "Editorial plan"}, http.MethodPatch, "/api/tasks/task-marketing"},
		{"Should scope task deletion", []string{"delete", "task-marketing"}, http.MethodDelete, "/api/tasks/task-marketing"},
		{"Should scope dependency creation", []string{"dependency", "add", "task-marketing", "--depends-on", "task-notes"}, http.MethodPost, "/api/tasks/task-marketing/dependencies"},
		{"Should scope dependency removal", []string{"dependency", "remove", "task-marketing", "task-notes"}, http.MethodDelete, "/api/tasks/task-marketing/dependencies/task-notes"},
		{"Should scope run enqueue", []string{"run", "enqueue", "task-marketing"}, http.MethodPost, "/api/tasks/task-marketing/runs"},
		{"Should scope run start", []string{"run", "start", "run-marketing"}, http.MethodPost, "/api/task-runs/run-marketing/start"},
		{"Should scope session attachment", []string{"run", "attach-session", "run-marketing", "--session", "sess-editor"}, http.MethodPost, "/api/task-runs/run-marketing/attach-session"},
		{"Should scope run completion", []string{"run", "complete", "run-marketing"}, http.MethodPost, "/api/task-runs/run-marketing/complete"},
		{"Should scope run failure", []string{"run", "fail", "run-marketing", "--error", "Publication deferred"}, http.MethodPost, "/api/task-runs/run-marketing/fail"},
		{"Should scope run cancellation", []string{"run", "cancel", "run-marketing"}, http.MethodPost, "/api/task-runs/run-marketing/cancel"},
		{"Should scope run recovery", []string{"run", "recover", "run-marketing"}, http.MethodPost, "/api/runs/run-marketing/recover"},
		{"Should scope run fan out", []string{"fan-out", "task-marketing", "--designation", "Review copy", "--idempotency-key", "copy-review"}, http.MethodPost, "/api/tasks/task-marketing/runs/fan-out"},
		{"Should scope forced release", []string{"release", "run-marketing"}, http.MethodPost, "/api/runs/run-marketing/release"},
		{"Should scope bulk release", []string{"release", "run-marketing", "run-notes"}, http.MethodPost, "/api/runs/bulk/release"},
		{"Should scope forced failure", []string{"fail", "run-marketing", "--reason", "Publication deferred"}, http.MethodPost, "/api/runs/run-marketing/fail"},
		{"Should scope bulk failure", []string{"fail", "run-marketing", "run-notes", "--reason", "Publication deferred"}, http.MethodPost, "/api/runs/bulk/fail"},
		{"Should scope run retry", []string{"retry", "run-marketing"}, http.MethodPost, "/api/runs/run-marketing/retry"},
		{"Should scope review requests", []string{"review", "request", "run-marketing"}, http.MethodPost, "/api/task-runs/run-marketing/reviews"},
		{"Should scope review verdicts", []string{"review", "submit", "review-marketing", "--run", "run-marketing", "--outcome", "approved", "--confidence", "0.9", "--reason", "Ready for publication", "--delivery-id", "editorial-review"}, http.MethodPost, "/api/task-reviews/review-marketing/verdict"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			reached := errors.New("task action reached transport")
			calls := 0
			transport := &daemonClient{
				target: LocalClientTarget("/tmp/compozy.sock"),
				httpClient: &http.Client{
					Transport: roundTripperFunc(func(request *http.Request) (*http.Response, error) {
						if request.Method == http.MethodGet && request.URL.Path == "/api/workspaces" {
							return newHTTPResponse(http.StatusOK, `{"workspaces":[]}`), nil
						}
						if strings.HasPrefix(request.URL.Path, "/api/workspaces/") {
							return newHTTPResponse(http.StatusNotFound, `{"error":"workspace not registered"}`), nil
						}
						calls++
						if request.Method != test.method || request.URL.Path != test.path {
							t.Fatalf(
								"request = %s %s, want %s %s",
								request.Method,
								request.URL.Path,
								test.method,
								test.path,
							)
						}
						if got := request.URL.Query().Get(profileFlagName); got != "marketing" {
							t.Errorf("request profile = %q, want marketing", got)
						}
						return nil, reached
					}),
				},
			}
			client := &profileTestDaemonClient{
				DaemonClient: transport,
				profileClientAPI: &profileClientStub{profiles: []contract.Profile{
					{ID: store.DefaultProfileID, Name: configDefaultKey, State: "active"},
					{ID: "profile-marketing", Name: "marketing", State: "active"},
				}},
			}
			args := append([]string{"task"}, test.args...)
			args = append(args, "--profile", "marketing", "-o", "json")
			deps := newTestDeps(t, client)
			workspace := t.TempDir()
			deps.getwd = func() (string, error) { return workspace, nil }
			_, _, err := executeRootCommand(t, deps, args...)
			if !errors.Is(err, reached) || calls != 1 {
				t.Fatalf("action error = %v, calls = %d, want one transport call", err, calls)
			}
		})
	}
}

// Invariant: a remote gateway defers implicit profile selection to the remote
// daemon, while explicit operator selection is transported without requiring
// the gateway to expose profile-management routes. The canonical profile scope
// suite owns this transport boundary.
func TestRemoteGatewayProfileSelection(t *testing.T) {
	t.Parallel()

	newCommand := func(t *testing.T) (*cobra.Command, commandDeps, *daemonClient) {
		t.Helper()
		command := &cobra.Command{Use: "prompt"}
		command.Flags().String(profileFlagName, "", "")
		client := &daemonClient{target: ClientTarget{kind: clientTargetGateway, name: "remote"}}
		deps := newTestDeps(t, client)
		deps.getenv = func(string) string { return "" }
		return command, deps, client
	}

	t.Run("Should defer implicit profile selection to the remote daemon", func(t *testing.T) {
		t.Parallel()
		command, deps, client := newCommand(t)

		handled, err := prepareRemoteGatewayProfileSelection(command, deps, client)
		if err != nil {
			t.Fatalf("prepareRemoteGatewayProfileSelection() error = %v", err)
		}
		if !handled {
			t.Fatal("prepareRemoteGatewayProfileSelection() handled = false")
		}
		if values := profileQueryValues(command.Context(), nil); values.Has(profileFlagName) {
			t.Fatalf("profile query = %v, want remote daemon default", values)
		}
	})

	t.Run("Should transport an explicit profile without a catalog read", func(t *testing.T) {
		t.Parallel()
		command, deps, client := newCommand(t)
		if err := command.Flags().Set(profileFlagName, "research"); err != nil {
			t.Fatalf("set --profile error = %v", err)
		}

		handled, err := prepareRemoteGatewayProfileSelection(command, deps, client)
		if err != nil {
			t.Fatalf("prepareRemoteGatewayProfileSelection() error = %v", err)
		}
		if !handled {
			t.Fatal("prepareRemoteGatewayProfileSelection() handled = false")
		}
		if got := profileQueryValues(command.Context(), nil).Get(profileFlagName); got != "research" {
			t.Fatalf("profile query = %q, want research", got)
		}
	})
}

func TestProfileCommandOutputContract(t *testing.T) {
	t.Parallel()

	t.Run("Should report an archived remembered selection as a default fallback", func(t *testing.T) {
		t.Parallel()
		deps := profileTestDeps(t, contract.ProfileSelection{
			Scope: contract.ProfileSelectionScopeWorkspace, WorkspaceID: "ws-1",
			Profile: "default", Note: "archived_remembered_fallback",
		})

		output, _, err := executeRootCommand(t, deps, "profile", "current", "-o", "json")
		if err != nil {
			t.Fatalf("profile current error = %v", err)
		}
		var current profileCurrentRecord
		if err := json.Unmarshal([]byte(output), &current); err != nil {
			t.Fatalf("json.Unmarshal(profile current) error = %v", err)
		}
		want := profileCurrentRecord{
			Profile: "default", Source: "default", Workspace: "my-saas", Note: "archived_remembered_fallback",
		}
		if current != want {
			t.Fatalf("profile current = %#v, want %#v", current, want)
		}
	})

	t.Run("Should render list and current JSON exactly [UT-076]", func(t *testing.T) {
		t.Parallel()
		deps := profileTestDeps(t)

		listOutput, _, err := executeRootCommand(t, deps, "profile", "list", "--profile", "marketing", "-o", "json")
		if err != nil {
			t.Fatalf("profile list error = %v", err)
		}
		var list []map[string]any
		if err := json.Unmarshal([]byte(listOutput), &list); err != nil {
			t.Fatalf("json.Unmarshal(profile list) error = %v", err)
		}
		if len(list) != 2 || list[1]["name"] != "marketing" || list[1]["current"] != true {
			t.Fatalf("profile list = %#v, want marketing current", list)
		}
		for _, forbidden := range []string{"id", "created_at", "archived_at", "resolution_source"} {
			if _, exists := list[0][forbidden]; exists {
				t.Fatalf("profile list contains non-contract field %q: %#v", forbidden, list[0])
			}
		}

		currentOutput, _, err := executeRootCommand(
			t,
			deps,
			"profile",
			"current",
			"--profile",
			"marketing",
			"-o",
			"json",
		)
		if err != nil {
			t.Fatalf("profile current error = %v", err)
		}
		var current profileCurrentRecord
		if err := json.Unmarshal([]byte(currentOutput), &current); err != nil {
			t.Fatalf("json.Unmarshal(profile current) error = %v", err)
		}
		if current != (profileCurrentRecord{Profile: "marketing", Source: "flag", Workspace: "my-saas"}) {
			t.Fatalf("profile current = %#v", current)
		}
	})

	t.Run("Should record root profile resolution on command context [UT-022]", func(t *testing.T) {
		t.Parallel()
		root := newRootCommand(profileTestDeps(t))
		root.SetArgs([]string{"profile", "current", "--profile", "marketing", "-o", "json"})
		root.SetOut(&bytes.Buffer{})
		root.SetErr(&bytes.Buffer{})
		if err := root.Execute(); err != nil {
			t.Fatalf("Execute() error = %v", err)
		}
		command, _, err := root.Find([]string{"profile", "current"})
		if err != nil {
			t.Fatalf("Find(profile current) error = %v", err)
		}
		resolution, ok := commandProfileResolution(command)
		if !ok || resolution.Source != profileResolutionFlag || resolution.Profile.Name != "marketing" {
			t.Fatalf("profile resolution = %#v, %v", resolution, ok)
		}
	})

	t.Run("Should emit a profile resolution frame for an empty JSONL list [UT-078]", func(t *testing.T) {
		t.Parallel()
		cmd := &cobra.Command{}
		cmd.Flags().String(outputFlagName, string(OutputJSONL), "")
		var output bytes.Buffer
		cmd.SetOut(&output)
		recordProfileResolution(cmd, profileResolution{
			Profile: contract.Profile{Name: "marketing"}, Source: profileResolutionRemembered, WorkspaceName: "my-saas",
		})
		if err := writeCommandOutput(cmd, profileListBundle(nil, "marketing")); err != nil {
			t.Fatalf("writeCommandOutput() error = %v", err)
		}
		if got := strings.TrimSpace(
			output.String(),
		); got != `{"kind":"profile_resolution","profile":"marketing","source":"remembered","workspace":"my-saas"}` {
			t.Fatalf("JSONL frame = %s", got)
		}
	})

	t.Run("Should resolve the profile at the shared workspace boundary [UT-022]", func(t *testing.T) {
		t.Parallel()
		deps := profileTestDeps(t)
		client, err := clientFromDeps(deps)
		if err != nil {
			t.Fatalf("clientFromDeps() error = %v", err)
		}
		root := &cobra.Command{Use: "compozy"}
		session := &cobra.Command{Use: "session"}
		list := &cobra.Command{Use: "list"}
		list.Flags().String(profileFlagName, "", "")
		session.AddCommand(list)
		root.AddCommand(session)
		if err := list.Flags().Set(profileFlagName, "marketing"); err != nil {
			t.Fatalf("set --profile error = %v", err)
		}

		if _, err := resolveCommandWorkspace(
			context.Background(), list, deps, client, workspaceResolutionRequest{},
		); err != nil {
			t.Fatalf("resolveCommandWorkspace() error = %v", err)
		}
		resolution, ok := commandProfileResolution(list)
		if !ok || resolution.Profile.Name != "marketing" || resolution.Source != profileResolutionFlag {
			t.Fatalf("profile resolution = %#v, %v", resolution, ok)
		}
	})

	t.Run("Should keep machine commands immune to profile selection [E2E-012]", func(t *testing.T) {
		t.Parallel()
		deps := profileTestDeps(t)
		deps.getenv = func(name string) string {
			if name == profileEnvName {
				return "missing"
			}
			return ""
		}
		client, err := clientFromDeps(deps)
		if err != nil {
			t.Fatalf("clientFromDeps() error = %v", err)
		}
		root := &cobra.Command{Use: "compozy"}
		doctor := &cobra.Command{Use: "doctor"}
		root.AddCommand(doctor)

		if _, err := resolveCommandWorkspace(
			context.Background(), doctor, deps, client, workspaceResolutionRequest{},
		); err != nil {
			t.Fatalf("resolveCommandWorkspace() error = %v", err)
		}
		if resolution, ok := commandProfileResolution(doctor); ok {
			t.Fatalf("machine command profile resolution = %#v", resolution)
		}
	})
}

func TestProfileStructuredErrorsCoverPublicCodes(t *testing.T) {
	t.Parallel()
	t.Run("Should reject explicit profile selection on machine-wide commands", func(t *testing.T) {
		t.Parallel()
		for _, args := range [][]string{
			{"daemon", "start", "--profile", "marketing"},
			{"doctor", "--profile", "marketing"},
			{"update", "--profile", "marketing"},
		} {
			_, _, err := executeRootCommand(t, newTestDeps(t, nil), args...)
			var profileErr *profileCommandError
			if err == nil || !errors.As(err, &profileErr) ||
				profileErr.payload.Error.Code != profileSelectionUnsupportedCode {
				t.Fatalf("command %v error = %v, want %s", args, err, profileSelectionUnsupportedCode)
			}
		}
	})

	codes := []string{
		"profile_not_found", "profile_archived", "profile_name_invalid", "profile_name_taken",
		"profile_name_reserved", "profile_permanent", "profile_owns_work", "profile_sessions_running",
		"profile_config_key_denied", "profile_secret_env_forbidden", "profile_selection_conflict",
		profileSelectionUnsupportedCode,
		"profile_plan_stale", "profile_unavailable", "profile_session_conflict",
		"profile_remote_management_forbidden", "profile_deliveries_in_flight", "profile_approvals_pending",
	}
	for _, code := range codes {
		t.Run("Should marshal "+code+" [UT-079]", func(t *testing.T) {
			t.Parallel()
			err := newProfileSelectionError(code, "message", "action")
			payload, ok := marshalStructuredExecutionError([]string{"profile", "-o", "json"}, err)
			if !ok {
				t.Fatal("marshalStructuredExecutionError() ok = false")
			}
			var decoded contract.ProfileErrorPayload
			if err := json.Unmarshal(payload, &decoded); err != nil {
				t.Fatalf("json.Unmarshal() error = %v", err)
			}
			if decoded.Error.Code != code || decoded.Error.Message != "message" || decoded.Error.Action != "action" {
				t.Fatalf("payload = %#v", decoded)
			}
		})
	}
}

func profileTestDeps(t *testing.T, selections ...contract.ProfileSelection) commandDeps {
	t.Helper()
	workspaceClient := &stubClient{getWorkspaceFn: func(context.Context, string) (WorkspaceDetailRecord, error) {
		return WorkspaceDetailRecord{
			Workspace: WorkspaceRecord{ID: "ws-1", Name: "my-saas", RootDir: "/workspace"},
		}, nil
	}}
	profiles := &profileClientStub{
		selections: selections,
		profiles: []contract.Profile{
			{Name: "default", Color: "#8E8EB5", Icon: new("circle"), State: "active", WorkItems: 12},
			{Name: "marketing", Color: "#FF7F3A", Icon: new("megaphone"), State: "active", WorkItems: 3},
		},
	}
	client := &profileTestDaemonClient{DaemonClient: workspaceClient, profileClientAPI: profiles}
	deps := newTestDeps(t, client)
	deps.getwd = func() (string, error) { return "/workspace", nil }
	deps.getenv = func(string) string { return "" }
	return deps
}
