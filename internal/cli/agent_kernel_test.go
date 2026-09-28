package cli

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/agentidentity"
	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/session"
	speedpkg "github.com/compozy/compozy/internal/speed"
	"github.com/compozy/compozy/internal/store"
)

func TestMeCommandJSONReturnsValidatedIdentity(t *testing.T) {
	t.Parallel()

	t.Run("Should return validated identity as JSON", func(t *testing.T) {
		t.Parallel()

		client := &stubClient{}
		deps := newAgentCommandTestDeps(t, client)
		client.agentMeFn = func(_ context.Context, credentials agentidentity.Credentials) (AgentMeRecord, error) {
			assertAgentCredentials(t, credentials)
			return AgentMeRecord{
				Self: contract.AgentIdentityPayload{
					SessionID: "sess-agent",
					AgentName: "coder",
					Provider:  "test-provider",
					Model:     "test-model",
				},
				Workspace: contract.AgentWorkspacePayload{
					ID:      "ws-1",
					RootDir: "/workspace/project",
				},
				Session: contract.AgentSessionPayload{
					ID:    "sess-agent",
					State: session.StateActive,

					CreatedAt: fixedTestNow,
					UpdatedAt: fixedTestNow,
				},
			}, nil
		}

		stdout, _, err := executeRootCommand(t, deps, "me", "-o", "json")
		if err != nil {
			t.Fatalf("compozy me error = %v", err)
		}

		var got AgentMeRecord
		if err := json.Unmarshal([]byte(stdout), &got); err != nil {
			t.Fatalf("json.Unmarshal(compozy me) error = %v", err)
		}
		if got.Self.SessionID != "sess-agent" || got.Self.AgentName != "coder" || got.Workspace.ID != "ws-1" {
			t.Fatalf("compozy me payload = %#v, want caller session/workspace identity", got)
		}
	})
}

func TestNotifyCommandReturnsDaemonProvableOutcome(t *testing.T) {
	t.Parallel()

	t.Run("Should forward the bounded request under agent identity", func(t *testing.T) {
		t.Parallel()

		client := &stubClient{}
		deps := newAgentCommandTestDeps(t, client)
		client.agentNotifyFn = func(
			_ context.Context,
			request AgentNotifyRequest,
			credentials agentidentity.Credentials,
		) (AgentNotifyRecord, error) {
			assertAgentCredentials(t, credentials)
			if request.Title != "Deps audit done" || request.Body != "3 findings, 1 high severity" {
				t.Fatalf("AgentNotify() request = %#v, want bounded title and body", request)
			}
			return AgentNotifyRecord{Outcome: "delivered"}, nil
		}

		stdout, _, err := executeRootCommand(
			t,
			deps,
			"notify",
			"Deps audit done",
			"--body",
			"3 findings, 1 high severity",
		)
		if err != nil {
			t.Fatalf("compozy notify error = %v", err)
		}
		if got, want := strings.TrimSpace(stdout), "OUTCOME   delivered"; got != want {
			t.Fatalf("compozy notify output = %q, want %q", got, want)
		}
	})

	t.Run("Should preserve rate-limit details in structured output", func(t *testing.T) {
		t.Parallel()

		client := &stubClient{}
		deps := newAgentCommandTestDeps(t, client)
		client.agentNotifyFn = func(
			_ context.Context,
			_ AgentNotifyRequest,
			credentials agentidentity.Credentials,
		) (AgentNotifyRecord, error) {
			assertAgentCredentials(t, credentials)
			return AgentNotifyRecord{Outcome: "rate-limited", RetryAfterMS: 875}, nil
		}

		stdout, _, err := executeRootCommand(t, deps, "notify", "Again", "-o", "json")
		if err != nil {
			t.Fatalf("compozy notify -o json error = %v", err)
		}
		var got AgentNotifyRecord
		if err := json.Unmarshal([]byte(stdout), &got); err != nil {
			t.Fatalf("json.Unmarshal(compozy notify) error = %v", err)
		}
		if got.Outcome != "rate-limited" || got.RetryAfterMS != 875 {
			t.Fatalf("compozy notify payload = %#v, want rate-limited/875", got)
		}
	})
}

func TestMeContextCommandJSONKeepsStableSectionOrder(t *testing.T) {
	t.Parallel()

	t.Run("Should keep stable JSON section order", func(t *testing.T) {
		t.Parallel()

		client := &stubClient{}
		deps := newAgentCommandTestDeps(t, client)
		client.agentContextFn = func(_ context.Context, credentials agentidentity.Credentials) (AgentContextRecord, error) {
			assertAgentCredentials(t, credentials)
			return AgentContextRecord{
				Self: contract.AgentIdentityPayload{
					SessionID: "sess-agent",
					AgentName: "coder",
					Provider:  "test-provider",
				},
				Workspace: contract.AgentWorkspacePayload{ID: "ws-1", RootDir: "/workspace/project"},
				Session: contract.AgentSessionPayload{
					ID:        "sess-agent",
					State:     session.StateActive,
					CreatedAt: fixedTestNow,
					UpdatedAt: fixedTestNow,
				},
				Task:         contract.AgentTaskContextPayload{Available: true},
				Capabilities: contract.AgentCapabilitySectionPayload{},
				Limits:       contract.AgentLimitsPayload{ContextSectionLimit: 20},
				Provenance: contract.AgentContextProvenancePayload{
					GeneratedAt: fixedTestNow,
					Source:      "test",
				},
			}, nil
		}

		stdout, _, err := executeRootCommand(t, deps, "me", "context", "-o", "json")
		if err != nil {
			t.Fatalf("compozy me context error = %v", err)
		}

		assertJSONKeyOrder(t, stdout, []string{
			"self",
			"workspace",
			"session",
			"task",
			"capabilities",
			"limits",
			"provenance",
		})
	})
}

func TestSpawnCommandMapsBoundedChildRequest(t *testing.T) {
	t.Parallel()

	t.Run("Should map bounded child request", func(t *testing.T) {
		t.Parallel()

		var gotRequest AgentSpawnRequest
		client := &stubClient{}
		deps := newAgentCommandTestDeps(t, client)
		client.agentSpawnFn = func(
			_ context.Context,
			request AgentSpawnRequest,
			credentials agentidentity.Credentials,
		) (AgentSpawnRecord, error) {
			assertAgentCredentials(t, credentials)
			gotRequest = request
			ttl := fixedTestNow.Add(2 * time.Minute)
			return AgentSpawnRecord{
				Session: SessionRecord{
					ID:        "sess-child",
					Name:      request.Name,
					AgentName: request.AgentName,
					Runtime: contract.SessionRuntimePayload{Effective: &contract.RuntimeSelectionPayload{
						Provider: request.Provider,
					}},
					WorkspaceID:   "ws-1",
					WorkspacePath: "/workspace/project",
					Type:          session.SessionTypeSpawned,
					State:         session.StateActive,
					CreatedAt:     fixedTestNow,
					UpdatedAt:     fixedTestNow,
				},
				Lineage: contract.SessionLineagePayload{
					ParentSessionID:  "sess-agent",
					RootSessionID:    "sess-agent",
					SpawnDepth:       1,
					SpawnRole:        request.SpawnRole,
					TTLExpiresAt:     &ttl,
					AutoStopOnParent: request.AutoStopOnParent,
					NotifyCreator:    request.NotifyCreator == nil || *request.NotifyCreator,
					SpawnBudget: contract.SpawnBudgetPayload{
						MaxChildren: 5,
						MaxDepth:    1,
						TTLSeconds:  request.TTLSeconds,
					},
					PermissionPolicy: request.Permissions,
				},
				Permissions: request.Permissions,
			}, nil
		}

		stdout, _, err := executeRootCommand(
			t,
			deps,
			"spawn",
			"--agent",
			"coder",
			"--provider",
			"codex",
			"--model",
			"gpt-test",
			"--reasoning-effort",
			"high",
			"--speed",
			"fast",
			"--acp-option",
			"context=1m",
			"--acp-toggle",
			"thinking=true",
			"--name",
			"child",
			"--workspace",
			"ws-target",
			"--prompt-overlay",
			"focus",
			"--role",
			"worker",
			"--ttl-seconds",
			"120",
			"--tool",
			"read",
			"--skill",
			"go",
			"--mcp-server",
			"filesystem",
			"--workspace-path",
			"/workspace/project",

			"--idempotency-key",
			"spawn-1",
			"-o",
			"json",
		)
		if err != nil {
			t.Fatalf("compozy spawn error = %v", err)
		}
		if gotRequest.AgentName != "coder" ||
			gotRequest.Provider != "codex" ||
			gotRequest.Model != "gpt-test" ||
			gotRequest.ReasoningEffort != "high" ||
			gotRequest.Speed != speedpkg.SpeedFast ||
			len(gotRequest.ACPOptions) != 2 ||
			gotRequest.Name != "child" ||
			gotRequest.Workspace != "ws-target" ||
			gotRequest.PromptOverlay != "focus" ||
			gotRequest.SpawnRole != "worker" ||
			gotRequest.TTLSeconds != 120 ||
			!gotRequest.AutoStopOnParent ||
			gotRequest.NotifyCreator != nil ||
			gotRequest.IdempotencyKey != "spawn-1" {
			t.Fatalf("spawn request = %#v, want parsed bounded spawn request", gotRequest)
		}
		if gotRequest.ACPOptions[0].ID != "context" || gotRequest.ACPOptions[0].ValueID != "1m" ||
			gotRequest.ACPOptions[1].ID != "thinking" || gotRequest.ACPOptions[1].BoolValue == nil ||
			!*gotRequest.ACPOptions[1].BoolValue {
			t.Fatalf("spawn ACP options = %#v, want typed select and boolean", gotRequest.ACPOptions)
		}
		if len(gotRequest.Permissions.Tools) != 1 ||
			gotRequest.Permissions.Tools[0] != "read" ||
			len(gotRequest.Permissions.Skills) != 1 ||
			gotRequest.Permissions.Skills[0] != "go" ||
			len(gotRequest.Permissions.MCPServers) != 1 ||
			gotRequest.Permissions.MCPServers[0] != "filesystem" ||
			len(gotRequest.Permissions.WorkspacePaths) != 1 ||
			gotRequest.Permissions.WorkspacePaths[0] != "/workspace/project" {
			t.Fatalf("spawn permissions = %#v, want all repeatable atom flags", gotRequest.Permissions)
		}

		var output AgentSpawnRecord
		if err := json.Unmarshal([]byte(stdout), &output); err != nil {
			t.Fatalf("json.Unmarshal(spawn output) error = %v", err)
		}
		if output.Session.ID != "sess-child" || output.Lineage.ParentSessionID != "sess-agent" {
			t.Fatalf("spawn output = %#v, want child session with parent lineage", output)
		}

		defaultStdout, _, err := executeRootCommand(
			t, deps, "spawn", "--agent", "coder", "--ttl-seconds", "120",
		)
		if err != nil {
			t.Fatalf("compozy spawn with default wake error = %v", err)
		}
		if !strings.Contains(defaultStdout, "Wake") || !strings.Contains(defaultStdout, "on-settle (default)") {
			t.Fatalf("default spawn output = %q, want effective wake state", defaultStdout)
		}

		disabledStdout, _, err := executeRootCommand(
			t, deps, "spawn", "--agent", "coder", "--ttl-seconds", "120", "--no-notify-creator",
		)
		if err != nil {
			t.Fatalf("compozy spawn --no-notify-creator error = %v", err)
		}
		if gotRequest.NotifyCreator == nil || *gotRequest.NotifyCreator {
			t.Fatalf("spawn request = %#v, want explicit notify_creator=false", gotRequest)
		}
		if !strings.Contains(disabledStdout, "Wake") || !strings.Contains(disabledStdout, "off") {
			t.Fatalf("disabled spawn output = %q, want wake off", disabledStdout)
		}
	})
}

func TestAgentCommandsRejectMissingIdentityBeforeAgentCalls(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args []string
	}{
		{name: "Should reject me without identity", args: []string{"me", "-o", "json"}},
		{name: "Should reject me context without identity", args: []string{"me", "context", "-o", "json"}},
		{name: "Should reject task next without identity", args: []string{"task", "next", "-o", "json"}},
		{
			name: "Should reject task next with a workspace before resolving it",
			args: []string{"task", "next", "--workspace", "ws-foreign", "-o", "json"},
		},
		{
			name: "Should reject task heartbeat without identity",
			args: []string{"task", "heartbeat", "run-1", "-o", "json"},
		},
		{
			name: "Should reject task complete without identity",
			args: []string{"task", "complete", "run-1", "-o", "json"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client := &stubClient{
				getWorkspaceFn: func(context.Context, string) (WorkspaceDetailRecord, error) {
					t.Fatal("GetWorkspace should not be called when agent env identity is missing")
					return WorkspaceDetailRecord{}, errors.New("unexpected")
				},
			}
			_, _, err := executeRootCommand(t, newMissingAgentIdentityDeps(t, client), tt.args...)
			if !errors.Is(err, agentidentity.ErrIdentityRequired) {
				t.Fatalf("executeRootCommand(%v) error = %v, want ErrIdentityRequired", tt.args, err)
			}
		})
	}
}

func TestAgentCommandsRenderHumanAndToonOutputs(t *testing.T) {
	t.Parallel()

	t.Run("Should render human and toon outputs", func(t *testing.T) {
		t.Parallel()

		client := &stubClient{}
		deps := newAgentCommandTestDeps(t, client)
		meRecord := AgentMeRecord{
			Self: contract.AgentIdentityPayload{
				SessionID: "sess-agent",
				AgentName: "coder",
				Provider:  "test-provider",
				Model:     "test-model",
			},
			Workspace: contract.AgentWorkspacePayload{ID: "ws-1", RootDir: "/workspace/project"},
			Session: contract.AgentSessionPayload{
				ID:        "sess-agent",
				State:     session.StateActive,
				CreatedAt: fixedTestNow,
				UpdatedAt: fixedTestNow,
			},
		}
		contextRecord := AgentContextRecord{
			Self:      meRecord.Self,
			Workspace: meRecord.Workspace,
			Session:   meRecord.Session,
			Provenance: contract.AgentContextProvenancePayload{
				GeneratedAt: fixedTestNow,
				Source:      "test",
			},
		}

		client.agentMeFn = func(context.Context, agentidentity.Credentials) (AgentMeRecord, error) {
			return meRecord, nil
		}
		client.agentContextFn = func(context.Context, agentidentity.Credentials) (AgentContextRecord, error) {
			return contextRecord, nil
		}

		tests := []struct {
			name string
			args []string
			want string
		}{
			{name: "Should render me human output", args: []string{"me", "-o", "human"}, want: "Agent"},
			{name: "Should render me toon output", args: []string{"me", "-o", "toon"}, want: "agent_me{session_id"},
			{
				name: "Should render context human output",
				args: []string{"me", "context", "-o", "human"},
				want: `"source": "test"`,
			},
			{
				name: "Should render context toon output",
				args: []string{"me", "context", "-o", "toon"},
				want: `"source": "test"`,
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				stdout, _, err := executeRootCommand(t, deps, tt.args...)
				if err != nil {
					t.Fatalf("executeRootCommand(%v) error = %v", tt.args, err)
				}
				if !strings.Contains(stdout, tt.want) {
					t.Fatalf("output = %q, want substring %q", stdout, tt.want)
				}
			})
		}
	})
}

func newAgentCommandTestDeps(t *testing.T, client *stubClient) commandDeps {
	t.Helper()

	client.getSessionFn = func(_ context.Context, id string) (SessionRecord, error) {
		if id != "sess-agent" {
			return SessionRecord{}, session.ErrSessionNotFound
		}
		return agentCommandSessionRecord(), nil
	}
	deps := newWorkspaceTestDeps(t, client)
	deps.getenv = agentCommandEnv
	return deps
}

func newMissingAgentIdentityDeps(t *testing.T, client *stubClient) commandDeps {
	t.Helper()

	client.getSessionFn = func(context.Context, string) (SessionRecord, error) {
		t.Fatal("GetSession should not be called when agent env identity is missing")
		return SessionRecord{}, errors.New("unexpected")
	}
	return newWorkspaceTestDeps(t, client)
}

func agentCommandEnv(key string) string {
	switch key {
	case agentidentity.EnvSessionID:
		return "sess-agent"
	case agentidentity.EnvAgent:
		return "coder"
	default:
		return ""
	}
}

func agentCommandSessionRecord() SessionRecord {
	return SessionRecord{
		ID:        "sess-agent",
		ProfileID: store.DefaultProfileID,
		Name:      "worker",
		AgentName: "coder",
		Runtime: contract.SessionRuntimePayload{Effective: &contract.RuntimeSelectionPayload{
			Provider: "test-provider",
		}},
		WorkspaceID:   "ws-1",
		WorkspacePath: "/workspace/project",
		Type:          session.SessionTypeUser,
		State:         session.StateActive,
		CreatedAt:     fixedTestNow,
		UpdatedAt:     fixedTestNow,
	}
}

func assertAgentCredentials(t *testing.T, credentials agentidentity.Credentials) {
	t.Helper()

	if credentials.SessionID != "sess-agent" ||
		credentials.AgentName != "coder" {
		t.Fatalf("credentials = %#v, want validated agent env identity", credentials)
	}
}

func assertJSONKeyOrder(t *testing.T, output string, keys []string) {
	t.Helper()

	previousIndex := -1
	for _, key := range keys {
		index := strings.Index(output, `"`+key+`"`)
		if index < 0 {
			t.Fatalf("JSON output missing key %q: %s", key, output)
		}
		if index <= previousIndex {
			t.Fatalf("JSON key %q appears out of order in %s", key, output)
		}
		previousIndex = index
	}
}
