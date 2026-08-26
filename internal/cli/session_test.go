package cli

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/acp"
	"github.com/compozy/compozy/internal/agentidentity"
	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/network/participation"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/store"
	toolspkg "github.com/compozy/compozy/internal/tools"
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

	deps := newWorkspaceTestDeps(t, &stubClient{
		createSessionFn: func(_ context.Context, request CreateSessionRequest) (SessionRecord, error) {
			if request.AgentName != "" {
				t.Fatalf("CreateSession() AgentName = %q, want empty", request.AgentName)
			}
			if request.Workspace != "/workspace/project" || request.WorkspacePath != "" {
				t.Fatalf("CreateSession() request = %#v, want canonical workspace", request)
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
		getSessionFn: func(context.Context, string) (SessionRecord, error) {
			t.Fatal("GetSession() called after logical session acceptance")
			return SessionRecord{}, nil
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

func TestSessionPromptPassesRuntimeSelection(t *testing.T) {
	t.Parallel()

	t.Run("Should serialize the runtime with the prompt", func(t *testing.T) {
		t.Parallel()

		var gotRequest SessionPromptRequest
		deps := newWorkspaceTestDeps(t, &stubClient{
			sendSessionPromptFn: func(_ context.Context, id string, request SessionPromptRequest) (SessionPromptRecord, error) {
				if id != "sess-1" {
					t.Fatalf("SendSessionPrompt() id = %q, want sess-1", id)
				}
				gotRequest = request
				return SessionPromptRecord{
					Prompt: SessionPromptResultRecord{Status: "accepted", NewTurnID: "turn-1"},
				}, nil
			},
		})

		stdout, _, err := executeRootCommand(
			t,
			deps,
			"session",
			"prompt",
			"sess-1",
			"Investigate the failing build",
			"--provider",
			"codex",
			"--model",
			"gpt-5.6-sol",
			"--reasoning-effort",
			"high",
			"--speed",
			"fast",
			"-o",
			"json",
		)
		if err != nil {
			t.Fatalf("executeRootCommand(session prompt) error = %v", err)
		}
		if gotRequest.Message != "Investigate the failing build" || gotRequest.Runtime == nil {
			t.Fatalf("SendSessionPrompt() request = %#v", gotRequest)
		}
		if gotRequest.MessageID == "" || gotRequest.IdempotencyKey == "" {
			t.Fatalf(
				"SendSessionPrompt() identity = %q/%q, want generated non-empty values",
				gotRequest.MessageID,
				gotRequest.IdempotencyKey,
			)
		}
		if gotRequest.Runtime.Provider != "codex" || gotRequest.Runtime.Model != "gpt-5.6-sol" ||
			gotRequest.Runtime.ReasoningEffort != "high" || gotRequest.Runtime.Speed != contract.SpeedFast {
			t.Fatalf("SendSessionPrompt() Runtime = %#v", gotRequest.Runtime)
		}
		var decoded SessionPromptRecord
		if err := json.Unmarshal([]byte(stdout), &decoded); err != nil {
			t.Fatalf("json.Unmarshal(session prompt) error = %v", err)
		}
		if decoded.Prompt.NewTurnID != "turn-1" {
			t.Fatalf("decoded prompt = %#v", decoded)
		}
	})
}

// Invariant: a prompt resolves and mutates its session inside the profile
// selected by the operator. The canonical CLI session command suite owns this
// command-context boundary.
func TestSessionPromptPreservesProfileScope(t *testing.T) {
	t.Parallel()

	t.Run("Should send the prompt in the selected profile", func(t *testing.T) {
		t.Parallel()

		client := &profileTestDaemonClient{
			DaemonClient: withWorkspaceResolution(&stubClient{
				sendSessionPromptFn: func(
					ctx context.Context,
					_ string,
					_ SessionPromptRequest,
				) (SessionPromptRecord, error) {
					if got := profileQueryValues(ctx, nil).Get(profileFlagName); got != "research" {
						t.Fatalf("SendSessionPrompt() profile query = %q, want research", got)
					}
					return SessionPromptRecord{
						Prompt: SessionPromptResultRecord{Status: "accepted", NewTurnID: "turn-1"},
					}, nil
				},
			}),
			profileClientAPI: &profileClientStub{profiles: []contract.Profile{
				{Name: "default", State: "active"},
				{Name: "research", State: "active"},
			}},
		}
		deps := newWorkspaceTestDeps(t, client)

		_, _, err := executeRootCommand(
			t,
			deps,
			"--profile",
			"research",
			"session",
			"prompt",
			"sess-1",
			"Investigate the failing build",
			"-o",
			"json",
		)
		if err != nil {
			t.Fatalf("executeRootCommand(session prompt --profile research) error = %v", err)
		}
	})
}

// Invariant: the session Goal CLI forwards the typed operation, target, and
// caller-selected runtime through the direct mutation API and preserves its
// structured result output. The canonical CLI session command suite owns this
// transport boundary.
func TestSessionGoalCommandForwardsTypedMutation(t *testing.T) {
	t.Parallel()

	t.Run("Should forward a typed Goal replacement mutation", func(t *testing.T) {
		t.Parallel()

		var gotID string
		var gotRequest contract.SessionGoalCommandRequest
		deps := newWorkspaceTestDeps(t, &stubClient{
			mutateSessionGoalFn: func(
				_ context.Context,
				id string,
				request contract.SessionGoalCommandRequest,
			) (contract.GoalCommandResult, error) {
				gotID = id
				gotRequest = request
				return contract.GoalCommandResult{Outcome: contract.GoalOutcomeReplaced}, nil
			},
		})

		stdout, _, err := executeRootCommand(
			t,
			deps,
			"session", "goal", "replace", "sess-1", "Ship the replacement",
			"--expected-run-id", "run-1",
			"--provider", "cursor",
			"--model", "grok-4.5",
			"--reasoning-effort", "high",
			"--speed", "fast",
			"-o", "json",
		)
		if err != nil {
			t.Fatalf("executeRootCommand(session goal replace) error = %v", err)
		}
		if gotID != "sess-1" || gotRequest.Operation != contract.SessionGoalOperationReplace ||
			gotRequest.Objective != "Ship the replacement" || gotRequest.ExpectedRunID != "run-1" ||
			gotRequest.Runtime == nil {
			t.Fatalf("MutateSessionGoal() = %q/%#v", gotID, gotRequest)
		}
		if gotRequest.Runtime.Provider != "cursor" || gotRequest.Runtime.Model != "grok-4.5" ||
			gotRequest.Runtime.ReasoningEffort != contract.ReasoningEffort("high") ||
			gotRequest.Runtime.Speed != contract.SpeedFast {
			t.Fatalf("MutateSessionGoal() runtime = %#v", gotRequest.Runtime)
		}
		var decoded contract.GoalCommandResult
		if err := json.Unmarshal([]byte(stdout), &decoded); err != nil {
			t.Fatalf("json.Unmarshal(session goal result) error = %v", err)
		}
		if decoded.Outcome != contract.GoalOutcomeReplaced {
			t.Fatalf("session goal output = %#v", decoded)
		}
	})
}

func TestSessionRewindForwardsTranscriptFences(t *testing.T) {
	t.Parallel()

	t.Run("Should forward the selected message identity and render the rewind result", func(t *testing.T) {
		t.Parallel()

		deps := newWorkspaceTestDeps(t, &stubClient{
			rewindSessionFn: func(
				_ context.Context,
				id string,
				request SessionRewindRequest,
			) (SessionRewindRecord, error) {
				if id != "sess-1" || request.MessageID != "msg-user-2" ||
					request.IdempotencyKey != "idem-rewind-1" || request.ExpectedEpoch == nil ||
					*request.ExpectedEpoch != 3 || request.ExpectedGeneration == nil ||
					*request.ExpectedGeneration != 8 || request.ExpectedMaxSequence == nil ||
					*request.ExpectedMaxSequence != 42 {
					t.Fatalf("RewindSession() id = %q request = %#v", id, request)
				}
				return SessionRewindRecord{
					Session: contract.SessionPayload{ID: id},
					Rewind: contract.SessionConversationRewindPayload{
						TranscriptEpoch: 4,
						TargetMessageID: "msg-user-2",
						ArchivedEvents:  7,
						Generation:      9,
						MaxSequence:     18,
						DraftText:       "Try a different direction",
					},
				}, nil
			},
		})

		stdout, _, err := executeRootCommand(
			t,
			deps,
			"session", "rewind", "sess-1",
			"--message-id", "msg-user-2",
			"--idempotency-key", "idem-rewind-1",
			"--expected-epoch", "3",
			"--expected-generation", "8",
			"--expected-max-sequence", "42",
			"-o", "json",
		)
		if err != nil {
			t.Fatalf("executeRootCommand(session rewind) error = %v", err)
		}
		var decoded SessionRewindRecord
		if err := json.Unmarshal([]byte(stdout), &decoded); err != nil {
			t.Fatalf("json.Unmarshal(session rewind) error = %v", err)
		}
		if decoded.Rewind.TargetMessageID != "msg-user-2" || decoded.Rewind.DraftText != "Try a different direction" {
			t.Fatalf("session rewind output = %#v", decoded)
		}
	})

	t.Run("Should read current transcript fences when no overrides are provided", func(t *testing.T) {
		t.Parallel()

		deps := newWorkspaceTestDeps(t, &stubClient{
			getSessionTranscriptFn: func(_ context.Context, id string) (SessionTranscriptRecord, error) {
				if id != "sess-1" {
					t.Fatalf("GetSessionTranscript() id = %q, want sess-1", id)
				}
				return SessionTranscriptRecord{Epoch: 5, Generation: 9, MaxSequence: 44}, nil
			},
			rewindSessionFn: func(
				_ context.Context,
				_ string,
				request SessionRewindRequest,
			) (SessionRewindRecord, error) {
				if request.ExpectedEpoch == nil || *request.ExpectedEpoch != 5 ||
					request.ExpectedGeneration == nil || *request.ExpectedGeneration != 9 ||
					request.ExpectedMaxSequence == nil || *request.ExpectedMaxSequence != 44 {
					t.Fatalf("RewindSession() request = %#v", request)
				}
				return SessionRewindRecord{
					Session: contract.SessionPayload{ID: "sess-1"},
					Rewind:  contract.SessionConversationRewindPayload{TargetMessageID: "msg-user-2"},
				}, nil
			},
		})

		_, _, err := executeRootCommand(
			t,
			deps,
			"session", "rewind", "sess-1",
			"--message-id", "msg-user-2",
			"--idempotency-key", "idem-auto-fence",
			"-o", "json",
		)
		if err != nil {
			t.Fatalf("executeRootCommand(session rewind auto fence) error = %v", err)
		}
	})
}

func TestSessionRuntimeCommandsPersistSelection(t *testing.T) {
	t.Parallel()

	t.Run("Should fetch the current revision before setting runtime", func(t *testing.T) {
		t.Parallel()

		deps := newWorkspaceTestDeps(t, &stubClient{
			getSessionFn: func(_ context.Context, id string) (SessionRecord, error) {
				return SessionRecord{ID: id, Runtime: contract.SessionRuntimePayload{SelectionRevision: 5}}, nil
			},
			setSessionRuntimeFn: func(
				_ context.Context,
				id string,
				request SetSessionRuntimeRequest,
			) (SessionRecord, error) {
				if id != "sess-1" || request.ExpectedRevision == nil || *request.ExpectedRevision != 5 ||
					request.Runtime.Provider != "claude" || request.Runtime.Model != "claude-fable-5" ||
					request.Runtime.ReasoningEffort != "max" || request.Runtime.Speed != contract.SpeedNormal {
					t.Fatalf("SetSessionRuntime() id = %q request = %#v", id, request)
				}
				return SessionRecord{
					ID: id,
					Runtime: contract.SessionRuntimePayload{
						Selected:          &request.Runtime,
						SelectionRevision: 6,
					},
				}, nil
			},
		})

		stdout, _, err := executeRootCommand(
			t,
			deps,
			"session", "runtime", "set", "sess-1",
			"--provider", "claude", "--model", "claude-fable-5", "--reasoning-effort", "max",
			"-o", "json",
		)
		if err != nil {
			t.Fatalf("executeRootCommand(session runtime set) error = %v", err)
		}
		var decoded SessionRecord
		if err := json.Unmarshal([]byte(stdout), &decoded); err != nil {
			t.Fatalf("json.Unmarshal() error = %v", err)
		}
		if decoded.Runtime.Selected == nil || decoded.Runtime.Selected.Model != "claude-fable-5" ||
			decoded.Runtime.SelectionRevision != 6 {
			t.Fatalf("decoded runtime = %#v, want selected Claude runtime at revision 6", decoded.Runtime)
		}
	})

	t.Run("Should pass an explicit revision when clearing runtime", func(t *testing.T) {
		t.Parallel()

		deps := newWorkspaceTestDeps(t, &stubClient{
			clearSessionRuntimeFn: func(_ context.Context, id string, revision int64) (SessionRecord, error) {
				if id != "sess-1" || revision != 8 {
					t.Fatalf("ClearSessionRuntime() id = %q revision = %d", id, revision)
				}
				return SessionRecord{ID: id, Runtime: contract.SessionRuntimePayload{SelectionRevision: 9}}, nil
			},
		})
		if _, _, err := executeRootCommand(
			t,
			deps,
			"session", "runtime", "clear", "sess-1", "--expected-revision", "8", "-o", "json",
		); err != nil {
			t.Fatalf("executeRootCommand(session runtime clear) error = %v", err)
		}
	})
}

func TestSessionPromptIdentity(t *testing.T) {
	t.Parallel()

	t.Run("Should forward an explicit durable identity pair", func(t *testing.T) {
		t.Parallel()

		var received SessionPromptRequest
		deps := newWorkspaceTestDeps(t, &stubClient{
			sendSessionPromptFn: func(_ context.Context, _ string, request SessionPromptRequest) (SessionPromptRecord, error) {
				received = request
				return SessionPromptRecord{Prompt: SessionPromptResultRecord{
					Status: "accepted", MessageID: request.MessageID, IdempotencyKey: request.IdempotencyKey,
				}}, nil
			},
		})

		stdout, _, err := executeRootCommand(t, deps,
			"session", "prompt", "sess-1", "retry safely",
			"--message-id", "msg-explicit", "--idempotency-key", "idem-explicit", "-o", "json",
		)
		if err != nil {
			t.Fatalf("executeRootCommand(session prompt explicit identity) error = %v", err)
		}
		if received.MessageID != "msg-explicit" || received.IdempotencyKey != "idem-explicit" {
			t.Fatalf(
				"SendSessionPrompt() identity = %q/%q, want msg-explicit/idem-explicit",
				received.MessageID,
				received.IdempotencyKey,
			)
		}
		var decoded SessionPromptRecord
		if err := json.Unmarshal([]byte(stdout), &decoded); err != nil {
			t.Fatalf("json.Unmarshal(explicit prompt response) error = %v", err)
		}
		if decoded.Prompt.MessageID != "msg-explicit" || decoded.Prompt.IdempotencyKey != "idem-explicit" {
			t.Fatalf(
				"prompt response identity = %q/%q, want msg-explicit/idem-explicit",
				decoded.Prompt.MessageID,
				decoded.Prompt.IdempotencyKey,
			)
		}
	})

	t.Run("Should reject incomplete or blank explicit identity pairs before submission", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			args []string
			want error
		}{
			{
				name: "Should reject only a message id",
				args: []string{"session", "prompt", "sess-1", "hello", "--message-id", "msg-only"},
				want: errPromptIdentityPairRequired,
			},
			{
				name: "Should reject only an idempotency key",
				args: []string{"session", "prompt", "sess-1", "hello", "--idempotency-key", "idem-only"},
				want: errPromptIdentityPairRequired,
			},
			{
				name: "Should reject blank identity values",
				args: []string{
					"session", "prompt", "sess-1", "hello",
					"--message-id", " ", "--idempotency-key", "idem",
				},
				want: errPromptIdentityBlank,
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				deps := newWorkspaceTestDeps(t, &stubClient{
					sendSessionPromptFn: func(context.Context, string, SessionPromptRequest) (SessionPromptRecord, error) {
						t.Fatal("SendSessionPrompt() called for invalid identity flags")
						return SessionPromptRecord{}, nil
					},
				})
				if _, _, err := executeRootCommand(
					t,
					deps,
					tt.args...); !errors.Is(err, tt.want) {
					t.Fatalf("executeRootCommand(%#v) error = %v, want identity validation error", tt.args, err)
				}
			})
		}
	})

	t.Run("Should preserve a replay admission envelope", func(t *testing.T) {
		t.Parallel()

		deps := newWorkspaceTestDeps(t, &stubClient{
			sendSessionPromptFn: func(_ context.Context, _ string, request SessionPromptRequest) (SessionPromptRecord, error) {
				return SessionPromptRecord{Prompt: SessionPromptResultRecord{
					Status: "queued", MessageID: request.MessageID, IdempotencyKey: request.IdempotencyKey,
					Replayed: true, Delivery: contract.PromptDeliveryAfterTurn, QueueEntryID: "queue-replayed",
				}}, nil
			},
		})

		stdout, _, err := executeRootCommand(t, deps,
			"session", "prompt", "sess-1", "retry safely",
			"--message-id", "msg-replay", "--idempotency-key", "idem-replay", "-o", "json",
		)
		if err != nil {
			t.Fatalf("executeRootCommand(session prompt replay) error = %v", err)
		}
		var decoded SessionPromptRecord
		if err := json.Unmarshal([]byte(stdout), &decoded); err != nil {
			t.Fatalf("json.Unmarshal(replay prompt response) error = %v", err)
		}
		if !decoded.Prompt.Replayed || decoded.Prompt.QueueEntryID != "queue-replayed" ||
			decoded.Prompt.MessageID != "msg-replay" || decoded.Prompt.IdempotencyKey != "idem-replay" {
			t.Fatalf("replay prompt response = %#v", decoded.Prompt)
		}
	})
}

func TestSessionPromptRejectsInvalidSpeed(t *testing.T) {
	t.Parallel()

	t.Run("Should reject invalid speed before sending a prompt", func(t *testing.T) {
		t.Parallel()

		deps := newTestDeps(t, &stubClient{
			sendSessionPromptFn: func(context.Context, string, SessionPromptRequest) (SessionPromptRecord, error) {
				t.Fatal("SendSessionPrompt() called with invalid speed")
				return SessionPromptRecord{}, nil
			},
		})
		if _, _, err := executeRootCommand(
			t,
			deps,
			"session",
			"prompt",
			"sess-1",
			"hello",
			"--speed",
			"turbo",
		); err == nil {
			t.Fatal("executeRootCommand(session prompt --speed turbo) error = nil")
		}
	})
}

func TestSessionNewWorkspaceOptions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		args    []string
		request CreateSessionRequest
	}{
		{
			name: "Should use a registered workspace",
			args: []string{"session", "new", "--workspace", "ws_abc", "-o", "json"},
			request: CreateSessionRequest{
				Workspace: "ws_abc",
			},
		},
		{
			name: "Should use an explicit cwd",
			args: []string{"session", "new", "--cwd", "/tmp/proj", "-o", "json"},
			request: CreateSessionRequest{
				WorkspacePath: "/tmp/proj",
			},
		},
		{
			name: "Should infer a registered cwd",
			args: []string{"session", "new", "-o", "json"},
			request: CreateSessionRequest{
				Workspace: "/workspace/project",
			},
		},
		{
			name: "Should start in an existing worktree",
			args: []string{"session", "new", "--workspace", "ws_abc", "--worktree", "wt-ready", "-o", "json"},
			request: CreateSessionRequest{
				Workspace: "ws_abc",
				Worktree:  "wt-ready",
			},
		},
		{
			name: "Should create a named worktree before starting",
			args: []string{"session", "new", "--workspace", "ws_abc", "--new-worktree=feature-session", "-o", "json"},
			request: CreateSessionRequest{
				Workspace:   "ws_abc",
				NewWorktree: &contract.NewSessionWorktreeRequest{Name: "feature-session"},
			},
		},
		{
			name: "Should request an automatically named worktree",
			args: []string{"session", "new", "--workspace", "ws_abc", "--new-worktree", "-o", "json"},
			request: CreateSessionRequest{
				Workspace:   "ws_abc",
				NewWorktree: &contract.NewSessionWorktreeRequest{},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client := &stubClient{
				createSessionFn: func(_ context.Context, request CreateSessionRequest) (SessionRecord, error) {
					if request.Workspace != tt.request.Workspace ||
						request.WorkspacePath != tt.request.WorkspacePath ||
						request.Worktree != tt.request.Worktree ||
						!reflect.DeepEqual(request.NewWorktree, tt.request.NewWorktree) {
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
			}
			if tt.name == "Should use an explicit cwd" {
				client.getWorkspaceFn = func(_ context.Context, ref string) (WorkspaceDetailRecord, error) {
					return WorkspaceDetailRecord{}, &workspacePathNotRegisteredError{path: ref}
				}
			}
			deps := newWorkspaceTestDeps(t, client)

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
			if tt.name == "Should use an explicit cwd" &&
				!strings.Contains(stdout, `"resolution_source": "cwd"`) {
				t.Fatalf("session new output = %s, want cwd resolution provenance", stdout)
			}
		})
	}

	t.Run("Should propagate non-fallback cwd resolution failures", func(t *testing.T) {
		t.Parallel()

		resolveErr := errors.New("workspace resolver unavailable")
		createCalls := 0
		deps := newWorkspaceTestDeps(t, &stubClient{
			getWorkspaceFn: func(context.Context, string) (WorkspaceDetailRecord, error) {
				return WorkspaceDetailRecord{}, resolveErr
			},
			createSessionFn: func(context.Context, CreateSessionRequest) (SessionRecord, error) {
				createCalls++
				return SessionRecord{ID: "unexpected"}, nil
			},
		})

		_, _, err := executeRootCommand(
			t,
			deps,
			"session",
			"new",
			"--cwd",
			"/workspace/project",
			"-o",
			"json",
		)
		if !errors.Is(err, resolveErr) {
			t.Fatalf("executeRootCommand(session new --cwd) error = %v, want resolver failure", err)
		}
		if createCalls != 0 {
			t.Fatalf("CreateSession() calls = %d, want 0 after resolver failure", createCalls)
		}
	})

	t.Run("Should reject mutually exclusive worktree selectors", func(t *testing.T) {
		t.Parallel()

		tests := [][]string{
			{"session", "new", "--cwd", "/tmp/proj", "--worktree", "wt-ready"},
			{"session", "new", "--cwd", "/tmp/proj", "--new-worktree"},
			{"session", "new", "--worktree", "wt-ready", "--new-worktree"},
		}
		for _, args := range tests {
			t.Run("Should reject "+strings.Join(args[2:], " "), func(t *testing.T) {
				t.Parallel()
				createCalls := 0
				deps := newWorkspaceTestDeps(t, &stubClient{
					createSessionFn: func(context.Context, CreateSessionRequest) (SessionRecord, error) {
						createCalls++
						return SessionRecord{}, nil
					},
				})
				_, _, err := executeRootCommand(t, deps, args...)
				if err == nil || !strings.Contains(err.Error(), "if any flags in the group") {
					t.Fatalf("executeRootCommand(%v) error = %v, want flag conflict", args, err)
				}
				if createCalls != 0 {
					t.Fatalf("CreateSession() calls = %d, want 0 for %v", createCalls, args)
				}
			})
		}
	})

	t.Run("Should defer a registered cross workspace cwd decision to the daemon", func(t *testing.T) {
		t.Parallel()

		client := &stubClient{
			createSessionFn: func(_ context.Context, request CreateSessionRequest) (SessionRecord, error) {
				if request.Workspace != "ws-foreign" {
					t.Fatalf("CreateSession() workspace = %q, want ws-foreign", request.Workspace)
				}
				return SessionRecord{
					ID:          "sess-foreign",
					AgentName:   "coder",
					WorkspaceID: request.Workspace,
					State:       session.StateActive,
				}, nil
			},
			getSessionFn: func(_ context.Context, id string) (SessionRecord, error) {
				return SessionRecord{
					ID:          id,
					ProfileID:   store.DefaultProfileID,
					AgentName:   "coder",
					WorkspaceID: "ws-session",
					State:       session.StateActive,
				}, nil
			},
			getWorkspaceFn: func(_ context.Context, ref string) (WorkspaceDetailRecord, error) {
				return WorkspaceDetailRecord{
					Workspace: WorkspaceRecord{ID: "ws-foreign", RootDir: ref},
				}, nil
			},
		}
		deps := newWorkspaceTestDeps(t, client)
		deps.getenv = func(key string) string {
			switch key {
			case agentidentity.EnvSessionID:
				return "sess-agent"
			case agentidentity.EnvAgent:
				return "coder"
			default:
				return ""
			}
		}

		stdout, _, err := executeRootCommand(
			t,
			deps,
			"session",
			"new",
			"--cwd",
			"/workspace/foreign",
			"-o",
			"json",
		)
		if err != nil {
			t.Fatalf("executeRootCommand(session new --cwd foreign) error = %v", err)
		}
		var output struct {
			ResolutionSource string `json:"resolution_source"`
		}
		if err := json.Unmarshal([]byte(stdout), &output); err != nil {
			t.Fatalf("json.Unmarshal(session new output) error = %v; output=%s", err, stdout)
		}
		if output.ResolutionSource != "cross_workspace_attempt" {
			t.Fatalf("session new resolution source = %q, want cross_workspace_attempt", output.ResolutionSource)
		}
	})
}

func TestSessionNewPassesNetworkParticipationFlags(t *testing.T) {
	t.Parallel()

	deps := newWorkspaceTestDeps(t, &stubClient{
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

		stdout, _, err := executeRootCommand(t, newWorkspaceTestDeps(t, &stubClient{}), "session", "new", "--help")
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

			deps := newWorkspaceTestDeps(t, &stubClient{
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

func TestSessionNewRejectsRemovedFlags(t *testing.T) {
	t.Parallel()

	tests := []struct {
		flag  string
		value string
	}{
		{flag: "--provider", value: "fake-alt"},
		{flag: "--model", value: "gpt-5.6-sol"},
		{flag: "--prompt", value: "legacy prompt"},
		{flag: "--speed", value: "fast"},
		{flag: "--reasoning-effort", value: "high"},
		{flag: "--no-wait"},
	}
	for _, tt := range tests {
		t.Run("Should reject "+tt.flag, func(t *testing.T) {
			t.Parallel()

			deps := newWorkspaceTestDeps(t, &stubClient{
				createSessionFn: func(context.Context, CreateSessionRequest) (SessionRecord, error) {
					t.Fatal("CreateSession() called with a removed flag")
					return SessionRecord{}, nil
				},
			})
			args := []string{"session", "new", tt.flag}
			if tt.value != "" {
				args = append(args, tt.value)
			}
			_, _, err := executeRootCommand(t, deps, args...)
			if err == nil || !strings.Contains(err.Error(), "unknown flag") {
				t.Fatalf("executeRootCommand(%s) error = %v, want unknown flag", tt.flag, err)
			}
		})
	}
}

func TestSessionNewRejectsInvalidWorkspaceFlags(t *testing.T) {
	t.Parallel()

	code, _, stderr := executeRootCommandWithExit(t, newWorkspaceTestDeps(t, &stubClient{}),
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

			code, _, stderr := executeRootCommandWithExit(t, newWorkspaceTestDeps(t, &stubClient{}),
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
	t.Run("Should pass every server-owned session page filter", func(t *testing.T) {
		t.Parallel()

		var seenQuery SessionListQuery

		deps := newWorkspaceTestDeps(t, &stubClient{
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
			"--worktree",
			"wt-filtered",
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
			"--archived",
			"-o",
			"json",
		)
		if err != nil {
			t.Fatalf("executeRootCommand(session list) error = %v", err)
		}
		if seenQuery.Workspace != "ws-filtered" || seenQuery.Worktree != "wt-filtered" ||
			seenQuery.State != "stopped" || seenQuery.Type != "user" ||
			seenQuery.Agent != "general" || seenQuery.Query != "demo" ||
			seenQuery.Archive != string(session.ArchiveOnly) ||
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
	})
}

func TestSessionArchiveCommandsReturnUpdatedSession(t *testing.T) {
	t.Run("Should return the updated session from archive commands", func(t *testing.T) {
		t.Parallel()

		archivedAt := fixedTestNow.Add(time.Minute)
		deps := newWorkspaceTestDeps(t, &stubClient{
			archiveSessionFn: func(_ context.Context, id string) (SessionRecord, error) {
				return SessionRecord{
					ID: id, AgentName: "coder", WorkspaceID: "ws-1", State: session.StateStopped,
					ArchivedAt: &archivedAt, CreatedAt: fixedTestNow, UpdatedAt: archivedAt,
				}, nil
			},
			unarchiveSessionFn: func(_ context.Context, id string) (SessionRecord, error) {
				return SessionRecord{
					ID: id, AgentName: "coder", WorkspaceID: "ws-1", State: session.StateStopped,
					CreatedAt: fixedTestNow, UpdatedAt: archivedAt,
				}, nil
			},
		})

		archiveOutput, _, err := executeRootCommand(t, deps, "session", "archive", "sess-1", "-o", "json")
		if err != nil {
			t.Fatalf("executeRootCommand(session archive) error = %v", err)
		}
		var archived SessionRecord
		if err := json.Unmarshal([]byte(archiveOutput), &archived); err != nil {
			t.Fatalf("json.Unmarshal(session archive) error = %v", err)
		}
		if archived.ArchivedAt == nil || archived.ID != "sess-1" {
			t.Fatalf("session archive output = %#v, want archive marker", archived)
		}

		unarchiveOutput, _, err := executeRootCommand(t, deps, "session", "unarchive", "sess-1", "-o", "json")
		if err != nil {
			t.Fatalf("executeRootCommand(session unarchive) error = %v", err)
		}
		var restored SessionRecord
		if err := json.Unmarshal([]byte(unarchiveOutput), &restored); err != nil {
			t.Fatalf("json.Unmarshal(session unarchive) error = %v", err)
		}
		if restored.ArchivedAt != nil || restored.ID != "sess-1" {
			t.Fatalf("session unarchive output = %#v, want restored session", restored)
		}
	})
}

func TestSessionRenameCommandReturnsUpdatedSession(t *testing.T) {
	t.Parallel()

	t.Run("Should rename a session and return the updated record", func(t *testing.T) {
		t.Parallel()

		deps := newWorkspaceTestDeps(t, &stubClient{
			renameSessionFn: func(
				_ context.Context,
				id string,
				request RenameSessionRequest,
			) (SessionRecord, error) {
				if id != "sess-1" || request.Name != "Checkout retries" {
					t.Fatalf("RenameSession() = id %q request %#v", id, request)
				}
				return SessionRecord{
					ID:          id,
					Name:        request.Name,
					AgentName:   "coder",
					WorkspaceID: "ws-1",
					State:       session.StateStopped,
					CreatedAt:   fixedTestNow,
					UpdatedAt:   fixedTestNow.Add(time.Minute),
				}, nil
			},
		})

		stdout, _, err := executeRootCommand(
			t,
			deps,
			"session",
			"rename",
			"sess-1",
			"Checkout retries",
			"-o",
			"json",
		)
		if err != nil {
			t.Fatalf("executeRootCommand(session rename) error = %v", err)
		}
		var renamed SessionRecord
		if err := json.Unmarshal([]byte(stdout), &renamed); err != nil {
			t.Fatalf("json.Unmarshal(session rename) error = %v", err)
		}
		if renamed.ID != "sess-1" || renamed.Name != "Checkout retries" {
			t.Fatalf("session rename output = %#v", renamed)
		}
	})
}

func TestSessionListRejectsConflictingArchiveFilters(t *testing.T) {
	t.Run("Should reject conflicting archive filters", func(t *testing.T) {
		t.Parallel()

		_, _, err := executeRootCommand(
			t,
			newWorkspaceTestDeps(t, &stubClient{}),
			"session",
			"list",
			"--archived",
			"--include-archived",
		)
		if err == nil || !strings.Contains(err.Error(), "mutually exclusive") {
			t.Fatalf("session list conflicting archive filters error = %v, want mutually exclusive", err)
		}
	})
}

func TestSessionListDefaultsToExactActiveState(t *testing.T) {
	t.Parallel()

	var seenQuery SessionListQuery
	deps := newWorkspaceTestDeps(t, &stubClient{
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

func TestSessionListAttentionFiltersAndSummary(t *testing.T) {
	t.Parallel()

	t.Run("Should pass needs-you filtering with attention-first ordering", func(t *testing.T) {
		t.Parallel()

		var seenQuery SessionListQuery
		deps := newWorkspaceTestDeps(t, &stubClient{
			listSessionPageFn: func(_ context.Context, query SessionListQuery) (SessionListPage, error) {
				seenQuery = query
				return SessionListPage{Page: contract.CountedCursorPagePayload{Limit: session.DefaultListLimit}}, nil
			},
		})
		if _, _, err := executeRootCommand(t, deps, "session", "list", "--attention", "-o", "json"); err != nil {
			t.Fatalf("executeRootCommand(session list --attention) error = %v", err)
		}
		if !seenQuery.Attention || seenQuery.Sort != session.ListSortAttention || seenQuery.State != "" {
			t.Fatalf("ListSessions() query = %#v, want attention filter without implicit state", seenQuery)
		}
	})

	t.Run("Should pass exact badges across every workspace", func(t *testing.T) {
		t.Parallel()

		var seenQuery SessionListQuery
		deps := newWorkspaceTestDeps(t, &stubClient{
			listSessionPageFn: func(_ context.Context, query SessionListQuery) (SessionListPage, error) {
				seenQuery = query
				return SessionListPage{Page: contract.CountedCursorPagePayload{Limit: session.DefaultListLimit}}, nil
			},
			getWorkspaceFn: func(context.Context, string) (WorkspaceDetailRecord, error) {
				t.Fatal("GetWorkspace() called for --all-workspaces")
				return WorkspaceDetailRecord{}, nil
			},
		})
		deps.getenv = func(key string) string {
			if key == workspaceEnvName {
				return "workspace-from-environment"
			}
			return ""
		}
		if _, _, err := executeRootCommand(
			t,
			deps,
			"session", "list", "--badge", "done", "--all-workspaces", "-o", "json",
		); err != nil {
			t.Fatalf("executeRootCommand(session list --badge done) error = %v", err)
		}
		if seenQuery.Workspace != "" || seenQuery.Sort != session.ListSortAttention ||
			!slices.Equal(seenQuery.Badges, []session.Badge{session.BadgeDone}) {
			t.Fatalf("ListSessions() query = %#v, want exact cross-workspace done filter", seenQuery)
		}
	})

	t.Run("Should reject an unknown badge with usage exit code", func(t *testing.T) {
		t.Parallel()

		_, _, err := executeRootCommand(
			t,
			newWorkspaceTestDeps(t, &stubClient{}),
			"session", "list", "--badge", "sleeping",
		)
		if err == nil || cliExitCodeForError(err) != 65 {
			t.Fatalf("session list invalid badge error = %v, exit = %d, want exit 65", err, cliExitCodeForError(err))
		}
		if !strings.Contains(err.Error(), string(session.BadgeWaitingForInput)) ||
			!strings.Contains(err.Error(), string(session.BadgeDone)) {
			t.Fatalf("session list invalid badge error = %q, want canonical vocabulary", err)
		}
	})

	t.Run("Should render the exact operator-wide summary", func(t *testing.T) {
		t.Parallel()

		want := SessionAttentionSummaryRecord{
			NeedsYou: 7,
			Finished: 3,
			ByWorkspace: []contract.WorkspaceAttentionSummaryPayload{
				{WorkspaceID: "ws-a", NeedsYou: 4, Finished: 1},
				{WorkspaceID: "ws-b", NeedsYou: 3, Finished: 2},
			},
		}
		deps := newWorkspaceTestDeps(t, &stubClient{
			getSessionAttentionSummaryFn: func(context.Context) (SessionAttentionSummaryRecord, error) {
				return want, nil
			},
		})
		stdout, _, err := executeRootCommand(t, deps, "session", "list", "--summary", "-o", "json")
		if err != nil {
			t.Fatalf("executeRootCommand(session list --summary) error = %v", err)
		}
		var got SessionAttentionSummaryRecord
		if err := json.Unmarshal([]byte(stdout), &got); err != nil {
			t.Fatalf("json.Unmarshal(session attention summary) error = %v", err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("session attention summary = %#v, want %#v", got, want)
		}
	})
}

func TestSessionInteractionsRendersSanitizedProjection(t *testing.T) {
	t.Parallel()

	t.Run("Should request and render the canonical pending interactions", func(t *testing.T) {
		t.Parallel()

		createdAt := fixedTestNow.Add(-time.Minute)
		want := SessionInteractionsRecord{Interactions: []contract.PendingInteractionPayload{
			{
				InteractionID: "interaction-1", Kind: "clarify", ProviderRequestID: "request-1",
				Status: "pending", Title: "Choose a deployment", CreatedAt: createdAt,
			},
		}}
		deps := newWorkspaceTestDeps(t, &stubClient{
			listSessionInteractionsFn: func(_ context.Context, id string, statuses []string) (SessionInteractionsRecord, error) {
				if id != "sess-1" || len(statuses) != 0 {
					t.Fatalf("ListSessionInteractions() id = %q statuses = %#v", id, statuses)
				}
				return want, nil
			},
		})
		stdout, _, err := executeRootCommand(t, deps, "session", "interactions", "sess-1", "-o", "json")
		if err != nil {
			t.Fatalf("executeRootCommand(session interactions) error = %v", err)
		}
		var got SessionInteractionsRecord
		if err := json.Unmarshal([]byte(stdout), &got); err != nil {
			t.Fatalf("json.Unmarshal(session interactions) error = %v", err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("session interactions = %#v, want %#v", got, want)
		}

		bundle := sessionInteractionsBundle(want)
		human, humanErr := bundle.human()
		if humanErr != nil || !strings.Contains(human, "interaction-1") ||
			!strings.Contains(human, "Choose a deployment") {
			t.Fatalf("session interactions human = %q, error = %v", human, humanErr)
		}
		toon, toonErr := bundle.toon()
		if toonErr != nil || !strings.Contains(toon, "interaction-1") ||
			!strings.Contains(toon, "request-1") {
			t.Fatalf("session interactions toon = %q, error = %v", toon, toonErr)
		}
	})
}

func TestSessionListResolvesOptionalWorkspaceOverride(t *testing.T) {
	t.Parallel()

	t.Run("Should resolve COMPOZY_WORKSPACE before listing sessions", func(t *testing.T) {
		t.Parallel()

		var seenQuery SessionListQuery
		deps := newWorkspaceTestDeps(t, &stubClient{
			getWorkspaceFn: func(_ context.Context, ref string) (WorkspaceDetailRecord, error) {
				if ref != "project-alias" {
					t.Fatalf("GetWorkspace() ref = %q, want project-alias", ref)
				}
				return WorkspaceDetailRecord{
					Workspace: WorkspaceRecord{ID: "ws-project", RootDir: "/workspace/project"},
				}, nil
			},
			listSessionPageFn: func(_ context.Context, query SessionListQuery) (SessionListPage, error) {
				seenQuery = query
				return SessionListPage{}, nil
			},
		})
		deps.getenv = func(key string) string {
			if key == workspaceEnvName {
				return "project-alias"
			}
			return ""
		}

		if _, _, err := executeRootCommand(t, deps, "session", "list", "-o", "json"); err != nil {
			t.Fatalf("executeRootCommand(session list) error = %v", err)
		}
		if seenQuery.Workspace != "ws-project" {
			t.Fatalf("seenQuery.Workspace = %q, want ws-project", seenQuery.Workspace)
		}
	})
}

func TestSessionListJSONLIncludesContinuationRecord(t *testing.T) {
	t.Parallel()

	deps := newWorkspaceTestDeps(t, &stubClient{
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

func TestSessionListProfileReadScope(t *testing.T) {
	t.Parallel()

	t.Run("Should emit aggregate resolution before owner-labeled rows [UT-077]", func(t *testing.T) {
		t.Parallel()
		workspaceClient := &stubClient{
			getWorkspaceFn: func(context.Context, string) (WorkspaceDetailRecord, error) {
				return WorkspaceDetailRecord{Workspace: WorkspaceRecord{
					ID: "ws-1", Name: "my-saas", RootDir: "/workspace",
				}}, nil
			},
			listSessionPageFn: func(context.Context, SessionListQuery) (SessionListPage, error) {
				return SessionListPage{Sessions: []SessionRecord{{
					ID: "sess-marketing", ProfileName: "marketing", State: session.StateActive,
				}}}, nil
			},
		}
		client := &profileTestDaemonClient{
			DaemonClient: workspaceClient,
			profileClientAPI: &profileClientStub{profiles: []contract.Profile{{
				Name: "default", State: "active",
			}}},
		}
		deps := newTestDeps(t, client)
		deps.getwd = func() (string, error) { return "/workspace", nil }
		deps.getenv = func(string) string { return "" }

		stdout, _, err := executeRootCommand(
			t, deps, "session", "list", "--workspace", "my-saas", "--all-profiles", "-o", "jsonl",
		)
		if err != nil {
			t.Fatalf("executeRootCommand(session list --all-profiles) error = %v", err)
		}
		lines := strings.Split(strings.TrimSpace(stdout), "\n")
		if len(lines) != 3 {
			t.Fatalf("aggregate JSONL lines = %d, want frame + row + page; output=%q", len(lines), stdout)
		}
		if lines[0] != `{"kind":"workspace_resolution","workspace":"my-saas","source":"flag"}` {
			t.Fatalf("aggregate resolution frame = %s", lines[0])
		}
		var row map[string]any
		if err := json.Unmarshal([]byte(lines[1]), &row); err != nil {
			t.Fatalf("json.Unmarshal(aggregate row) error = %v", err)
		}
		if row["profile_name"] != "marketing" {
			t.Fatalf("aggregate row profile_name = %#v, want marketing", row["profile_name"])
		}
	})

	t.Run("Should reject profile and aggregate selection together [UT-024]", func(t *testing.T) {
		t.Parallel()
		deps := profileTestDeps(t)
		_, _, err := executeRootCommand(
			t, deps, "session", "list", "--profile", "marketing", "--all-profiles", "-o", "json",
		)
		var profileErr *profileCommandError
		if !errors.As(err, &profileErr) || profileErr.payload.Error.Code != "profile_selection_conflict" {
			t.Fatalf("aggregate conflict error = %v, want profile_selection_conflict", err)
		}
	})
}

func TestSessionRepairPassesFlagsAndRendersJSON(t *testing.T) {
	t.Run("Should pass flags and render JSON", func(t *testing.T) {
		t.Parallel()

		var seenQuery SessionRepairQuery
		var seenID string
		deps := newWorkspaceTestDeps(t, &stubClient{
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
	deps := newWorkspaceTestDeps(t, &stubClient{
		listSessionClarificationsFn: func(_ context.Context, sessionID string) (ClarificationsRecord, error) {
			if sessionID != "sess-1" {
				t.Fatalf("session id = %q, want sess-1", sessionID)
			}
			return want, nil
		},
	})

	stdout, _, err := executeRootCommand(
		t,
		deps,
		"session",
		sessionClarifyCommandUse,
		"pending",
		"sess-1",
		"-o",
		"json",
	)
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
	deps := newWorkspaceTestDeps(t, &stubClient{
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
			return ClarificationAnswerRecord{
				ClarifyAnswer: toolspkg.ClarifyAnswer{Choice: &wireChoice},
				Outcome:       "answered",
				RequestID:     requestID,
			}, nil
		},
	})

	stdout, _, err := executeRootCommand(
		t,
		deps,
		"session",
		sessionClarifyCommandUse,
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
	if got.Choice == nil || *got.Choice != wireChoice || got.Text != "" || got.Fallback ||
		got.Outcome != "answered" || got.RequestID != "req-1" {
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
			args := []string{"session", sessionClarifyCommandUse, "answer", "sess-1", "req-1"}
			args = append(args, tt.args...)
			_, _, err := executeRootCommand(t, newWorkspaceTestDeps(t, &stubClient{}), args...)
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

	deps := newWorkspaceTestDeps(t, &stubClient{
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

		deps := newWorkspaceTestDeps(t, &stubClient{
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

		deps := newWorkspaceTestDeps(t, &stubClient{
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

func TestSessionCommandsRendersUnifiedCatalog(t *testing.T) {
	t.Parallel()
	t.Run("Should render the unified command catalog", func(t *testing.T) {
		t.Parallel()

		deps := newWorkspaceTestDeps(t, &stubClient{
			listSessionCommandsFn: func(_ context.Context, id string) (SessionCommandsRecord, error) {
				if id != "sess-1" {
					t.Fatalf("ListSessionCommands() id = %q, want sess-1", id)
				}
				return SessionCommandsRecord{
					Revision: "revision-1",
					Commands: []contract.SessionCommandPayload{{
						ID: "skill:extension:ops:review", CanonicalToken: "/ops:review",
						DisplayName: "/review", Description: "Review carefully", Lane: "skill",
						Source: contract.SessionCommandSource{
							Kind: "extension", ID: "ops", Key: "ops", Scope: "workspace",
						},
						Placements: []string{"standalone", "inline"},
					}},
				}, nil
			},
		})
		stdout, _, err := executeRootCommand(t, deps, "session", "commands", "sess-1", "-o", "json")
		if err != nil {
			t.Fatalf("executeRootCommand(session commands) error = %v", err)
		}
		var decoded SessionCommandsRecord
		if err := json.Unmarshal([]byte(stdout), &decoded); err != nil {
			t.Fatalf("json.Unmarshal(session commands) error = %v", err)
		}
		if decoded.Revision != "revision-1" || len(decoded.Commands) != 1 ||
			decoded.Commands[0].CanonicalToken != "/ops:review" ||
			decoded.Commands[0].Source.Key != "ops" {
			t.Fatalf("decoded commands = %#v", decoded)
		}
	})
}

func TestSessionWaitReturnsMatchingState(t *testing.T) {
	t.Parallel()
	t.Run("Should pass normalized predicates and render the reached state", func(t *testing.T) {
		t.Parallel()

		var captured SessionWaitRequest
		deps := newWorkspaceTestDeps(t, &stubClient{
			getSessionFn: func(context.Context, string) (SessionRecord, error) {
				return SessionRecord{
					ID:              "sess-1",
					AgentName:       "coder",
					WorkspaceID:     "ws-1",
					WorkspacePath:   "/workspace/project",
					State:           session.StateActive,
					TranscriptEpoch: 8,
					CreatedAt:       fixedTestNow,
					UpdatedAt:       fixedTestNow,
				}, nil
			},
			waitSessionFn: func(
				_ context.Context,
				workspaceID string,
				sessionID string,
				request SessionWaitRequest,
			) (SessionWaitRecord, error) {
				if workspaceID != "ws-1" || sessionID != "sess-1" {
					t.Fatalf("WaitSession() scope = %q/%q, want ws-1/sess-1", workspaceID, sessionID)
				}
				captured = request
				return SessionWaitRecord{
					SessionID: sessionID,
					Outcome:   "state-reached",
					State:     "idle",
					WaitedMS:  38_000,
					Until:     []string{"waiting-for-input", "idle"},
					Revision:  11,
				}, nil
			},
		})

		stdout, _, err := executeRootCommand(
			t,
			deps,
			"session",
			"wait",
			"sess-1",
			"--until",
			"waiting-for-input,idle,idle",
			"--timeout",
			"120s",
			"-o",
			"json",
		)
		if err != nil {
			t.Fatalf("executeRootCommand() error = %v", err)
		}
		if captured.TimeoutMS != 120_000 || captured.Epoch != 8 || len(captured.Until) != 2 ||
			captured.Until[0] != "waiting-for-input" || captured.Until[1] != "idle" {
			t.Fatalf("WaitSession() request = %#v, want normalized flags and epoch pin", captured)
		}
		var decoded SessionWaitRecord
		if err := json.Unmarshal([]byte(stdout), &decoded); err != nil {
			t.Fatalf("json.Unmarshal() error = %v", err)
		}
		if decoded.Outcome != "state-reached" || decoded.State != "idle" || decoded.WaitedMS != 38_000 ||
			decoded.Revision != 11 {
			t.Fatalf("decoded wait = %#v, want reached idle with revision fence", decoded)
		}
	})
}

func TestSessionWaitReturnsTimeoutExitAndPayload(t *testing.T) {
	t.Parallel()
	t.Run("Should return exit 75 with a structured timeout outcome", func(t *testing.T) {
		t.Parallel()

		deps := newWorkspaceTestDeps(t, &stubClient{
			getSessionFn: func(context.Context, string) (SessionRecord, error) {
				return SessionRecord{
					ID:            "sess-1",
					AgentName:     "coder",
					WorkspaceID:   "ws-1",
					WorkspacePath: "/workspace/project",
					State:         session.StateActive,
					CreatedAt:     fixedTestNow,
					UpdatedAt:     fixedTestNow,
				}, nil
			},
			waitSessionFn: func(
				_ context.Context,
				_ string,
				sessionID string,
				request SessionWaitRequest,
			) (SessionWaitRecord, error) {
				return SessionWaitRecord{
					SessionID: sessionID,
					Outcome:   "timeout",
					WaitedMS:  request.TimeoutMS,
					Until:     request.Until,
					ResumeID:  "wait-1",
				}, nil
			},
		})

		stdout, _, err := executeRootCommand(
			t,
			deps,
			"session",
			"wait",
			"sess-1",
			"--until",
			"idle",
			"--timeout",
			"5s",
			"-o",
			"json",
		)
		if err == nil || cliExitCodeForError(err) != 75 {
			t.Fatalf("session wait timeout error = %v, exit = %d, want 75", err, cliExitCodeForError(err))
		}
		var decoded SessionWaitRecord
		if err := json.Unmarshal([]byte(stdout), &decoded); err != nil {
			t.Fatalf("json.Unmarshal() error = %v", err)
		}
		if decoded.Outcome != "timeout" || decoded.WaitedMS != 5_000 || decoded.ResumeID != "wait-1" ||
			len(decoded.Until) != 1 || decoded.Until[0] != "idle" {
			t.Fatalf("decoded wait timeout = %#v, want explicit timeout outcome", decoded)
		}
	})
}

func TestSessionWaitUnboundedResumesGaplesslyAndRecoversExpiredRegistration(t *testing.T) {
	t.Parallel()
	t.Run("Should resume gaplessly and replace an expired registration", func(t *testing.T) {
		t.Parallel()

		calls := 0
		deps := newWorkspaceTestDeps(t, &stubClient{
			getSessionFn: func(context.Context, string) (SessionRecord, error) {
				return SessionRecord{ID: "sess-1", WorkspaceID: "ws-1", TranscriptEpoch: 4}, nil
			},
			waitSessionFn: func(
				_ context.Context,
				_ string,
				sessionID string,
				request SessionWaitRequest,
			) (SessionWaitRecord, error) {
				calls++
				switch calls {
				case 1:
					if request.ResumeID != "" || request.TimeoutMS != 300_000 {
						t.Fatalf("first wait request = %#v, want fresh bounded wait", request)
					}
					return SessionWaitRecord{
						SessionID: sessionID, Outcome: "timeout", WaitedMS: 300_000, ResumeID: "wait-1",
					}, nil
				case 2:
					if request.ResumeID != "wait-1" {
						t.Fatalf("second wait resume_id = %q, want wait-1", request.ResumeID)
					}
					return SessionWaitRecord{}, &daemonAPIError{
						statusCode: http.StatusGone,
						status:     http.StatusText(http.StatusGone),
						payload:    contract.ErrorPayload{Error: "wait expired", Code: "wait_expired"},
					}
				case 3:
					if request.ResumeID != "" {
						t.Fatalf("third wait resume_id = %q, want fresh registration", request.ResumeID)
					}
					return SessionWaitRecord{
						SessionID: sessionID, Outcome: "state-reached", State: "idle", WaitedMS: 1_000,
					}, nil
				default:
					t.Fatalf("WaitSession() calls = %d, want 3", calls)
					return SessionWaitRecord{}, nil
				}
			},
		})

		stdout, _, err := executeRootCommand(
			t,
			deps,
			"session",
			"wait",
			"sess-1",
			"--unbounded",
			"-o",
			"json",
		)
		if err != nil {
			t.Fatalf("executeRootCommand() error = %v", err)
		}
		if calls != 3 {
			t.Fatalf("WaitSession() calls = %d, want 3", calls)
		}
		var decoded SessionWaitRecord
		if err := json.Unmarshal([]byte(stdout), &decoded); err != nil {
			t.Fatalf("json.Unmarshal() error = %v", err)
		}
		if decoded.Outcome != "state-reached" || decoded.State != "idle" || decoded.WaitedMS != 301_000 {
			t.Fatalf("decoded unbounded wait = %#v, want accumulated state outcome", decoded)
		}
	})
}

func TestSessionWaitRejectsUnknownStateBeforeCallingDaemon(t *testing.T) {
	t.Parallel()
	t.Run("Should reject an unknown state before contacting the daemon", func(t *testing.T) {
		t.Parallel()

		deps := newWorkspaceTestDeps(t, &stubClient{
			getSessionFn: func(context.Context, string) (SessionRecord, error) {
				t.Fatal("GetSession() called for invalid wait state")
				return SessionRecord{}, nil
			},
			waitSessionFn: func(context.Context, string, string, SessionWaitRequest) (SessionWaitRecord, error) {
				t.Fatal("WaitSession() called for invalid wait state")
				return SessionWaitRecord{}, nil
			},
		})

		_, _, err := executeRootCommand(t, deps, "session", "wait", "sess-1", "--until", "sleeping")
		if err == nil || cliExitCodeForError(err) != 65 || !strings.Contains(err.Error(), "unknown session state") {
			t.Fatalf("invalid wait error = %v, exit = %d, want vocabulary error/65", err, cliExitCodeForError(err))
		}
	})
}

// Invariant: stopping a session preserves the selected profile through the
// mutation and follow-up read. The canonical CLI session command suite owns
// this lifecycle boundary.
func TestSessionStopFetchesUpdatedSession(t *testing.T) {
	t.Parallel()

	t.Run("Should fetch the updated session through the selected profile", func(t *testing.T) {
		t.Parallel()

		var stoppedID string

		client := &profileTestDaemonClient{
			DaemonClient: withWorkspaceResolution(&stubClient{
				stopSessionFn: func(ctx context.Context, id string) error {
					if got := profileQueryValues(ctx, nil).Get(profileFlagName); got != "research" {
						t.Fatalf("StopSession() profile query = %q, want research", got)
					}
					stoppedID = id
					return nil
				},
				getSessionFn: func(ctx context.Context, id string) (SessionRecord, error) {
					if got := profileQueryValues(ctx, nil).Get(profileFlagName); got != "research" {
						t.Fatalf("GetSession() profile query = %q, want research", got)
					}
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
			}),
			profileClientAPI: &profileClientStub{profiles: []contract.Profile{
				{Name: "default", State: "active"},
				{Name: "research", State: "active"},
			}},
		}
		deps := newWorkspaceTestDeps(t, client)

		stdout, _, err := executeRootCommand(
			t,
			deps,
			"--profile",
			"research",
			"session",
			"stop",
			"sess-1",
			"-o",
			"json",
		)
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
	})
}

func TestSessionRemoveDeletesSession(t *testing.T) {
	t.Parallel()

	t.Run("Should delete session and return session record", func(t *testing.T) {
		t.Parallel()

		var deletedID string

		deps := newWorkspaceTestDeps(t, &stubClient{
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

	deps := newWorkspaceTestDeps(t, &stubClient{
		getSessionStatusFn: func(_ context.Context, id string) (SessionStatusRecord, error) {
			if id != "sess-1" {
				t.Fatalf("GetSessionStatus() id = %q, want sess-1", id)
			}
			return SessionStatusRecord{
				SessionID:       id,
				AgentName:       "coder",
				WorkspaceID:     "ws-1",
				State:           "idle",
				Badge:           session.BadgeWaitingForInput,
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
	if decoded.SessionID != "sess-1" || decoded.State != "idle" ||
		decoded.Badge != session.BadgeWaitingForInput || !decoded.EligibleForWake {
		t.Fatalf("decoded = %#v, want sess-1 eligible idle health", decoded)
	}

	human, _, err := executeRootCommand(t, deps, "session", "status", "sess-1")
	if err != nil {
		t.Fatalf("executeRootCommand(human) error = %v", err)
	}
	if !strings.Contains(human, string(session.BadgeWaitingForInput)) {
		t.Fatalf("human status = %q, want badge %q", human, session.BadgeWaitingForInput)
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
		deps := newWorkspaceTestDeps(t, &stubClient{
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
		deps := newWorkspaceTestDeps(t, &stubClient{
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
		deps := newWorkspaceTestDeps(t, &stubClient{
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

	deps := newWorkspaceTestDeps(t, &stubClient{
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

func TestSessionAttachmentsUploadRendersAttachmentPayload(t *testing.T) {
	t.Parallel()

	t.Run("Should print the direct attachment payload and forward arguments", func(t *testing.T) {
		t.Parallel()

		filePath := filepath.Join(t.TempDir(), "frame.png")
		if err := os.WriteFile(filePath, []byte("attachment"), 0o600); err != nil {
			t.Fatalf("os.WriteFile() error = %v", err)
		}
		var (
			gotSessionID string
			gotFilePath  string
		)
		deps := newWorkspaceTestDeps(t, &stubClient{
			uploadSessionAttachmentFn: func(_ context.Context, sessionID string, path string) (SessionAttachmentRecord, error) {
				gotSessionID = sessionID
				gotFilePath = path
				return SessionAttachmentRecord{
					ID:        "att-1",
					Name:      "frame.png",
					MIMEType:  "image/png",
					Bytes:     10,
					SHA256:    "sha256:abc",
					Kind:      contract.PromptAttachmentKindImage,
					Width:     4,
					Height:    3,
					CreatedAt: fixedTestNow,
				}, nil
			},
		})

		stdout, _, err := executeRootCommand(
			t,
			deps,
			"session", "attachments", "upload", "sess-1", filePath, "-o", "json",
		)
		if err != nil {
			t.Fatalf("executeRootCommand(session attachments upload) error = %v", err)
		}
		if gotSessionID != "sess-1" || gotFilePath != filePath {
			t.Fatalf(
				"UploadSessionAttachment() = (%q, %q), want (%q, %q)",
				gotSessionID,
				gotFilePath,
				"sess-1",
				filePath,
			)
		}

		var decoded SessionAttachmentRecord
		if err := json.Unmarshal([]byte(stdout), &decoded); err != nil {
			t.Fatalf("json.Unmarshal(session attachment) error = %v", err)
		}
		if decoded.ID != "att-1" || decoded.Name != "frame.png" || decoded.MIMEType != "image/png" {
			t.Fatalf("decoded = %#v, want direct attachment payload", decoded)
		}
	})
}

func TestSessionResumeLatestUsesBoundedPage(t *testing.T) {
	t.Parallel()

	var seenQuery SessionListQuery
	deps := newWorkspaceTestDeps(t, &stubClient{
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

	deps := newWorkspaceTestDeps(t, &stubClient{
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

	t.Run("Should render a flat structured Goal status result", func(t *testing.T) {
		t.Parallel()

		deps := newWorkspaceTestDeps(t, &stubClient{
			sendSessionPromptFn: func(
				_ context.Context,
				id string,
				request SessionPromptRequest,
			) (SessionPromptRecord, error) {
				if id != "sess-1" || request.Message != "/goal status" {
					t.Fatalf("SendSessionPrompt() = %q/%q", id, request.Message)
				}
				return SessionPromptRecord{
					Prompt: SessionPromptResultRecord{
						MessageID: "msg-goal-status", IdempotencyKey: "idem-goal-status",
					},
					Goal: &result,
				}, nil
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
		if string(wrapped["message_id"]) != `"msg-goal-status"` ||
			string(wrapped["idempotency_key"]) != `"idem-goal-status"` {
			t.Fatalf("Goal command JSON identity = %s", stdout)
		}
	})

	t.Run("Should render durable Goal identities in human and TOON formats", func(t *testing.T) {
		t.Parallel()

		for _, tt := range []struct {
			name     string
			format   string
			wantKeys []string
		}{
			{
				name:     "Should render identities in human output",
				format:   "human",
				wantKeys: []string{"Message ID", "Idempotency Key", "msg-goal-status", "idem-goal-status"},
			},
			{
				name:     "Should render identities in TOON output",
				format:   "toon",
				wantKeys: []string{"message_id", "idempotency_key", "msg-goal-status", "idem-goal-status"},
			},
		} {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				deps := newWorkspaceTestDeps(t, &stubClient{
					sendSessionPromptFn: func(
						context.Context,
						string,
						SessionPromptRequest,
					) (SessionPromptRecord, error) {
						return SessionPromptRecord{
							Prompt: SessionPromptResultRecord{
								MessageID: "msg-goal-status", IdempotencyKey: "idem-goal-status",
							},
							Goal: &result,
						}, nil
					},
				})
				stdout, _, err := executeRootCommand(
					t,
					deps,
					"session", "prompt", "sess-1", "/goal status", "-o", tt.format,
				)
				if err != nil {
					t.Fatalf("executeRootCommand(session prompt Goal %s) error = %v", tt.format, err)
				}
				for _, want := range tt.wantKeys {
					if !strings.Contains(stdout, want) {
						t.Fatalf("Goal command %s = %q, want %q", tt.format, stdout, want)
					}
				}
			})
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
				errorDeps := newWorkspaceTestDeps(t, &stubClient{
					sendSessionPromptFn: func(
						context.Context,
						string,
						SessionPromptRequest,
					) (SessionPromptRecord, error) {
						return SessionPromptRecord{}, goalConflict()
					},
					streamPromptSessionFn: func(
						context.Context,
						string,
						SessionPromptRequest,
						SSEHandler,
					) error {
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
		deps := newWorkspaceTestDeps(t, &stubClient{
			sendSessionPromptFn: func(_ context.Context, id string, request SessionPromptRequest) (SessionPromptRecord, error) {
				if id != "sess-1" {
					t.Fatalf("SendSessionPrompt() id = %q, want sess-1", id)
				}
				gotRequest = request
				return SessionPromptRecord{Prompt: SessionPromptResultRecord{
					Status:          "queued",
					Mode:            contract.PromptModeQueue,
					Delivery:        contract.PromptDeliveryAfterTurn,
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
		if gotRequest.Message != "hello" || gotRequest.Mode != contract.PromptModeQueue ||
			gotRequest.MessageID == "" || gotRequest.IdempotencyKey == "" {
			t.Fatalf("SendSessionPrompt() request = %#v, want queue hello", gotRequest)
		}
		var decoded SessionPromptRecord
		if err := json.Unmarshal([]byte(stdout), &decoded); err != nil {
			t.Fatalf("json.Unmarshal(queue prompt) error = %v", err)
		}
		if decoded.Prompt.QueueEntryID != "queue-1" ||
			decoded.Prompt.Delivery != contract.PromptDeliveryAfterTurn {
			t.Fatalf("decoded = %#v, want queued prompt result", decoded)
		}
	})

	t.Run("Should send explicit interrupt mode", func(t *testing.T) {
		t.Parallel()

		var gotRequest SessionPromptRequest
		deps := newWorkspaceTestDeps(t, &stubClient{
			sendSessionPromptFn: func(_ context.Context, id string, request SessionPromptRequest) (SessionPromptRecord, error) {
				if id != "sess-1" {
					t.Fatalf("SendSessionPrompt() id = %q, want sess-1", id)
				}
				gotRequest = request
				return SessionPromptRecord{Prompt: SessionPromptResultRecord{
					Status:    "interrupting",
					Mode:      contract.PromptModeInterrupt,
					Delivery:  contract.PromptDeliveryInterruptThenPrompt,
					NewTurnID: "turn-2",
				}}, nil
			},
		})

		_, _, err := executeRootCommand(
			t,
			deps,
			"session",
			"prompt",
			"sess-1",
			"replace",
			"--interrupt",
			"--expected-turn-id",
			"turn-1",
		)
		if err != nil {
			t.Fatalf("executeRootCommand(session prompt --interrupt) error = %v", err)
		}
		if gotRequest.Message != "replace" || gotRequest.Mode != contract.PromptModeInterrupt ||
			gotRequest.ExpectedTurnID != "turn-1" ||
			gotRequest.MessageID == "" || gotRequest.IdempotencyKey == "" {
			t.Fatalf("SendSessionPrompt() request = %#v, want interrupt replace", gotRequest)
		}
	})

	t.Run("Should require an expected turn for interrupt", func(t *testing.T) {
		t.Parallel()

		deps := newWorkspaceTestDeps(t, &stubClient{
			sendSessionPromptFn: func(context.Context, string, SessionPromptRequest) (SessionPromptRecord, error) {
				t.Fatal("SendSessionPrompt() called without an expected turn id")
				return SessionPromptRecord{}, nil
			},
		})

		_, _, err := executeRootCommand(t, deps, "session", "prompt", "sess-1", "replace", "--interrupt")
		assertErrorContains(t, err, "--expected-turn-id is required")
	})

	t.Run("Should use steer endpoint", func(t *testing.T) {
		t.Parallel()

		var (
			gotID             string
			gotText           string
			gotMessageID      string
			gotIdempotencyKey string
			gotExpectedTurnID string
		)
		deps := newWorkspaceTestDeps(t, &stubClient{
			steerSessionPromptFn: func(
				_ context.Context,
				id string,
				request contract.SteerPromptRequest,
			) (SessionPromptRecord, error) {
				gotID = id
				gotText = request.Text
				gotMessageID = request.MessageID
				gotIdempotencyKey = request.IdempotencyKey
				gotExpectedTurnID = request.ExpectedTurnID
				return SessionPromptRecord{Prompt: SessionPromptResultRecord{
					Status:          "steering",
					Mode:            contract.PromptModeSteer,
					Delivery:        contract.PromptDeliveryInterruptThenPrompt,
					QueueEntryID:    "steer-1",
					QueueGeneration: 3,
				}}, nil
			},
		})

		_, _, err := executeRootCommand(
			t,
			deps,
			"session",
			"prompt",
			"sess-1",
			"prefer small patch",
			"--steer",
			"--expected-turn-id",
			"turn-1",
		)
		if err != nil {
			t.Fatalf("executeRootCommand(session prompt --steer) error = %v", err)
		}
		if gotID != "sess-1" || gotText != "prefer small patch" || gotMessageID == "" || gotIdempotencyKey == "" {
			t.Fatalf("SteerSessionPrompt() = (%q, %q), want (sess-1, prefer small patch)", gotID, gotText)
		}
		if gotExpectedTurnID != "turn-1" {
			t.Fatalf("SteerSessionPrompt() expected turn = %q, want turn-1", gotExpectedTurnID)
		}
	})

	t.Run("Should require an expected turn for steer", func(t *testing.T) {
		t.Parallel()

		deps := newWorkspaceTestDeps(t, &stubClient{
			steerSessionPromptFn: func(context.Context, string, contract.SteerPromptRequest) (SessionPromptRecord, error) {
				t.Fatal("SteerSessionPrompt() called without an expected turn id")
				return SessionPromptRecord{}, nil
			},
		})

		_, _, err := executeRootCommand(t, deps, "session", "prompt", "sess-1", "prefer small patch", "--steer")
		assertErrorContains(t, err, "--expected-turn-id is required")
	})

	t.Run("Should reject mutually exclusive busy actions", func(t *testing.T) {
		t.Parallel()

		deps := newWorkspaceTestDeps(t, &stubClient{})
		_, _, err := executeRootCommand(t, deps, "session", "prompt", "sess-1", "hello", "--queue", "--steer")
		if err == nil || !strings.Contains(err.Error(), "choose only one") {
			t.Fatalf("executeRootCommand(mutually exclusive) error = %v, want choose only one", err)
		}
	})

	for _, tt := range []struct {
		name string
		args []string
	}{
		{
			name: "Should reject runtime selection with steer",
			args: []string{
				"session", "prompt", "sess-1", "hello", "--steer", "--expected-turn-id", "turn-1", "--provider", "codex",
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			deps := newWorkspaceTestDeps(t, &stubClient{})
			_, _, err := executeRootCommand(t, deps, tt.args...)
			if err == nil || !strings.Contains(err.Error(), "runtime selection applies only to submitted prompts") {
				t.Fatalf("executeRootCommand(%#v) error = %v, want runtime selection rejection", tt.args, err)
			}
		})
	}
}

func TestSessionInputCommands(t *testing.T) {
	t.Parallel()

	t.Run("Should list pending input with structured output", func(t *testing.T) {
		t.Parallel()

		client := &profileTestDaemonClient{
			DaemonClient: withWorkspaceResolution(&stubClient{
				listSessionInputsFn: func(ctx context.Context, sessionID string) (SessionInputListRecord, error) {
					if sessionID != "sess-1" {
						t.Fatalf("ListSessionInputs() session = %q, want sess-1", sessionID)
					}
					if got := profileQueryValues(ctx, nil).Get(profileFlagName); got != "marketing" {
						t.Fatalf("ListSessionInputs() profile query = %q, want marketing", got)
					}
					return SessionInputListRecord{Inputs: []SessionInputRecord{{
						ID: "queue-1", SessionID: "sess-1", Status: "queued", Mode: contract.PromptModeQueue,
						Delivery: contract.PromptDeliveryAfterTurn, Text: "Run the focused test first.",
					}}}, nil
				},
			}),
			profileClientAPI: &profileClientStub{profiles: []contract.Profile{
				{Name: "default", State: "active"},
				{Name: "marketing", State: "active"},
			}},
		}
		deps := newWorkspaceTestDeps(t, client)

		stdout, _, err := executeRootCommand(
			t,
			deps,
			"--profile",
			"marketing",
			"session",
			"input",
			"list",
			"sess-1",
			"-o",
			"json",
		)
		if err != nil {
			t.Fatalf("executeRootCommand(session input list) error = %v", err)
		}
		var decoded SessionInputListRecord
		if err := json.Unmarshal([]byte(stdout), &decoded); err != nil {
			t.Fatalf("json.Unmarshal(session input list) error = %v", err)
		}
		if len(decoded.Inputs) != 1 || decoded.Inputs[0].ID != "queue-1" {
			t.Fatalf("session input list = %#v, want queue-1", decoded)
		}
	})

	t.Run("Should replace queued input with a durable identity", func(t *testing.T) {
		t.Parallel()

		var received ReplaceSessionInputRequest
		deps := newDefaultProfileWorkspaceTestDeps(t, &stubClient{
			replaceSessionInputFn: func(
				ctx context.Context,
				sessionID string,
				inputID string,
				request ReplaceSessionInputRequest,
			) (SessionInputRecord, error) {
				if sessionID != "sess-1" || inputID != "queue-1" {
					t.Fatalf("ReplaceSessionInput() target = %q/%q, want sess-1/queue-1", sessionID, inputID)
				}
				if got := profileQueryValues(ctx, nil).Get(profileFlagName); got != "default" {
					t.Fatalf("ReplaceSessionInput() profile query = %q, want default", got)
				}
				received = request
				return SessionInputRecord{
					ID:             inputID,
					SessionID:      sessionID,
					MessageID:      request.MessageID,
					IdempotencyKey: request.IdempotencyKey,
					Status:         "queued",
					Mode:           contract.PromptModeQueue,
					Delivery:       contract.PromptDeliveryAfterTurn,
					Text:           request.Text,
				}, nil
			},
		})

		stdout, _, err := executeRootCommand(
			t,
			deps,
			"session",
			"input",
			"edit",
			"sess-1",
			"queue-1",
			"Run the focused test first.",
			"-o",
			"json",
		)
		if err != nil {
			t.Fatalf("executeRootCommand(session input edit) error = %v", err)
		}
		if received.MessageID == "" || received.IdempotencyKey == "" || received.Text != "Run the focused test first." {
			t.Fatalf("ReplaceSessionInput() request = %#v", received)
		}
		var decoded struct {
			Input SessionInputRecord `json:"input"`
		}
		if err := json.Unmarshal([]byte(stdout), &decoded); err != nil {
			t.Fatalf("json.Unmarshal(session input edit) error = %v", err)
		}
		if decoded.Input.ID != "queue-1" || decoded.Input.Text != received.Text {
			t.Fatalf("session input edit = %#v", decoded)
		}
	})

	t.Run("Should promote queued input with an active turn fence", func(t *testing.T) {
		t.Parallel()

		var received PromoteSessionInputRequest
		deps := newDefaultProfileWorkspaceTestDeps(t, &stubClient{
			promoteSessionInputFn: func(
				_ context.Context,
				sessionID string,
				inputID string,
				request PromoteSessionInputRequest,
			) (SessionPromptRecord, error) {
				if sessionID != "sess-1" || inputID != "queue-1" {
					t.Fatalf("PromoteSessionInput() target = %q/%q, want sess-1/queue-1", sessionID, inputID)
				}
				received = request
				return SessionPromptRecord{Prompt: SessionPromptResultRecord{
					Status: "steering", Mode: contract.PromptModeSteer,
					Delivery:     contract.PromptDeliveryInterruptThenPrompt,
					QueueEntryID: inputID,
				}}, nil
			},
		})

		stdout, _, err := executeRootCommand(
			t,
			deps,
			"session",
			"input",
			"steer",
			"sess-1",
			"queue-1",
			"Prefer the smaller patch.",
			"--expected-turn-id",
			"turn-1",
			"-o",
			"json",
		)
		if err != nil {
			t.Fatalf("executeRootCommand(session input steer) error = %v", err)
		}
		if received.ExpectedTurnID != "turn-1" || received.MessageID == "" || received.IdempotencyKey == "" {
			t.Fatalf("PromoteSessionInput() request = %#v", received)
		}
		var decoded SessionPromptRecord
		if err := json.Unmarshal([]byte(stdout), &decoded); err != nil {
			t.Fatalf("json.Unmarshal(session input steer) error = %v", err)
		}
		if decoded.Prompt.Delivery != contract.PromptDeliveryInterruptThenPrompt {
			t.Fatalf(
				"session input steer delivery = %q, want %q",
				decoded.Prompt.Delivery,
				contract.PromptDeliveryInterruptThenPrompt,
			)
		}
	})

	t.Run("Should require an active turn fence when promoting input", func(t *testing.T) {
		t.Parallel()

		deps := newDefaultProfileWorkspaceTestDeps(t, &stubClient{
			promoteSessionInputFn: func(
				context.Context,
				string,
				string,
				PromoteSessionInputRequest,
			) (SessionPromptRecord, error) {
				t.Fatal("PromoteSessionInput() called without an expected turn id")
				return SessionPromptRecord{}, nil
			},
		})

		_, _, err := executeRootCommand(
			t,
			deps,
			"session",
			"input",
			"steer",
			"sess-1",
			"queue-1",
			"Prefer the smaller patch.",
		)
		assertErrorContains(t, err, "expected-turn-id")
	})

	t.Run("Should cancel queued input through the input surface", func(t *testing.T) {
		t.Parallel()

		deps := newDefaultProfileWorkspaceTestDeps(t, &stubClient{
			cancelSessionInputFn: func(_ context.Context, sessionID string, inputID string) (SessionPromptRecord, error) {
				if sessionID != "sess-1" || inputID != "queue-1" {
					t.Fatalf("CancelSessionInput() target = %q/%q, want sess-1/queue-1", sessionID, inputID)
				}
				return SessionPromptRecord{Prompt: SessionPromptResultRecord{
					Status: "canceled", QueueEntryID: inputID,
				}}, nil
			},
		})

		_, _, err := executeRootCommand(t, deps, "session", "input", "cancel", "sess-1", "queue-1")
		if err != nil {
			t.Fatalf("executeRootCommand(session input cancel) error = %v", err)
		}
	})

	t.Run("Should cancel the active prompt through the direct verb", func(t *testing.T) {
		t.Parallel()

		deps := newWorkspaceTestDeps(t, &stubClient{
			getSessionFn: func(context.Context, string) (SessionRecord, error) {
				return SessionRecord{ID: "sess-1", WorkspaceID: "ws-1"}, nil
			},
			cancelSessionPromptFn: func(
				_ context.Context,
				sessionID string,
			) (SessionPromptCancelRecord, error) {
				if sessionID != "sess-1" {
					t.Fatalf("CancelSessionPrompt() id = %q, want sess-1", sessionID)
				}
				return SessionPromptCancelRecord{
					SessionID: sessionID, Outcome: "canceled", TurnID: "turn-1",
				}, nil
			},
		})
		stdout, _, err := executeRootCommand(
			t,
			deps,
			"session",
			"prompt-cancel",
			"sess-1",
			"-o",
			"json",
		)
		if err != nil {
			t.Fatalf("executeRootCommand(session prompt-cancel) error = %v", err)
		}
		var decoded SessionPromptCancelRecord
		if err := json.Unmarshal([]byte(stdout), &decoded); err != nil {
			t.Fatalf("json.Unmarshal(prompt cancel) error = %v", err)
		}
		if decoded.Outcome != "canceled" || decoded.TurnID != "turn-1" {
			t.Fatalf("prompt cancel payload = %#v, want canceled turn", decoded)
		}
	})

	t.Run("Should return exit 66 when no prompt is active", func(t *testing.T) {
		t.Parallel()

		deps := newWorkspaceTestDeps(t, &stubClient{
			getSessionFn: func(context.Context, string) (SessionRecord, error) {
				return SessionRecord{ID: "sess-1", WorkspaceID: "ws-1"}, nil
			},
			cancelSessionPromptFn: func(
				_ context.Context,
				sessionID string,
			) (SessionPromptCancelRecord, error) {
				return SessionPromptCancelRecord{
					SessionID: sessionID, Outcome: "nothing-in-flight",
				}, nil
			},
		})
		stdout, _, err := executeRootCommand(
			t,
			deps,
			"session",
			"prompt-cancel",
			"sess-1",
			"-o",
			"json",
		)
		if err == nil || cliExitCodeForError(err) != 66 {
			t.Fatalf("prompt cancel error = %v, exit = %d, want 66", err, cliExitCodeForError(err))
		}
		var decoded SessionPromptCancelRecord
		if err := json.Unmarshal([]byte(stdout), &decoded); err != nil {
			t.Fatalf("json.Unmarshal(prompt cancel) error = %v", err)
		}
		if decoded.Outcome != "nothing-in-flight" {
			t.Fatalf("prompt cancel payload = %#v, want nothing-in-flight", decoded)
		}
	})
}

func TestSessionPromptJSONLOutput(t *testing.T) {
	t.Run("Should stream prompt events as JSONL without buffering the completed turn", func(t *testing.T) {
		t.Parallel()

		var streamCalled bool
		deps := newWorkspaceTestDeps(t, &stubClient{
			promptSessionFn: func(context.Context, string, string) ([]AgentEventRecord, error) {
				t.Fatal("PromptSession should not be called for prompt -o jsonl")
				return nil, nil
			},
			streamPromptSessionFn: func(
				_ context.Context,
				id string,
				request SessionPromptRequest,
				handler SSEHandler,
			) error {
				streamCalled = true
				if id != "sess-1" || request.Message != "hello" {
					t.Fatalf("StreamPromptSession() = (%q, %#v), want sess-1/hello", id, request)
				}
				if request.MessageID == "" || request.IdempotencyKey == "" {
					t.Fatalf(
						"StreamPromptSession() identity = %q/%q, want generated non-empty values",
						request.MessageID,
						request.IdempotencyKey,
					)
				}
				if request.Runtime == nil || request.Runtime.Provider != "codex" ||
					request.Runtime.Model != "gpt-5.6-sol" {
					t.Fatalf("StreamPromptSession() runtime = %#v, want codex/gpt-5.6-sol", request.Runtime)
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

		stdout, _, err := executeRootCommand(
			t,
			deps,
			"session", "prompt", "sess-1", "hello",
			"--provider", "codex", "--model", "gpt-5.6-sol",
			"-o", "jsonl",
		)
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

	t.Run("Should keep emitted JSONL frames and return a nonzero command error", func(t *testing.T) {
		t.Parallel()

		streamErr := errors.New("peer disconnected before response")
		deps := newWorkspaceTestDeps(t, &stubClient{
			streamPromptSessionFn: func(
				_ context.Context,
				_ string,
				_ SessionPromptRequest,
				handler SSEHandler,
			) error {
				for _, event := range []SSEEvent{
					{Data: mustJSON(t, map[string]any{
						"type":  "text-delta",
						"id":    "turn-1-text",
						"delta": "partial before disconnect",
					})},
					{Data: mustJSON(t, map[string]any{
						"type":      "error",
						"errorText": streamErr.Error(),
					})},
				} {
					if err := handler(event); err != nil {
						return err
					}
				}
				return streamErr
			},
		})

		stdout, _, err := executeRootCommand(
			t,
			deps,
			"session", "prompt", "sess-1", "hello", "-o", "jsonl",
		)
		if !errors.Is(err, streamErr) {
			t.Fatalf("executeRootCommand(session prompt jsonl) error = %v, want %v", err, streamErr)
		}
		lines := strings.Split(strings.TrimSpace(stdout), "\n")
		if got, want := len(lines), 2; got != want {
			t.Fatalf("session prompt jsonl lines = %d, want %d; output=%q", got, want, stdout)
		}
		if !strings.Contains(lines[0], "partial before disconnect") ||
			!strings.Contains(lines[1], "peer disconnected before response") {
			t.Fatalf("session prompt jsonl output = %q, want partial chunk and terminal error", stdout)
		}
	})

	t.Run("Should emit one direct line for a structured Goal result", func(t *testing.T) {
		t.Parallel()

		result := contract.SendPromptResultResponse{Prompt: contract.SendPromptResultPayload{
			Status: "accepted", MessageID: "msg-goal-clear", IdempotencyKey: "idem-goal-clear",
			Goal: &contract.GoalCommandResult{Outcome: contract.GoalOutcomeCleared},
		}}
		deps := newWorkspaceTestDeps(t, &stubClient{
			streamPromptSessionFn: func(
				_ context.Context,
				id string,
				request SessionPromptRequest,
				handler SSEHandler,
			) error {
				if id != "sess-1" || request.Message != "/goal clear" {
					t.Fatalf("StreamPromptSession() = %q/%#v", id, request)
				}
				return handler(SSEEvent{Event: promptResultEventName, Data: mustJSON(t, result)})
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
		var decoded contract.SendPromptResultResponse
		if err := json.Unmarshal([]byte(lines[0]), &decoded); err != nil {
			t.Fatalf("json.Unmarshal(Goal prompt jsonl) error = %v", err)
		}
		if decoded.Prompt.Goal == nil || decoded.Prompt.Goal.Outcome != contract.GoalOutcomeCleared ||
			decoded.Prompt.MessageID != "msg-goal-clear" || decoded.Prompt.IdempotencyKey != "idem-goal-clear" {
			t.Fatalf("Goal prompt jsonl = %#v", decoded)
		}
	})
}

func TestSessionListBundleRendersHumanAndToon(t *testing.T) {
	t.Parallel()

	items := []SessionRecord{{
		ID:          "sess-1",
		ProfileName: "default",
		Name:        "demo",
		AgentName:   "coder",
		Runtime: contract.SessionRuntimePayload{Effective: &contract.RuntimeSelectionPayload{
			Provider: "fake",
		}},
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
		!strings.Contains(human, "default") ||
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
		!strings.Contains(toon, "default") ||
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

	bundle := sessionBundle(&SessionRecord{
		ID:        "sess-1",
		Name:      "demo",
		AgentName: "coder",
		Runtime: contract.SessionRuntimePayload{Effective: &contract.RuntimeSelectionPayload{
			Provider: "fake",
			Speed:    contract.SpeedFast,
			SpeedResolution: &contract.SpeedResolution{
				Requested: contract.SpeedFast,
				Status:    contract.SpeedResolutionUnsupported,
				Reason:    contract.SpeedResolutionReasonCapabilityAbsent,
			},
		}},
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
	if !strings.Contains(strings.ToLower(human), "provider") ||
		!strings.Contains(human, "fake") ||
		!strings.Contains(human, "fast") ||
		!strings.Contains(human, "unsupported:capability_absent") {
		t.Fatalf("sessionBundle().human() = %q, want provider and speed output", human)
	}

	toon, err := bundle.toon()
	if err != nil {
		t.Fatalf("sessionBundle().toon() error = %v", err)
	}
	if !strings.Contains(strings.ToLower(toon), "provider") ||
		!strings.Contains(toon, "fake") ||
		!strings.Contains(toon, "fast") ||
		!strings.Contains(toon, "unsupported:capability_absent") {
		t.Fatalf("sessionBundle().toon() = %q, want provider and speed output", toon)
	}
}
