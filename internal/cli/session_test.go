package cli

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/compozy/agh/internal/acp"
	"github.com/compozy/agh/internal/api/contract"
	"github.com/compozy/agh/internal/network/participation"
	"github.com/compozy/agh/internal/session"
	"github.com/compozy/agh/internal/store"
)

func TestParseSinceFlagRFC3339(t *testing.T) {
	t.Parallel()

	got, err := parseSinceFlag("2026-04-03T11:55:00Z", func() time.Time { return fixedTestNow })
	if err != nil {
		t.Fatalf("parseSinceFlag(RFC3339) error = %v", err)
	}
	if want := time.Date(2026, 4, 3, 11, 55, 0, 0, time.UTC); !got.Equal(want) {
		t.Fatalf("parseSinceFlag(RFC3339) = %s, want %s", got, want)
	}
}

func TestParseSinceFlagRelativeDuration(t *testing.T) {
	t.Parallel()

	got, err := parseSinceFlag("5m", func() time.Time { return fixedTestNow })
	if err != nil {
		t.Fatalf("parseSinceFlag(5m) error = %v", err)
	}
	if want := fixedTestNow.Add(-5 * time.Minute); !got.Equal(want) {
		t.Fatalf("parseSinceFlag(5m) = %s, want %s", got, want)
	}
}

func TestSessionNewUsesConfigDefaultWhenAgentFlagIsOmitted(t *testing.T) {
	t.Parallel()

	deps := newTestDeps(t, &stubClient{
		createSessionFn: func(_ context.Context, request CreateSessionRequest) (SessionRecord, error) {
			if request.AgentName != "" {
				t.Fatalf("CreateSession() AgentName = %q, want empty", request.AgentName)
			}
			if request.WorkspacePath != "/workspace/project" || request.Workspace != "" {
				t.Fatalf("CreateSession() request = %#v, want workspace_path only", request)
			}
			return SessionRecord{
				ID:            "sess-1",
				AgentName:     "general",
				WorkspaceID:   "ws-1",
				WorkspacePath: request.WorkspacePath,
				State:         session.StateActive,
				CreatedAt:     fixedTestNow,
				UpdatedAt:     fixedTestNow,
			}, nil
		},
	})

	stdout, _, err := executeRootCommand(t, deps, "session", "new", "-o", "json")
	if err != nil {
		t.Fatalf("executeRootCommand(session new) error = %v", err)
	}

	var decoded SessionRecord
	if err := json.Unmarshal([]byte(stdout), &decoded); err != nil {
		t.Fatalf("json.Unmarshal(session new) error = %v", err)
	}
	if decoded.AgentName != "general" {
		t.Fatalf("decoded.AgentName = %q, want %q", decoded.AgentName, "general")
	}
}

func TestSessionNewWaitsForStartupByDefault(t *testing.T) {
	t.Parallel()

	var statusCalls int
	deps := newTestDeps(t, &stubClient{
		createSessionFn: func(_ context.Context, request CreateSessionRequest) (SessionRecord, error) {
			if request.Prompt != "Plan the migration" {
				t.Fatalf("CreateSession() Prompt = %q, want initial prompt", request.Prompt)
			}
			return SessionRecord{ID: "sess-1", State: session.StateStarting}, nil
		},
		getSessionFn: func(_ context.Context, id string) (SessionRecord, error) {
			statusCalls++
			if id != "sess-1" {
				t.Fatalf("GetSession() id = %q, want sess-1", id)
			}
			return SessionRecord{ID: id, State: session.StateActive}, nil
		},
	})
	deps.pollInterval = time.Millisecond

	stdout, _, err := executeRootCommand(t, deps, "session", "new", "--prompt", "Plan the migration", "-o", "json")
	if err != nil {
		t.Fatalf("executeRootCommand(session new) error = %v", err)
	}
	if statusCalls != 1 {
		t.Fatalf("GetSession() calls = %d, want 1", statusCalls)
	}
	var decoded SessionRecord
	if err := json.Unmarshal([]byte(stdout), &decoded); err != nil {
		t.Fatalf("json.Unmarshal(session new) error = %v", err)
	}
	if decoded.State != session.StateActive {
		t.Fatalf("decoded.State = %q, want %q", decoded.State, session.StateActive)
	}
}

func TestSessionNewNoWaitReturnsAcceptedStartingSession(t *testing.T) {
	t.Parallel()

	deps := newTestDeps(t, &stubClient{
		createSessionFn: func(_ context.Context, _ CreateSessionRequest) (SessionRecord, error) {
			return SessionRecord{ID: "sess-1", State: session.StateStarting}, nil
		},
		getSessionFn: func(context.Context, string) (SessionRecord, error) {
			t.Fatal("GetSession() called with --no-wait")
			return SessionRecord{}, nil
		},
	})

	stdout, _, err := executeRootCommand(t, deps, "session", "new", "--no-wait", "-o", "json")
	if err != nil {
		t.Fatalf("executeRootCommand(session new --no-wait) error = %v", err)
	}
	var decoded SessionRecord
	if err := json.Unmarshal([]byte(stdout), &decoded); err != nil {
		t.Fatalf("json.Unmarshal(session new --no-wait) error = %v", err)
	}
	if decoded.State != session.StateStarting {
		t.Fatalf("decoded.State = %q, want %q", decoded.State, session.StateStarting)
	}
}

func TestSessionNewPassesInitialPromptWithRuntimeOverrides(t *testing.T) {
	t.Parallel()

	t.Run("Should serialize the prompt with runtime overrides in no-wait mode", func(t *testing.T) {
		t.Parallel()

		deps := newTestDeps(t, &stubClient{
			createSessionFn: func(_ context.Context, request CreateSessionRequest) (SessionRecord, error) {
				if request.Prompt != "Investigate the failing build" {
					t.Fatalf("CreateSession() Prompt = %q, want initial prompt", request.Prompt)
				}
				if request.Provider != "codex" || request.Model != "gpt-5.6-sol" || request.ReasoningEffort != "high" {
					t.Fatalf("CreateSession() runtime overrides = %#v", request)
				}
				return SessionRecord{ID: "sess-1", State: session.StateStarting}, nil
			},
			getSessionFn: func(context.Context, string) (SessionRecord, error) {
				t.Fatal("GetSession() called with --no-wait --prompt")
				return SessionRecord{}, nil
			},
		})

		stdout, _, err := executeRootCommand(
			t,
			deps,
			"session",
			"new",
			"--no-wait",
			"--prompt",
			"Investigate the failing build",
			"--provider",
			"codex",
			"--model",
			"gpt-5.6-sol",
			"--reasoning-effort",
			"high",
			"-o",
			"json",
		)
		if err != nil {
			t.Fatalf("executeRootCommand(session new --no-wait --prompt) error = %v", err)
		}
		var decoded SessionRecord
		if err := json.Unmarshal([]byte(stdout), &decoded); err != nil {
			t.Fatalf("json.Unmarshal(session new --no-wait --prompt) error = %v", err)
		}
		if decoded.State != session.StateStarting {
			t.Fatalf("decoded.State = %q, want %q", decoded.State, session.StateStarting)
		}
	})
}

func TestSessionNewReportsDurableStartupFailure(t *testing.T) {
	t.Parallel()

	deps := newTestDeps(t, &stubClient{
		createSessionFn: func(_ context.Context, _ CreateSessionRequest) (SessionRecord, error) {
			return SessionRecord{ID: "sess-1", State: session.StateStarting}, nil
		},
		getSessionFn: func(_ context.Context, id string) (SessionRecord, error) {
			return SessionRecord{
				ID:    id,
				State: session.StateStopped,
				Failure: &contract.SessionFailurePayload{
					Kind:    store.FailureStartup,
					Summary: "provider authentication expired",
				},
			}, nil
		},
	})
	deps.pollInterval = time.Millisecond

	_, _, err := executeRootCommand(t, deps, "session", "new", "-o", "json")
	if err == nil || !strings.Contains(err.Error(), "provider authentication expired") {
		t.Fatalf("executeRootCommand(session new) error = %v, want startup failure summary", err)
	}
}

func TestSessionNewWorkspaceOptions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		args    []string
		request CreateSessionRequest
	}{
		{
			name: "registered workspace",
			args: []string{"session", "new", "--workspace", "ws_abc", "-o", "json"},
			request: CreateSessionRequest{
				Workspace: "ws_abc",
			},
		},
		{
			name: "explicit cwd",
			args: []string{"session", "new", "--cwd", "/tmp/proj", "-o", "json"},
			request: CreateSessionRequest{
				WorkspacePath: "/tmp/proj",
			},
		},
		{
			name: "default cwd fallback",
			args: []string{"session", "new", "-o", "json"},
			request: CreateSessionRequest{
				WorkspacePath: "/workspace/project",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			deps := newTestDeps(t, &stubClient{
				createSessionFn: func(_ context.Context, request CreateSessionRequest) (SessionRecord, error) {
					if request.Workspace != tt.request.Workspace || request.WorkspacePath != tt.request.WorkspacePath {
						t.Fatalf("CreateSession() request = %#v, want %#v", request, tt.request)
					}
					return SessionRecord{
						ID:            "sess-1",
						AgentName:     "general",
						WorkspaceID:   "ws-1",
						WorkspacePath: request.WorkspacePath,
						State:         session.StateActive,
						CreatedAt:     fixedTestNow,
						UpdatedAt:     fixedTestNow,
					}, nil
				},
			})

			stdout, _, err := executeRootCommand(t, deps, tt.args...)
			if err != nil {
				t.Fatalf("executeRootCommand(%v) error = %v", tt.args, err)
			}

			var decoded SessionRecord
			if err := json.Unmarshal([]byte(stdout), &decoded); err != nil {
				t.Fatalf("json.Unmarshal(session new) error = %v", err)
			}
			if decoded.ID != "sess-1" {
				t.Fatalf("decoded.ID = %q, want %q", decoded.ID, "sess-1")
			}
		})
	}
}

func TestSessionNewPassesNetworkParticipationFlags(t *testing.T) {
	t.Parallel()

	deps := newTestDeps(t, &stubClient{
		createSessionFn: func(_ context.Context, request CreateSessionRequest) (SessionRecord, error) {
			assertLiveNamedParticipationRequest(t, request.NetworkParticipation, "builders")
			return SessionRecord{
				ID:                           "sess-1",
				AgentName:                    "general",
				WorkspaceID:                  "ws-1",
				WorkspacePath:                request.WorkspacePath,
				ResolvedNetworkParticipation: testLiveResolvedParticipation("builders"),
				State:                        session.StateActive,
				CreatedAt:                    fixedTestNow,
				UpdatedAt:                    fixedTestNow,
			}, nil
		},
	})

	stdout, _, err := executeRootCommand(
		t,
		deps,
		"session",
		"new",
		"--network",
		"live",
		"--network-channel-strategy",
		"named",
		"--network-channel",
		"builders",
		"-o",
		"json",
	)
	if err != nil {
		t.Fatalf("executeRootCommand(session new --network*) error = %v", err)
	}

	var decoded SessionRecord
	if err := json.Unmarshal([]byte(stdout), &decoded); err != nil {
		t.Fatalf("json.Unmarshal(session new --network*) error = %v", err)
	}
	assertResolvedParticipation(t, decoded.ResolvedNetworkParticipation, *testLiveResolvedParticipation("builders"))
}

func TestSessionNewAdvertisesAndAcceptsOnlyNamedChannelStrategy(t *testing.T) {
	t.Parallel()

	t.Run("Should advertise only the named channel strategy", func(t *testing.T) {
		t.Parallel()

		stdout, _, err := executeRootCommand(t, newTestDeps(t, &stubClient{}), "session", "new", "--help")
		if err != nil {
			t.Fatalf("executeRootCommand(session new --help) error = %v", err)
		}
		if !strings.Contains(stdout, "Live channel strategy: named") ||
			strings.Contains(stdout, "named, run, or loop_run") {
			t.Fatalf("session new help = %q, want named-only channel strategy", stdout)
		}
	})

	for _, strategy := range []participation.ChannelStrategy{
		participation.StrategyRun,
		participation.StrategyLoopRun,
	} {
		t.Run("Should reject unsupported "+string(strategy)+" strategy", func(t *testing.T) {
			t.Parallel()

			deps := newTestDeps(t, &stubClient{
				createSessionFn: func(context.Context, CreateSessionRequest) (SessionRecord, error) {
					t.Fatal("CreateSession should not be called for an unsupported session strategy")
					return SessionRecord{}, nil
				},
			})
			_, _, err := executeRootCommand(
				t,
				deps,
				"session", "new",
				"--network", "live",
				"--network-channel-strategy", string(strategy),
			)
			if err == nil || !strings.Contains(err.Error(), "must be named for session creation") {
				t.Fatalf("session new %s strategy error = %v, want named-only rejection", strategy, err)
			}
		})
	}
}

func TestSessionNewPassesProviderFlag(t *testing.T) {
	t.Parallel()

	deps := newTestDeps(t, &stubClient{
		createSessionFn: func(_ context.Context, request CreateSessionRequest) (SessionRecord, error) {
			if request.Provider != "fake-alt" {
				t.Fatalf("CreateSession() Provider = %q, want %q", request.Provider, "fake-alt")
			}
			return SessionRecord{
				ID:            "sess-1",
				AgentName:     "general",
				Provider:      request.Provider,
				WorkspaceID:   "ws-1",
				WorkspacePath: request.WorkspacePath,
				State:         session.StateActive,
				CreatedAt:     fixedTestNow,
				UpdatedAt:     fixedTestNow,
			}, nil
		},
	})

	stdout, _, err := executeRootCommand(
		t,
		deps,
		"session",
		"new",
		"--provider",
		"fake-alt",
		"-o",
		"json",
	)
	if err != nil {
		t.Fatalf("executeRootCommand(session new --provider) error = %v", err)
	}

	var decoded SessionRecord
	if err := json.Unmarshal([]byte(stdout), &decoded); err != nil {
		t.Fatalf("json.Unmarshal(session new --provider) error = %v", err)
	}
	if decoded.Provider != "fake-alt" {
		t.Fatalf("decoded.Provider = %q, want %q", decoded.Provider, "fake-alt")
	}
}

func TestSessionNewRejectsInvalidWorkspaceFlags(t *testing.T) {
	t.Parallel()

	code, _, stderr := executeRootCommandWithExit(t, newTestDeps(t, &stubClient{}),
		"session", "new", "--workspace", "ws_abc", "--cwd", "/tmp/proj",
	)
	if code != 1 {
		t.Fatalf("executeRootCommandWithExit() code = %d, want 1", code)
	}
	if !strings.Contains(stderr, "--workspace and --cwd are mutually exclusive") {
		t.Fatalf("stderr = %q, want workspace flag validation message", stderr)
	}
}

func TestSessionNewRejectsRelativeCWD(t *testing.T) {
	t.Parallel()

	tests := []string{".", "../project"}
	for _, cwd := range tests {
		t.Run(cwd, func(t *testing.T) {
			t.Parallel()

			code, _, stderr := executeRootCommandWithExit(t, newTestDeps(t, &stubClient{}),
				"session", "new", "--cwd", cwd,
			)
			if code != 1 {
				t.Fatalf("executeRootCommandWithExit(%q) code = %d, want 1", cwd, code)
			}
			if !strings.Contains(stderr, "--cwd must be an absolute path") {
				t.Fatalf("stderr = %q, want absolute path validation message", stderr)
			}
		})
	}
}

func TestSessionListPassesServerOwnedPageFilters(t *testing.T) {
	t.Parallel()

	var seenQuery SessionListQuery

	deps := newTestDeps(t, &stubClient{
		listSessionPageFn: func(_ context.Context, query SessionListQuery) (SessionListPage, error) {
			seenQuery = query
			return SessionListPage{Sessions: []SessionRecord{{
				ID:            "sess-1",
				AgentName:     "general",
				WorkspaceID:   "ws-filtered",
				WorkspacePath: "/workspace/project",
				State:         session.StateActive,
				Health: &contract.SessionHealthPayload{
					State:  contract.SessionHealthStateIdle,
					Health: contract.SessionHealthHealthy,
				},
				CreatedAt: fixedTestNow,
				UpdatedAt: fixedTestNow,
			}}, Page: contract.CountedCursorPagePayload{
				NextCursor: "cursor-next",
				HasMore:    true,
				Total:      7,
				Limit:      2,
			}}, nil
		},
	})

	stdout, _, err := executeRootCommand(
		t,
		deps,
		"session",
		"list",
		"--workspace",
		"ws-filtered",
		"--all",
		"--state",
		"stopped",
		"--type",
		"user",
		"--agent",
		"general",
		"--query",
		"demo",
		"--sort",
		"last_activity",
		"--cursor",
		"cursor-prev",
		"--limit",
		"2",
		"--include-health",
		"-o",
		"json",
	)
	if err != nil {
		t.Fatalf("executeRootCommand(session list) error = %v", err)
	}
	if seenQuery.Workspace != "ws-filtered" || seenQuery.State != "stopped" || seenQuery.Type != "user" ||
		seenQuery.Agent != "general" || seenQuery.Query != "demo" ||
		!seenQuery.IncludeHealth ||
		seenQuery.Sort != "last_activity" || seenQuery.Cursor != "cursor-prev" || seenQuery.Limit != 2 {
		t.Fatalf("seenQuery = %#v, want all server-owned page filters", seenQuery)
	}

	var decoded SessionListPage
	if err := json.Unmarshal([]byte(stdout), &decoded); err != nil {
		t.Fatalf("json.Unmarshal(session list) error = %v", err)
	}
	if len(decoded.Sessions) != 1 || decoded.Sessions[0].WorkspaceID != "ws-filtered" ||
		decoded.Sessions[0].Health == nil || decoded.Sessions[0].Health.Health != contract.SessionHealthHealthy ||
		decoded.Page.NextCursor != "cursor-next" || decoded.Page.Total != 7 || !decoded.Page.HasMore {
		t.Fatalf("decoded = %#v, want filtered session", decoded)
	}
}

func TestSessionListDefaultsToExactActiveState(t *testing.T) {
	t.Parallel()

	var seenQuery SessionListQuery
	deps := newTestDeps(t, &stubClient{
		listSessionPageFn: func(_ context.Context, query SessionListQuery) (SessionListPage, error) {
			seenQuery = query
			return SessionListPage{Page: contract.CountedCursorPagePayload{Limit: session.DefaultListLimit}}, nil
		},
	})
	if _, _, err := executeRootCommand(t, deps, "session", "list", "-o", "json"); err != nil {
		t.Fatalf("executeRootCommand(session list) error = %v", err)
	}
	if seenQuery.State != string(session.StateActive) {
		t.Fatalf("seenQuery.State = %q, want exact active hard cut", seenQuery.State)
	}
}

func TestSessionListJSONLIncludesContinuationRecord(t *testing.T) {
	t.Parallel()

	deps := newTestDeps(t, &stubClient{
		listSessionPageFn: func(_ context.Context, _ SessionListQuery) (SessionListPage, error) {
			return SessionListPage{
				Sessions: []SessionRecord{{
					ID:    "sess-1",
					State: session.StateActive,
					Health: &contract.SessionHealthPayload{
						State:  contract.SessionHealthStateIdle,
						Health: contract.SessionHealthHealthy,
					},
				}},
				Page: contract.CountedCursorPagePayload{
					NextCursor: "cursor-next",
					HasMore:    true,
					Total:      3,
					Limit:      1,
				},
			}, nil
		},
	})
	stdout, _, err := executeRootCommand(t, deps, "session", "list", "-o", "jsonl")
	if err != nil {
		t.Fatalf("executeRootCommand(session list jsonl) error = %v", err)
	}
	lines := strings.Split(strings.TrimSpace(stdout), "\n")
	if len(lines) != 2 {
		t.Fatalf("session list jsonl lines = %d, want session + page; output=%q", len(lines), stdout)
	}
	var item SessionRecord
	if err := json.Unmarshal([]byte(lines[0]), &item); err != nil {
		t.Fatalf("json.Unmarshal(session jsonl item) error = %v", err)
	}
	if item.Health == nil || item.Health.State != contract.SessionHealthStateIdle ||
		item.Health.Health != contract.SessionHealthHealthy {
		t.Fatalf("session jsonl item = %#v, want nested health", item)
	}
	var continuation struct {
		Type string                            `json:"type"`
		Page contract.CountedCursorPagePayload `json:"page"`
	}
	if err := json.Unmarshal([]byte(lines[1]), &continuation); err != nil {
		t.Fatalf("json.Unmarshal(page continuation) error = %v", err)
	}
	if continuation.Type != "page" || continuation.Page.NextCursor != "cursor-next" ||
		!continuation.Page.HasMore || continuation.Page.Total != 3 || continuation.Page.Limit != 1 {
		t.Fatalf("continuation = %#v, want usable page metadata", continuation)
	}
}

func TestSessionRepairPassesFlagsAndRendersJSON(t *testing.T) {
	t.Run("Should pass flags and render JSON", func(t *testing.T) {
		t.Parallel()

		var seenQuery SessionRepairQuery
		var seenID string
		deps := newTestDeps(t, &stubClient{
			repairSessionFn: func(_ context.Context, id string, query SessionRepairQuery) (SessionRepairRecord, error) {
				seenID = id
				seenQuery = query
				return SessionRepairRecord{
					SessionID: id,
					Issues: []SessionRepairIssueRecord{{
						Code:     session.RepairIssueStopReasonRequiresForce,
						Severity: session.RepairSeverityError,
						TurnID:   "turn-1",
					}},
					Actions: []SessionRepairActionRecord{{
						Code:      session.RepairActionAppendTerminalError,
						TurnID:    "turn-1",
						Persisted: false,
					}},
				}, nil
			},
		})

		stdout, _, err := executeRootCommand(
			t,
			deps,
			"session",
			"repair",
			"sess-1",
			"--dry-run",
			"--force",
			"-o",
			"json",
		)
		if err != nil {
			t.Fatalf("executeRootCommand(session repair) error = %v", err)
		}
		if seenID != "sess-1" || !seenQuery.DryRun || !seenQuery.Force {
			t.Fatalf("repair call = id %q query %#v, want dry-run force for sess-1", seenID, seenQuery)
		}

		var decoded SessionRepairRecord
		if err := json.Unmarshal([]byte(stdout), &decoded); err != nil {
			t.Fatalf("json.Unmarshal(session repair) error = %v", err)
		}
		if decoded.SessionID != "sess-1" || len(decoded.Issues) != 1 || len(decoded.Actions) != 1 {
			t.Fatalf("decoded repair = %#v, want one issue and one action for sess-1", decoded)
		}
	})
}

func TestSessionClarifyPendingUsesLiveDaemonProjection(t *testing.T) {
	t.Parallel()

	want := ClarificationsRecord{Clarifications: []ClarificationPendingRecord{{
		RequestID: "req-1",
		SessionID: "sess-1",
		AgentName: "reviewer",
		Question:  "Which workspace should I use?",
		Choices:   []string{"staging", "production"},
		AskedAt:   fixedTestNow,
		Deadline:  fixedTestNow.Add(5 * time.Minute),
	}}}
	deps := newTestDeps(t, &stubClient{
		listSessionClarificationsFn: func(_ context.Context, sessionID string) (ClarificationsRecord, error) {
			if sessionID != "sess-1" {
				t.Fatalf("session id = %q, want sess-1", sessionID)
			}
			return want, nil
		},
	})

	stdout, _, err := executeRootCommand(t, deps, "session", "clarify", "pending", "sess-1", "-o", "json")
	if err != nil {
		t.Fatalf("executeRootCommand(session clarify pending) error = %v", err)
	}
	var got ClarificationsRecord
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("json.Unmarshal(session clarify pending) error = %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("pending clarifications = %#v, want %#v", got, want)
	}
}

func TestSessionClarifyAnswerTranslatesOneBasedChoiceAtCLIBoundary(t *testing.T) {
	t.Parallel()

	wireChoice := 1
	deps := newTestDeps(t, &stubClient{
		answerSessionClarificationFn: func(
			_ context.Context,
			sessionID string,
			requestID string,
			request ClarificationAnswerRequest,
		) (ClarificationAnswerRecord, error) {
			if sessionID != "sess-1" || requestID != "req-1" {
				t.Fatalf("answer target = %q/%q, want sess-1/req-1", sessionID, requestID)
			}
			if request.ChoiceIndex == nil || *request.ChoiceIndex != wireChoice || request.Text != "" {
				t.Fatalf("answer request = %#v, want zero-based choice %d", request, wireChoice)
			}
			return ClarificationAnswerRecord{Choice: &wireChoice}, nil
		},
	})

	stdout, _, err := executeRootCommand(
		t,
		deps,
		"session",
		"clarify",
		"answer",
		"sess-1",
		"req-1",
		"--choice",
		"2",
		"-o",
		"json",
	)
	if err != nil {
		t.Fatalf("executeRootCommand(session clarify answer) error = %v", err)
	}
	var got ClarificationAnswerRecord
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("json.Unmarshal(session clarify answer) error = %v", err)
	}
	if got.Choice == nil || *got.Choice != wireChoice || got.Text != "" || got.Fallback {
		t.Fatalf("clarification answer = %#v, want exact wire result", got)
	}
}

func TestSessionClarifyAnswerRequiresExactlyOneAnswerFlag(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "Should reject a missing answer",
			wantErr: "exactly one of --choice or --text is required",
		},
		{
			name:    "Should reject both answer forms",
			args:    []string{"--choice", "1", "--text", "staging"},
			wantErr: "exactly one of --choice or --text is required",
		},
		{
			name:    "Should reject a zero choice",
			args:    []string{"--choice", "0"},
			wantErr: "--choice must be at least 1",
		},
		{
			name:    "Should reject blank text",
			args:    []string{"--text", "   "},
			wantErr: "--text cannot be blank",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			args := []string{"session", "clarify", "answer", "sess-1", "req-1"}
			args = append(args, tt.args...)
			_, _, err := executeRootCommand(t, newTestDeps(t, &stubClient{}), args...)
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf(
					"executeRootCommand(session clarify answer) error = %v, want %q",
					err,
					tt.wantErr,
				)
			}
		})
	}
}

func TestSessionEventsFollowUsesSSE(t *testing.T) {
	t.Parallel()

	var (
		streamCalled bool
		querySeen    SessionEventQuery
	)

	deps := newTestDeps(t, &stubClient{
		streamSessionFn: func(_ context.Context, id string, query SessionEventQuery, _ string, handler SSEHandler) error {
			streamCalled = true
			querySeen = query
			return handler(SSEEvent{
				ID:    "5",
				Event: session.EventTypeSessionStopped,
				Data: mustJSON(t, SessionEventRecord{
					ID:        "evt-5",
					SessionID: id,
					Sequence:  5,
					TurnID:    "turn-1",
					Type:      session.EventTypeSessionStopped,
					AgentName: "coder",
					Timestamp: fixedTestNow,
				}),
			})
		},
		sessionEventsFn: func(context.Context, string, SessionEventQuery) ([]SessionEventRecord, error) {
			t.Fatal("SessionEvents should not be called when --follow is set")
			return nil, nil
		},
	})

	stdout, _, err := executeRootCommand(
		t,
		deps,
		"session",
		"events",
		"sess-1",
		"--type",
		"tool_call",
		"--follow",
		"-o",
		"json",
	)
	if err != nil {
		t.Fatalf("executeRootCommand() error = %v", err)
	}
	if !streamCalled {
		t.Fatal("StreamSessionEvents was not called")
	}
	if querySeen.Type != "tool_call" {
		t.Fatalf("querySeen.Type = %q, want %q", querySeen.Type, "tool_call")
	}

	lines := strings.Split(strings.TrimSpace(stdout), "\n")
	if len(lines) != 1 {
		t.Fatalf("follow stdout lines = %d, want 1", len(lines))
	}
	var decoded SessionEventRecord
	if err := json.Unmarshal([]byte(lines[0]), &decoded); err != nil {
		t.Fatalf("json.Unmarshal(stream line) error = %v", err)
	}
	if decoded.Type != session.EventTypeSessionStopped {
		t.Fatalf("decoded.Type = %q, want %q", decoded.Type, session.EventTypeSessionStopped)
	}
}

func TestSessionEventsJSONLOutput(t *testing.T) {
	t.Run("Should render one persisted session event per JSONL line", func(t *testing.T) {
		t.Parallel()

		deps := newTestDeps(t, &stubClient{
			sessionEventsFn: func(_ context.Context, id string, query SessionEventQuery) ([]SessionEventRecord, error) {
				if id != "sess-1" {
					t.Fatalf("SessionEvents() id = %q, want sess-1", id)
				}
				if query.Type != "agent_message" {
					t.Fatalf("SessionEvents() query.Type = %q, want agent_message", query.Type)
				}
				if query.Last != 2 || query.AfterSequence != 7 {
					t.Fatalf("SessionEvents() query = %#v, want last 2 after 7", query)
				}
				return []SessionEventRecord{
					{
						ID:        "evt-1",
						SessionID: id,
						Sequence:  1,
						Type:      "agent_message",
						Timestamp: fixedTestNow,
					},
					{
						ID:        "evt-2",
						SessionID: id,
						Sequence:  2,
						Type:      "done",
						Timestamp: fixedTestNow,
					},
				}, nil
			},
		})

		stdout, _, err := executeRootCommand(
			t,
			deps,
			"session",
			"events",
			"sess-1",
			"--type",
			"agent_message",
			"--last",
			"2",
			"--after",
			"7",
			"-o",
			"jsonl",
		)
		if err != nil {
			t.Fatalf("executeRootCommand(session events jsonl) error = %v", err)
		}

		lines := strings.Split(strings.TrimSpace(stdout), "\n")
		if len(lines) != 2 {
			t.Fatalf("jsonl line count = %d, want 2; output=%q", len(lines), stdout)
		}
		var decoded SessionEventRecord
		if err := json.Unmarshal([]byte(lines[0]), &decoded); err != nil {
			t.Fatalf("json.Unmarshal(first session event line) error = %v", err)
		}
		if decoded.ID != "evt-1" || decoded.Sequence != 1 {
			t.Fatalf("decoded first event = %#v, want evt-1 sequence 1", decoded)
		}
	})
}

func TestSessionHistoryUsesCursorFlags(t *testing.T) {
	t.Run("Should use cursor flags for session history", func(t *testing.T) {
		t.Parallel()

		deps := newTestDeps(t, &stubClient{
			sessionHistoryFn: func(_ context.Context, id string, query SessionEventQuery) ([]TurnHistoryRecord, error) {
				if id != "sess-1" {
					t.Fatalf("SessionHistory() id = %q, want sess-1", id)
				}
				if query.Last != 3 || query.AfterSequence != 11 {
					t.Fatalf("SessionHistory() query = %#v, want last 3 after 11", query)
				}
				return []TurnHistoryRecord{
					{
						TurnID: "turn-1",
						Events: []SessionEventRecord{
							{
								ID:        "evt-12",
								SessionID: id,
								Sequence:  12,
								TurnID:    "turn-1",
								Type:      "agent_message",
								Timestamp: fixedTestNow,
							},
						},
					},
				}, nil
			},
		})

		stdout, _, err := executeRootCommand(
			t,
			deps,
			"session",
			"history",
			"sess-1",
			"--last",
			"3",
			"--after",
			"11",
			"-o",
			"json",
		)
		if err != nil {
			t.Fatalf("executeRootCommand(session history) error = %v", err)
		}
		var decoded []TurnHistoryRecord
		if err := json.Unmarshal([]byte(stdout), &decoded); err != nil {
			t.Fatalf("json.Unmarshal(session history) error = %v", err)
		}
		if len(decoded) != 1 || decoded[0].TurnID != "turn-1" {
			t.Fatalf("decoded history = %#v, want one turn", decoded)
		}
	})
}

func TestSessionWaitReturnsImmediatelyForStoppedSession(t *testing.T) {
	t.Parallel()

	deps := newTestDeps(t, &stubClient{
		getSessionFn: func(context.Context, string) (SessionRecord, error) {
			return SessionRecord{
				ID:            "sess-1",
				AgentName:     "coder",
				WorkspaceID:   "ws-1",
				WorkspacePath: "/workspace/project",
				State:         session.StateStopped,
				CreatedAt:     fixedTestNow,
				UpdatedAt:     fixedTestNow,
			}, nil
		},
		streamSessionFn: func(context.Context, string, SessionEventQuery, string, SSEHandler) error {
			t.Fatal("StreamSessionEvents should not be called for an already stopped session")
			return nil
		},
	})

	stdout, _, err := executeRootCommand(t, deps, "session", "wait", "sess-1", "-o", "json")
	if err != nil {
		t.Fatalf("executeRootCommand() error = %v", err)
	}

	var decoded SessionRecord
	if err := json.Unmarshal([]byte(stdout), &decoded); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if decoded.State != session.StateStopped {
		t.Fatalf("decoded.State = %q, want %q", decoded.State, session.StateStopped)
	}
}

func TestSessionWaitStreamsUntilStopped(t *testing.T) {
	t.Parallel()

	getCalls := 0
	deps := newTestDeps(t, &stubClient{
		getSessionFn: func(context.Context, string) (SessionRecord, error) {
			getCalls++
			state := session.StateActive
			if getCalls > 1 {
				state = session.StateStopped
			}
			return SessionRecord{
				ID:            "sess-1",
				AgentName:     "coder",
				WorkspaceID:   "ws-1",
				WorkspacePath: "/workspace/project",
				State:         state,
				CreatedAt:     fixedTestNow,
				UpdatedAt:     fixedTestNow,
			}, nil
		},
		streamSessionFn: func(_ context.Context, id string, _ SessionEventQuery, _ string, handler SSEHandler) error {
			return handler(SSEEvent{
				ID:    "2",
				Event: session.EventTypeSessionStopped,
				Data: mustJSON(t, SessionEventRecord{
					ID:        "evt-2",
					SessionID: id,
					Sequence:  2,
					Type:      session.EventTypeSessionStopped,
					AgentName: "coder",
					Timestamp: fixedTestNow,
				}),
			})
		},
	})

	stdout, _, err := executeRootCommand(t, deps, "session", "wait", "sess-1", "-o", "json")
	if err != nil {
		t.Fatalf("executeRootCommand() error = %v", err)
	}
	if getCalls != 2 {
		t.Fatalf("GetSession() calls = %d, want 2", getCalls)
	}

	var decoded SessionRecord
	if err := json.Unmarshal([]byte(stdout), &decoded); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if decoded.State != session.StateStopped {
		t.Fatalf("decoded.State = %q, want %q", decoded.State, session.StateStopped)
	}
}

func TestSessionStopFetchesUpdatedSession(t *testing.T) {
	t.Parallel()

	var stoppedID string

	deps := newTestDeps(t, &stubClient{
		stopSessionFn: func(_ context.Context, id string) error {
			stoppedID = id
			return nil
		},
		getSessionFn: func(_ context.Context, id string) (SessionRecord, error) {
			return SessionRecord{
				ID:            id,
				AgentName:     "coder",
				WorkspaceID:   "ws-1",
				WorkspacePath: "/workspace/project",
				State:         session.StateStopped,
				CreatedAt:     fixedTestNow,
				UpdatedAt:     fixedTestNow,
			}, nil
		},
	})

	stdout, _, err := executeRootCommand(t, deps, "session", "stop", "sess-1", "-o", "json")
	if err != nil {
		t.Fatalf("executeRootCommand() error = %v", err)
	}
	if stoppedID != "sess-1" {
		t.Fatalf("StopSession() id = %q, want %q", stoppedID, "sess-1")
	}

	var decoded SessionRecord
	if err := json.Unmarshal([]byte(stdout), &decoded); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if decoded.State != session.StateStopped {
		t.Fatalf("decoded.State = %q, want %q", decoded.State, session.StateStopped)
	}
}

func TestSessionRemoveDeletesSession(t *testing.T) {
	t.Parallel()

	t.Run("Should delete session and return session record", func(t *testing.T) {
		t.Parallel()

		var deletedID string

		deps := newTestDeps(t, &stubClient{
			getSessionFn: func(_ context.Context, id string) (SessionRecord, error) {
				return SessionRecord{
					ID:            id,
					AgentName:     "coder",
					WorkspaceID:   "ws-1",
					WorkspacePath: "/workspace/project",
					State:         session.StateStopped,
					CreatedAt:     fixedTestNow,
					UpdatedAt:     fixedTestNow,
				}, nil
			},
			deleteSessionFn: func(_ context.Context, id string) error {
				deletedID = id
				return nil
			},
		})

		stdout, _, err := executeRootCommand(t, deps, "session", "remove", "sess-1", "-o", "json")
		if err != nil {
			t.Fatalf("executeRootCommand() error = %v", err)
		}
		if deletedID != "sess-1" {
			t.Fatalf("DeleteSession() id = %q, want %q", deletedID, "sess-1")
		}

		var decoded SessionRecord
		if err := json.Unmarshal([]byte(stdout), &decoded); err != nil {
			t.Fatalf("json.Unmarshal() error = %v", err)
		}
		if decoded.ID != "sess-1" {
			t.Fatalf("decoded.ID = %q, want %q", decoded.ID, "sess-1")
		}
	})
}

func TestSessionStatusReturnsHealthStatus(t *testing.T) {
	t.Parallel()

	deps := newTestDeps(t, &stubClient{
		getSessionStatusFn: func(_ context.Context, id string) (SessionStatusRecord, error) {
			if id != "sess-1" {
				t.Fatalf("GetSessionStatus() id = %q, want sess-1", id)
			}
			return SessionStatusRecord{
				SessionID:       id,
				AgentName:       "coder",
				WorkspaceID:     "ws-1",
				State:           "idle",
				Health:          "healthy",
				Attachable:      true,
				EligibleForWake: true,
				UpdatedAt:       fixedTestNow,
			}, nil
		},
	})

	stdout, _, err := executeRootCommand(t, deps, "session", "status", "sess-1", "-o", "json")
	if err != nil {
		t.Fatalf("executeRootCommand() error = %v", err)
	}

	var decoded SessionStatusRecord
	if err := json.Unmarshal([]byte(stdout), &decoded); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if decoded.SessionID != "sess-1" || decoded.State != "idle" || !decoded.EligibleForWake {
		t.Fatalf("decoded = %#v, want sess-1 eligible idle health", decoded)
	}
}

func TestSessionUsageCommandPreservesCostProvenance(t *testing.T) {
	t.Parallel()

	t.Run("Should return the exact estimated usage contract as JSON", func(t *testing.T) {
		t.Parallel()

		input := int64(128_400)
		output := int64(24_900)
		total := input + output
		amount := 0.7587
		deps := newTestDeps(t, &stubClient{
			getSessionUsageFn: func(_ context.Context, id string) (SessionUsageRecord, error) {
				if id != "sess-1" {
					t.Fatalf("GetSessionUsage() id = %q, want sess-1", id)
				}
				return SessionUsageRecord{
					InputTokens:  &input,
					OutputTokens: &output,
					TotalTokens:  &total,
					TotalCost:    &amount,
					CostCurrency: "USD",
					CostStatus:   "estimated",
					CostSource:   "catalog_config",
					TurnCount:    1,
				}, nil
			},
		})

		stdout, _, err := executeRootCommand(t, deps, "session", "usage", "sess-1", "-o", "json")
		if err != nil {
			t.Fatalf("executeRootCommand() error = %v", err)
		}
		var decoded SessionUsageRecord
		if err := json.Unmarshal([]byte(stdout), &decoded); err != nil {
			t.Fatalf("json.Unmarshal() error = %v", err)
		}
		if decoded.TotalCost == nil || *decoded.TotalCost != amount || decoded.CostCurrency != "USD" ||
			decoded.CostStatus != "estimated" || decoded.CostSource != "catalog_config" {
			t.Fatalf("decoded usage = %#v, want estimated/catalog_config USD %.4f", decoded, amount)
		}
	})

	t.Run("Should render included usage without a monetary amount", func(t *testing.T) {
		t.Parallel()

		total := int64(42)
		contradictoryAmount := 99.0
		deps := newTestDeps(t, &stubClient{
			getSessionUsageFn: func(context.Context, string) (SessionUsageRecord, error) {
				return SessionUsageRecord{
					TotalTokens:  &total,
					TotalCost:    &contradictoryAmount,
					CostCurrency: "USD",
					CostStatus:   "included",
					CostSource:   "none",
					TurnCount:    1,
				}, nil
			},
		})

		stdout, _, err := executeRootCommand(t, deps, "session", "usage", "sess-1")
		if err != nil {
			t.Fatalf("executeRootCommand() error = %v", err)
		}
		if !strings.Contains(stdout, "Included") || strings.Contains(stdout, "USD 99") {
			t.Fatalf("session usage output = %q, want Included without monetary amount", stdout)
		}
	})

	t.Run("Should suppress an amount without authoritative status", func(t *testing.T) {
		t.Parallel()

		amount := 18.42
		deps := newTestDeps(t, &stubClient{
			getSessionUsageFn: func(context.Context, string) (SessionUsageRecord, error) {
				return SessionUsageRecord{TotalCost: &amount, CostCurrency: "USD"}, nil
			},
		})

		stdout, _, err := executeRootCommand(t, deps, "session", "usage", "sess-1")
		if err != nil {
			t.Fatalf("executeRootCommand() error = %v", err)
		}
		if strings.Contains(stdout, "USD 18.42") {
			t.Fatalf("session usage output = %q, want no amount without cost status", stdout)
		}
	})
}

func TestSessionResumeReturnsSessionRecord(t *testing.T) {
	t.Parallel()

	deps := newTestDeps(t, &stubClient{
		resumeSessionFn: func(_ context.Context, id string) (SessionRecord, error) {
			return SessionRecord{
				ID:            id,
				AgentName:     "coder",
				WorkspaceID:   "ws-1",
				WorkspacePath: "/workspace/project",
				State:         session.StateActive,
				CreatedAt:     fixedTestNow,
				UpdatedAt:     fixedTestNow,
			}, nil
		},
	})

	stdout, _, err := executeRootCommand(t, deps, "session", "resume", "sess-1", "-o", "json")
	if err != nil {
		t.Fatalf("executeRootCommand() error = %v", err)
	}

	var decoded SessionRecord
	if err := json.Unmarshal([]byte(stdout), &decoded); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if decoded.ID != "sess-1" || decoded.State != session.StateActive {
		t.Fatalf("decoded = %#v, want sess-1 active", decoded)
	}
}

func TestSessionResumeLatestUsesBoundedPage(t *testing.T) {
	t.Parallel()

	var seenQuery SessionListQuery
	deps := newTestDeps(t, &stubClient{
		listSessionPageFn: func(_ context.Context, query SessionListQuery) (SessionListPage, error) {
			seenQuery = query
			return SessionListPage{
				Sessions: []SessionRecord{{ID: "sess-latest", State: session.StateActive}},
				Page:     contract.CountedCursorPagePayload{Total: 4, Limit: 1},
			}, nil
		},
		resumeSessionFn: func(_ context.Context, id string) (SessionRecord, error) {
			if id != "sess-latest" {
				t.Fatalf("ResumeSession() id = %q, want sess-latest", id)
			}
			return SessionRecord{ID: id, State: session.StateActive}, nil
		},
	})
	if _, _, err := executeRootCommand(
		t,
		deps,
		"session",
		"resume",
		"--latest",
		"--workspace",
		"ws-alpha",
		"-o",
		"json",
	); err != nil {
		t.Fatalf("executeRootCommand(session resume --latest) error = %v", err)
	}
	if seenQuery.Workspace != "ws-alpha" || !seenQuery.Resumable ||
		seenQuery.Sort != session.ListSortLastActivity || seenQuery.Limit != 1 {
		t.Fatalf("latest query = %#v, want one resumable last-activity page", seenQuery)
	}
}

func TestSessionPromptRendersReturnedEvents(t *testing.T) {
	t.Parallel()

	var (
		promptID  string
		promptMsg string
	)

	deps := newTestDeps(t, &stubClient{
		sendSessionPromptFn: func(_ context.Context, id string, request SessionPromptRequest) (SessionPromptRecord, error) {
			promptID = id
			promptMsg = request.Message
			return SessionPromptRecord{Events: []AgentEventRecord{{
				SessionID: id,
				TurnID:    "turn-1",
				Type:      acp.EventTypeAgentMessage,
				Timestamp: fixedTestNow,
				Text:      "hello back",
			}}}, nil
		},
	})

	stdout, _, err := executeRootCommand(t, deps, "session", "prompt", "sess-1", "hello", "-o", "json")
	if err != nil {
		t.Fatalf("executeRootCommand() error = %v", err)
	}
	if promptID != "sess-1" || promptMsg != "hello" {
		t.Fatalf("PromptSession() = (%q, %q), want (%q, %q)", promptID, promptMsg, "sess-1", "hello")
	}

	var decoded []AgentEventRecord
	if err := json.Unmarshal([]byte(stdout), &decoded); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if len(decoded) != 1 || decoded[0].Text != "hello back" {
		t.Fatalf("decoded = %#v, want one agent event", decoded)
	}
}

func TestSessionPromptRendersStructuredGoalResult(t *testing.T) {
	t.Parallel()

	t.Run("Should render a flat structured Goal status result", func(t *testing.T) {
		t.Parallel()

		result := contract.GoalCommandResult{
			Outcome: contract.GoalOutcomeStatus,
			Snapshot: &contract.GoalSnapshot{
				RunID: "run-goal", NodeID: "goal", Objective: "ship safely",
				OriginSessionID: "sess-1", BoundSessionID: "sess-1",
				Status: "active", RunStatus: contract.LoopRunStatusRunning,
				TurnsUsed: 1, TurnLimit: 4, Live: true, ContractSummary: "objective satisfied",
				Context: contract.GoalContextSnapshot{State: contract.GoalContextUnknown, NudgeRatio: 0.8},
			},
		}
		deps := newTestDeps(t, &stubClient{
			sendSessionPromptFn: func(
				_ context.Context,
				id string,
				request SessionPromptRequest,
			) (SessionPromptRecord, error) {
				if id != "sess-1" || request.Message != "/goal status" {
					t.Fatalf("SendSessionPrompt() = %q/%q", id, request.Message)
				}
				return SessionPromptRecord{Goal: &result}, nil
			},
		})

		stdout, _, err := executeRootCommand(
			t,
			deps,
			"session", "prompt", "sess-1", "/goal status", "-o", "json",
		)
		if err != nil {
			t.Fatalf("executeRootCommand(session prompt Goal json) error = %v", err)
		}
		var decoded contract.GoalCommandResult
		if err := json.Unmarshal([]byte(stdout), &decoded); err != nil {
			t.Fatalf("json.Unmarshal(Goal command result) error = %v", err)
		}
		if decoded.Outcome != contract.GoalOutcomeStatus || decoded.Snapshot == nil ||
			decoded.Snapshot.RunID != "run-goal" {
			t.Fatalf("Goal command result = %#v", decoded)
		}
		var wrapped map[string]json.RawMessage
		if err := json.Unmarshal([]byte(stdout), &wrapped); err != nil {
			t.Fatalf("json.Unmarshal(Goal command object) error = %v", err)
		}
		if _, nested := wrapped["goal"]; nested {
			t.Fatalf("Goal command JSON was nested: %s", stdout)
		}
	})

	t.Run("Should surface Goal conflicts in structured formats", func(t *testing.T) {
		t.Parallel()

		for _, format := range []string{"json", "jsonl"} {
			t.Run("Should render the conflict as "+format, func(t *testing.T) {
				t.Parallel()

				reason := contract.GoalReasonReplaceRequired
				goalConflict := func() error {
					return &goalCommandAPIError{
						statusCode: 409,
						status:     "Conflict",
						result: contract.GoalCommandResult{
							Outcome: contract.GoalOutcomeError, ReasonCode: &reason,
						},
					}
				}
				errorDeps := newTestDeps(t, &stubClient{
					sendSessionPromptFn: func(
						context.Context,
						string,
						SessionPromptRequest,
					) (SessionPromptRecord, error) {
						return SessionPromptRecord{}, goalConflict()
					},
					streamPromptSessionFn: func(context.Context, string, string, SSEHandler) error {
						return goalConflict()
					},
				})
				exitCode, _, stderr := executeRootCommandWithExit(
					t,
					errorDeps,
					"session", "prompt", "sess-1", "/goal ship", "-o", format,
				)
				if exitCode == 0 {
					t.Fatalf("Goal conflict %s exit code = 0", format)
				}
				var errorResult contract.GoalCommandResult
				if err := json.Unmarshal([]byte(strings.TrimSpace(stderr)), &errorResult); err != nil {
					t.Fatalf("json.Unmarshal(Goal conflict %s) error = %v; output=%q", format, err, stderr)
				}
				if errorResult.Outcome != contract.GoalOutcomeError || errorResult.ReasonCode == nil ||
					*errorResult.ReasonCode != contract.GoalReasonReplaceRequired {
					t.Fatalf("Goal conflict %s = %#v", format, errorResult)
				}
			})
		}
	})
}

func TestSessionPromptBusyInputActions(t *testing.T) {
	t.Run("Should send explicit queue mode", func(t *testing.T) {
		t.Parallel()

		var gotRequest SessionPromptRequest
		deps := newTestDeps(t, &stubClient{
			sendSessionPromptFn: func(_ context.Context, id string, request SessionPromptRequest) (SessionPromptRecord, error) {
				if id != "sess-1" {
					t.Fatalf("SendSessionPrompt() id = %q, want sess-1", id)
				}
				gotRequest = request
				return SessionPromptRecord{Prompt: SessionPromptResultRecord{
					Status:          "queued",
					Mode:            contract.PromptModeQueue,
					Queued:          true,
					QueueEntryID:    "queue-1",
					QueuePosition:   1,
					QueueGeneration: 7,
				}}, nil
			},
		})

		stdout, _, err := executeRootCommand(t, deps, "session", "prompt", "sess-1", "hello", "--queue", "-o", "json")
		if err != nil {
			t.Fatalf("executeRootCommand(session prompt --queue) error = %v", err)
		}
		if gotRequest.Message != "hello" || gotRequest.Mode != contract.PromptModeQueue {
			t.Fatalf("SendSessionPrompt() request = %#v, want queue hello", gotRequest)
		}
		var decoded SessionPromptRecord
		if err := json.Unmarshal([]byte(stdout), &decoded); err != nil {
			t.Fatalf("json.Unmarshal(queue prompt) error = %v", err)
		}
		if decoded.Prompt.QueueEntryID != "queue-1" || !decoded.Prompt.Queued {
			t.Fatalf("decoded = %#v, want queued prompt result", decoded)
		}
	})

	t.Run("Should send explicit interrupt mode", func(t *testing.T) {
		t.Parallel()

		var gotRequest SessionPromptRequest
		deps := newTestDeps(t, &stubClient{
			sendSessionPromptFn: func(_ context.Context, id string, request SessionPromptRequest) (SessionPromptRecord, error) {
				if id != "sess-1" {
					t.Fatalf("SendSessionPrompt() id = %q, want sess-1", id)
				}
				gotRequest = request
				return SessionPromptRecord{Prompt: SessionPromptResultRecord{
					Status:      "interrupted",
					Mode:        contract.PromptModeInterrupt,
					Interrupted: true,
					NewTurnID:   "turn-2",
				}}, nil
			},
		})

		_, _, err := executeRootCommand(t, deps, "session", "prompt", "sess-1", "replace", "--interrupt")
		if err != nil {
			t.Fatalf("executeRootCommand(session prompt --interrupt) error = %v", err)
		}
		if gotRequest.Message != "replace" || gotRequest.Mode != contract.PromptModeInterrupt {
			t.Fatalf("SendSessionPrompt() request = %#v, want interrupt replace", gotRequest)
		}
	})

	t.Run("Should use steer endpoint", func(t *testing.T) {
		t.Parallel()

		var (
			gotID   string
			gotText string
		)
		deps := newTestDeps(t, &stubClient{
			steerSessionPromptFn: func(_ context.Context, id string, text string) (SessionPromptRecord, error) {
				gotID = id
				gotText = text
				return SessionPromptRecord{Prompt: SessionPromptResultRecord{
					Status:          "staged",
					Mode:            contract.PromptModeSteer,
					Staged:          true,
					QueueEntryID:    "steer-1",
					QueueGeneration: 3,
				}}, nil
			},
		})

		_, _, err := executeRootCommand(t, deps, "session", "prompt", "sess-1", "prefer small patch", "--steer")
		if err != nil {
			t.Fatalf("executeRootCommand(session prompt --steer) error = %v", err)
		}
		if gotID != "sess-1" || gotText != "prefer small patch" {
			t.Fatalf("SteerSessionPrompt() = (%q, %q), want (sess-1, prefer small patch)", gotID, gotText)
		}
	})

	t.Run("Should cancel queued entry with one arg", func(t *testing.T) {
		t.Parallel()

		var (
			gotID      string
			gotEntryID string
		)
		deps := newTestDeps(t, &stubClient{
			cancelQueuedSessionPromptFn: func(_ context.Context, id string, queueEntryID string) (SessionPromptRecord, error) {
				gotID = id
				gotEntryID = queueEntryID
				return SessionPromptRecord{Prompt: SessionPromptResultRecord{
					Status:        "canceled",
					QueueEntryID:  queueEntryID,
					QueuePosition: 1,
				}}, nil
			},
		})

		_, _, err := executeRootCommand(t, deps, "session", "prompt", "sess-1", "--cancel", "queue-1")
		if err != nil {
			t.Fatalf("executeRootCommand(session prompt --cancel) error = %v", err)
		}
		if gotID != "sess-1" || gotEntryID != "queue-1" {
			t.Fatalf("CancelQueuedSessionPrompt() = (%q, %q), want (sess-1, queue-1)", gotID, gotEntryID)
		}
	})

	t.Run("Should reject mutually exclusive busy actions", func(t *testing.T) {
		t.Parallel()

		deps := newTestDeps(t, &stubClient{})
		_, _, err := executeRootCommand(t, deps, "session", "prompt", "sess-1", "hello", "--queue", "--steer")
		if err == nil || !strings.Contains(err.Error(), "choose only one") {
			t.Fatalf("executeRootCommand(mutually exclusive) error = %v, want choose only one", err)
		}
	})
}

func TestSessionPromptJSONLOutput(t *testing.T) {
	t.Run("Should stream prompt events as JSONL without buffering the completed turn", func(t *testing.T) {
		t.Parallel()

		var streamCalled bool
		deps := newTestDeps(t, &stubClient{
			promptSessionFn: func(context.Context, string, string) ([]AgentEventRecord, error) {
				t.Fatal("PromptSession should not be called for prompt -o jsonl")
				return nil, nil
			},
			streamPromptSessionFn: func(_ context.Context, id string, message string, handler SSEHandler) error {
				streamCalled = true
				if id != "sess-1" || message != "hello" {
					t.Fatalf("StreamPromptSession() = (%q, %q), want (sess-1, hello)", id, message)
				}
				events := []SSEEvent{
					{
						Data: mustJSON(t, map[string]any{
							"type":      "start",
							"messageId": "turn-1",
						}),
					},
					{
						Data: mustJSON(t, map[string]any{
							"type": "text-start",
							"id":   "turn-1-text",
						}),
					},
					{
						Data: mustJSON(t, map[string]any{
							"type":  "text-delta",
							"id":    "turn-1-text",
							"delta": "hello back",
						}),
					},
					{
						Data: mustJSON(t, map[string]any{
							"type": "text-end",
							"id":   "turn-1-text",
						}),
					},
					{
						Data: mustJSON(t, map[string]any{
							"type":         "finish",
							"finishReason": "stop",
						}),
					},
					{
						Data: []byte("[DONE]"),
					},
				}
				for _, event := range events {
					if err := handler(event); err != nil {
						return err
					}
				}
				return nil
			},
		})

		stdout, _, err := executeRootCommand(t, deps, "session", "prompt", "sess-1", "hello", "-o", "jsonl")
		if err != nil {
			t.Fatalf("executeRootCommand(session prompt jsonl) error = %v", err)
		}
		if !streamCalled {
			t.Fatal("StreamPromptSession was not called")
		}

		lines := strings.Split(strings.TrimSpace(stdout), "\n")
		if len(lines) != 5 {
			t.Fatalf("jsonl line count = %d, want 5; output=%q", len(lines), stdout)
		}
		partTypes := make([]string, 0, len(lines))
		for _, line := range lines {
			var payload map[string]any
			if err := json.Unmarshal([]byte(line), &payload); err != nil {
				t.Fatalf("json.Unmarshal(prompt jsonl line) error = %v; line=%s", err, line)
			}
			partType, ok := payload["type"].(string)
			if !ok {
				t.Fatalf("prompt jsonl line missing type: %#v", payload)
			}
			partTypes = append(partTypes, partType)
		}
		wantTypes := []string{"start", "text-start", "text-delta", "text-end", "finish"}
		if !reflect.DeepEqual(partTypes, wantTypes) {
			t.Fatalf("prompt jsonl part types = %#v, want %#v", partTypes, wantTypes)
		}
	})

	t.Run("Should emit one direct line for a structured Goal result", func(t *testing.T) {
		t.Parallel()

		result := contract.GoalCommandResult{Outcome: contract.GoalOutcomeCleared}
		deps := newTestDeps(t, &stubClient{
			streamPromptSessionFn: func(
				_ context.Context,
				id string,
				message string,
				handler SSEHandler,
			) error {
				if id != "sess-1" || message != "/goal clear" {
					t.Fatalf("StreamPromptSession() = %q/%q", id, message)
				}
				return handler(SSEEvent{Event: goalResultEventName, Data: mustJSON(t, result)})
			},
		})

		stdout, _, err := executeRootCommand(
			t,
			deps,
			"session", "prompt", "sess-1", "/goal clear", "-o", "jsonl",
		)
		if err != nil {
			t.Fatalf("executeRootCommand(session prompt Goal jsonl) error = %v", err)
		}
		lines := strings.Split(strings.TrimSpace(stdout), "\n")
		if len(lines) != 1 {
			t.Fatalf("Goal prompt jsonl lines = %d, output=%q", len(lines), stdout)
		}
		var decoded contract.GoalCommandResult
		if err := json.Unmarshal([]byte(lines[0]), &decoded); err != nil {
			t.Fatalf("json.Unmarshal(Goal prompt jsonl) error = %v", err)
		}
		if decoded.Outcome != contract.GoalOutcomeCleared {
			t.Fatalf("Goal prompt jsonl = %#v", decoded)
		}
	})
}

func TestSessionListBundleRendersHumanAndToon(t *testing.T) {
	t.Parallel()

	items := []SessionRecord{{
		ID:                           "sess-1",
		Name:                         "demo",
		AgentName:                    "coder",
		Provider:                     "fake",
		WorkspaceID:                  "ws-1",
		WorkspacePath:                "/workspace/project",
		ResolvedNetworkParticipation: testLiveResolvedParticipation("builders"),
		State:                        session.StateActive,
		Health: &contract.SessionHealthPayload{
			State:  contract.SessionHealthStateIdle,
			Health: contract.SessionHealthHealthy,
		},
		UpdatedAt: fixedTestNow,
	}}

	bundle := sessionListBundle(SessionListPage{
		Sessions: items,
		Page: contract.CountedCursorPagePayload{
			NextCursor: "cursor-next",
			HasMore:    true,
			Total:      4,
			Limit:      len(items),
		},
	}, func() time.Time {
		return fixedTestNow.Add(time.Hour)
	})

	human, err := bundle.human()
	if err != nil {
		t.Fatalf("sessionListBundle().human() error = %v", err)
	}
	if !strings.Contains(human, "sess-1") ||
		!strings.Contains(strings.ToLower(human), "provider") ||
		!strings.Contains(human, "fake") ||
		!strings.Contains(human, "/workspace/project") ||
		!strings.Contains(strings.ToLower(human), "channel") ||
		!strings.Contains(human, "builders") || !strings.Contains(human, "idle") ||
		!strings.Contains(human, "healthy") || !strings.Contains(human, "cursor-next") {
		t.Fatalf("sessionListBundle().human() = %q, want session, workspace, channel, and health output", human)
	}

	toon, err := bundle.toon()
	if err != nil {
		t.Fatalf("sessionListBundle().toon() error = %v", err)
	}
	if !strings.Contains(toon, "sessions") ||
		!strings.Contains(toon, "sess-1") ||
		!strings.Contains(strings.ToLower(toon), "provider") ||
		!strings.Contains(toon, "fake") ||
		!strings.Contains(strings.ToLower(toon), "channel") ||
		!strings.Contains(toon, "builders") || !strings.Contains(toon, "idle") ||
		!strings.Contains(toon, "healthy") || !strings.Contains(toon, "cursor-next") ||
		!strings.Contains(toon, "has_more") {
		t.Fatalf("sessionListBundle().toon() = %q, want sessions array output with provider, channel, and health", toon)
	}
}

func TestSessionBundleRendersProviderInHumanAndToon(t *testing.T) {
	t.Parallel()

	bundle := sessionBundle(SessionRecord{
		ID:            "sess-1",
		Name:          "demo",
		AgentName:     "coder",
		Provider:      "fake",
		WorkspaceID:   "ws-1",
		WorkspacePath: "/workspace/project",
		State:         session.StateActive,
		CreatedAt:     fixedTestNow,
		UpdatedAt:     fixedTestNow,
	}, func() time.Time {
		return fixedTestNow.Add(time.Minute)
	})

	human, err := bundle.human()
	if err != nil {
		t.Fatalf("sessionBundle().human() error = %v", err)
	}
	if !strings.Contains(strings.ToLower(human), "provider") || !strings.Contains(human, "fake") {
		t.Fatalf("sessionBundle().human() = %q, want provider output", human)
	}

	toon, err := bundle.toon()
	if err != nil {
		t.Fatalf("sessionBundle().toon() error = %v", err)
	}
	if !strings.Contains(strings.ToLower(toon), "provider") || !strings.Contains(toon, "fake") {
		t.Fatalf("sessionBundle().toon() = %q, want provider output", toon)
	}
}
