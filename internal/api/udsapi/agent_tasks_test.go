package udsapi

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/compozy/agh/internal/api/contract"
	"github.com/compozy/agh/internal/session"
	taskpkg "github.com/compozy/agh/internal/task"
)

func TestAgentTaskClaimNextUsesCallerIdentityAndReturnsCoordinationChannel(t *testing.T) {
	t.Parallel()

	rawToken := "agh_claim_TESTTOKEN123"
	leaseUntil := time.Date(2026, 4, 26, 10, 5, 0, 0, time.UTC)
	claimHash, err := taskpkg.ClaimTokenHash(rawToken)
	if err != nil {
		t.Fatalf("ClaimTokenHash() error = %v", err)
	}

	var seenCriteria taskpkg.ClaimCriteria
	var seenActor taskpkg.ActorContext
	handlers := newAgentTaskHandlers(t, &stubTaskManager{
		ClaimNextRunFn: func(
			_ context.Context,
			criteria taskpkg.ClaimCriteria,
			actor taskpkg.ActorContext,
		) (*taskpkg.ClaimResult, error) {
			seenCriteria = criteria
			seenActor = actor
			run := agentTaskRun(taskpkg.TaskRunStatusClaimed)
			run.ClaimTokenHash = claimHash
			run.LeaseUntil = leaseUntil
			taskRecord := agentTaskRecord()
			return &taskpkg.ClaimResult{
				Task:       &taskRecord,
				Run:        run,
				ClaimToken: rawToken,
				LeaseUntil: leaseUntil,
				CoordinationChannel: &taskpkg.CoordinationChannelMetadata{
					ID:                  "builders",
					DisplayName:         "Builders",
					Purpose:             "coordinated execution",
					WorkspaceID:         "ws-1",
					TaskID:              "task-1",
					RunID:               "run-1",
					WorkflowID:          "wf-1",
					AllowedMessageKinds: []string{"status", "result"},
					LastActivityAt:      leaseUntil.Add(-time.Minute),
				},
			}, nil
		},
	})
	handlers.AgentContextService = agentContextServiceFunc(
		func(_ context.Context, info *session.Info) (contract.AgentContextPayload, error) {
			if info.ID != "sess-agent" {
				t.Fatalf("ContextForSession info = %#v, want caller session", info)
			}
			return contract.AgentContextPayload{
				Capabilities: contract.AgentCapabilitySectionPayload{
					Capabilities: []contract.AgentCapabilityPayload{{ID: "go"}},
				},
			}, nil
		},
	)
	engine := newTestRouter(t, handlers)

	recorder := performAgentKernelRequest(
		t,
		engine,
		http.MethodPost,
		"/api/agent/tasks/claim-next",
		[]byte(
			`{"run_id":"run-1","workspace_id":"ws-1",`+
				`"required_capabilities":["manual"],"priority_min":2,"lease_seconds":120}`,
		),
		agentKernelHeaders(),
	)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	if strings.Contains(recorder.Body.String(), rawToken) ||
		strings.Contains(recorder.Body.String(), `"claim_token"`) {
		t.Fatalf("claim response exposed raw token material: %s", recorder.Body.String())
	}

	var response contract.AgentTaskClaimResponse
	decodeJSONResponse(t, recorder, &response)
	if response.Claim.Lease.ClaimTokenHash != claimHash ||
		response.Claim.Lease.ResolvedNetworkParticipation == nil ||
		response.Claim.Lease.ResolvedNetworkParticipation.ChannelID != "builders" ||
		response.Claim.CoordinationChannel == nil ||
		response.Claim.CoordinationChannel.DisplayName != "Builders" ||
		response.Claim.Run.CoordinationChannel == nil {
		t.Fatalf("claim response = %#v, want hash and coordination metadata", response.Claim)
	}
	if seenCriteria.RunID != "run-1" ||
		seenCriteria.WorkspaceID != "ws-1" ||
		seenCriteria.ClaimerSessionID != "sess-agent" ||
		seenCriteria.AgentName != "coder" ||
		seenCriteria.ParticipationChannel != "" ||
		seenCriteria.PriorityMin != 2 ||
		seenCriteria.LeaseDuration != 120*time.Second {
		t.Fatalf("criteria = %#v, want exact run plus caller workspace/session/agent and flags", seenCriteria)
	}
	wantParticipation := udsTestLiveParticipation("ws-1", "builders")
	if seenCriteria.CallerNetworkParticipation == nil ||
		*seenCriteria.CallerNetworkParticipation != wantParticipation {
		t.Fatalf(
			"criteria.CallerNetworkParticipation = %#v, want %#v",
			seenCriteria.CallerNetworkParticipation,
			wantParticipation,
		)
	}
	if !containsString(seenCriteria.RequiredCapabilities, "manual") ||
		!containsString(seenCriteria.RequiredCapabilities, "go") {
		t.Fatalf(
			"criteria.RequiredCapabilities = %#v, want request and context capabilities",
			seenCriteria.RequiredCapabilities,
		)
	}
	if seenActor.Actor.Kind != taskpkg.ActorKindAgentSession ||
		seenActor.Actor.Ref != "sess-agent" ||
		seenActor.Origin.Ref != "agent.task.next" {
		t.Fatalf("actor = %#v, want agent-session actor and action origin", seenActor)
	}
}

func TestAgentTaskClaimNextNoWorkReturnsNoContent(t *testing.T) {
	t.Parallel()

	handlers := newAgentTaskHandlers(t, &stubTaskManager{
		ClaimNextRunFn: func(
			context.Context,
			taskpkg.ClaimCriteria,
			taskpkg.ActorContext,
		) (*taskpkg.ClaimResult, error) {
			return nil, taskpkg.ErrNoClaimableRun
		},
	})
	recorder := performAgentKernelRequest(
		t,
		newTestRouter(t, handlers),
		http.MethodPost,
		"/api/agent/tasks/claim-next",
		[]byte(`{}`),
		agentKernelHeaders(),
	)
	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusNoContent, recorder.Body.String())
	}
	if strings.TrimSpace(recorder.Body.String()) != "" {
		t.Fatalf("body = %q, want empty no-work response", recorder.Body.String())
	}
}

func TestAgentTaskClaimNextWorkspaceCapacityDeferral(t *testing.T) {
	t.Parallel()

	t.Run("Should return conflict for a non-waiting caller when workspace capacity is full", func(t *testing.T) {
		t.Parallel()

		handlers := newAgentTaskHandlers(t, &stubTaskManager{
			ClaimNextRunFn: func(
				context.Context,
				taskpkg.ClaimCriteria,
				taskpkg.ActorContext,
			) (*taskpkg.ClaimResult, error) {
				return nil, taskpkg.ErrWorkspaceActiveRunCapReached
			},
		})
		recorder := performAgentKernelRequest(
			t,
			newTestRouter(t, handlers),
			http.MethodPost,
			"/api/agent/tasks/claim-next",
			[]byte(`{}`),
			agentKernelHeaders(),
		)
		if recorder.Code != http.StatusConflict {
			t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusConflict, recorder.Body.String())
		}
		if !strings.Contains(recorder.Body.String(), taskpkg.ErrWorkspaceActiveRunCapReached.Error()) {
			t.Fatalf("body = %q, want capacity deferral detail", recorder.Body.String())
		}
	})

	t.Run("Should poll until workspace capacity becomes available for a waiting caller", func(t *testing.T) {
		t.Parallel()

		claimCalls := 0
		taskRecord := agentTaskRecord()
		handlers := newAgentTaskHandlers(t, &stubTaskManager{
			ClaimNextRunFn: func(
				context.Context,
				taskpkg.ClaimCriteria,
				taskpkg.ActorContext,
			) (*taskpkg.ClaimResult, error) {
				claimCalls++
				if claimCalls == 1 {
					return nil, taskpkg.ErrWorkspaceActiveRunCapReached
				}
				return &taskpkg.ClaimResult{Task: &taskRecord, Run: agentTaskRun(taskpkg.TaskRunStatusClaimed)}, nil
			},
		})
		handlers.PollInterval = time.Millisecond
		recorder := performAgentKernelRequest(
			t,
			newTestRouter(t, handlers),
			http.MethodPost,
			"/api/agent/tasks/claim-next",
			[]byte(`{"wait":true}`),
			agentKernelHeaders(),
		)
		if recorder.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
		}
		if !strings.Contains(recorder.Body.String(), `"run_id":"run-1"`) {
			t.Fatalf("body = %q, want claimed run payload", recorder.Body.String())
		}
		if claimCalls != 2 {
			t.Fatalf("ClaimNextRun() calls = %d, want 2", claimCalls)
		}
	})
}

func TestAgentTaskLeaseMutationsUseSessionBoundLookupAndDoNotEchoToken(t *testing.T) {
	t.Parallel()

	rawToken := "agh_claim_MUTATIONTOKEN123"
	lookupFn := func(
		_ context.Context,
		sessionID string,
		runID string,
	) (taskpkg.AutonomyLeaseHandle, error) {
		return agentTaskLeaseHandleForTest(t, sessionID, runID, rawToken, time.Now().UTC().Add(time.Minute)), nil
	}
	for _, tt := range []struct {
		name    string
		path    string
		body    string
		manager *stubTaskManager
		status  taskpkg.RunStatus
	}{
		{
			name:   "heartbeat",
			path:   "/api/agent/tasks/run-1/heartbeat",
			body:   `{"lease_seconds":60}`,
			status: taskpkg.TaskRunStatusClaimed,
			manager: &stubTaskManager{
				LookupActiveRunForSessionFn: lookupFn,
				HeartbeatRunLeaseFn: func(
					_ context.Context,
					heartbeat taskpkg.LeaseHeartbeat,
					actor taskpkg.ActorContext,
				) (*taskpkg.Run, error) {
					if heartbeat.RunID != "run-1" ||
						heartbeat.ClaimToken != rawToken ||
						heartbeat.LeaseDuration != time.Minute ||
						actor.Actor.Ref != "sess-agent" {
						t.Fatalf("heartbeat = %#v actor=%#v, want fenced caller request", heartbeat, actor)
					}
					run := agentTaskRun(taskpkg.TaskRunStatusClaimed)
					run.LeaseUntil = time.Date(2026, 4, 26, 10, 6, 0, 0, time.UTC)
					return &run, nil
				},
			},
		},
		{
			name:   "complete",
			path:   "/api/agent/tasks/run-1/complete",
			body:   `{"result":{"ok":true}}`,
			status: taskpkg.TaskRunStatusCompleted,
			manager: &stubTaskManager{
				LookupActiveRunForSessionFn: lookupFn,
				CompleteRunLeaseFn: func(
					_ context.Context,
					completion taskpkg.LeaseCompletion,
					actor taskpkg.ActorContext,
				) (*taskpkg.Run, error) {
					if completion.RunID != "run-1" ||
						completion.ClaimToken != rawToken ||
						string(completion.Result.Value) != `{"ok":true}` ||
						actor.Actor.Ref != "sess-agent" {
						t.Fatalf("completion = %#v actor=%#v, want fenced caller request", completion, actor)
					}
					run := agentTaskRun(taskpkg.TaskRunStatusCompleted)
					return &run, nil
				},
			},
		},
		{
			name:   "fail",
			path:   "/api/agent/tasks/run-1/fail",
			body:   `{"error":"boom","metadata":{"code":"E_TASK"}}`,
			status: taskpkg.TaskRunStatusFailed,
			manager: &stubTaskManager{
				LookupActiveRunForSessionFn: lookupFn,
				FailRunLeaseFn: func(
					_ context.Context,
					failure taskpkg.LeaseFailure,
					actor taskpkg.ActorContext,
				) (*taskpkg.Run, error) {
					if failure.RunID != "run-1" ||
						failure.ClaimToken != rawToken ||
						failure.Failure.Error != "boom" ||
						string(failure.Failure.Metadata) != `{"code":"E_TASK"}` ||
						actor.Actor.Ref != "sess-agent" {
						t.Fatalf("failure = %#v actor=%#v, want fenced caller request", failure, actor)
					}
					run := agentTaskRun(taskpkg.TaskRunStatusFailed)
					return &run, nil
				},
			},
		},
		{
			name:   "release",
			path:   "/api/agent/tasks/run-1/release",
			body:   `{"reason":"handoff"}`,
			status: taskpkg.TaskRunStatusQueued,
			manager: &stubTaskManager{
				LookupActiveRunForSessionFn: lookupFn,
				ReleaseRunLeaseFn: func(
					_ context.Context,
					release taskpkg.LeaseRelease,
					actor taskpkg.ActorContext,
				) (*taskpkg.Run, error) {
					if release.RunID != "run-1" ||
						release.ClaimToken != rawToken ||
						release.Reason != "handoff" ||
						actor.Actor.Ref != "sess-agent" {
						t.Fatalf("release = %#v actor=%#v, want fenced caller request", release, actor)
					}
					run := agentTaskRun(taskpkg.TaskRunStatusQueued)
					return &run, nil
				},
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			recorder := performAgentKernelRequest(
				t,
				newTestRouter(t, newAgentTaskHandlers(t, tt.manager)),
				http.MethodPost,
				tt.path,
				[]byte(tt.body),
				agentKernelHeaders(),
			)
			if recorder.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
			}
			if strings.Contains(recorder.Body.String(), rawToken) {
				t.Fatalf("response leaked raw token %q: %s", rawToken, recorder.Body.String())
			}
			if strings.Contains(recorder.Body.String(), `"claim_token"`) {
				t.Fatalf("response exposed raw claim_token field: %s", recorder.Body.String())
			}
			var response contract.AgentTaskLeaseResponse
			decodeJSONResponse(t, recorder, &response)
			if response.Lease.RunID != "run-1" ||
				response.Lease.Status != tt.status ||
				response.Lease.SessionID != "sess-agent" ||
				response.Lease.ResolvedNetworkParticipation == nil ||
				response.Lease.ResolvedNetworkParticipation.ChannelID != "builders" {
				t.Fatalf(
					"lease = %#v, want run-1 status %s session sess-agent channel builders",
					response.Lease,
					tt.status,
				)
			}
		})
	}
}

func TestAgentTaskHandlersRejectDeniedMalformedAndRedactToken(t *testing.T) {
	t.Parallel()

	rawToken := "agh_claim_DENIEDTOKEN123"
	t.Run("Should missing identity", func(t *testing.T) {
		t.Parallel()

		recorder := performAgentKernelRequest(
			t,
			newTestRouter(t, newAgentTaskHandlers(t, &stubTaskManager{})),
			http.MethodPost,
			"/api/agent/tasks/claim-next",
			[]byte(`{}`),
			map[string]string{},
		)
		if recorder.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want %d", recorder.Code, http.StatusUnauthorized)
		}
	})

	t.Run("Should permission denied redacts token in error", func(t *testing.T) {
		t.Parallel()

		handlers := newAgentTaskHandlers(t, &stubTaskManager{
			ClaimNextRunFn: func(
				context.Context,
				taskpkg.ClaimCriteria,
				taskpkg.ActorContext,
			) (*taskpkg.ClaimResult, error) {
				return nil, fmt.Errorf("%w: denied %s", taskpkg.ErrPermissionDenied, rawToken)
			},
		})
		recorder := performAgentKernelRequest(
			t,
			newTestRouter(t, handlers),
			http.MethodPost,
			"/api/agent/tasks/claim-next",
			[]byte(`{}`),
			agentKernelHeaders(),
		)
		if recorder.Code != http.StatusForbidden {
			t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusForbidden, recorder.Body.String())
		}
		if strings.Contains(recorder.Body.String(), rawToken) ||
			!strings.Contains(recorder.Body.String(), "agh_claim_[REDACTED]") {
			t.Fatalf("body = %s, want redacted claim token", recorder.Body.String())
		}
	})

	t.Run("Should malformed payload", func(t *testing.T) {
		t.Parallel()

		recorder := performAgentKernelRequest(
			t,
			newTestRouter(t, newAgentTaskHandlers(t, &stubTaskManager{})),
			http.MethodPost,
			"/api/agent/tasks/run-1/heartbeat",
			[]byte(`{"lease_seconds":`),
			agentKernelHeaders(),
		)
		if recorder.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusBadRequest, recorder.Body.String())
		}
	})

	t.Run("Should reject unknown lease mutation fields", func(t *testing.T) {
		t.Parallel()

		for _, testCase := range []struct {
			name string
			path string
			body string
		}{
			{
				name: "heartbeat",
				path: "/api/agent/tasks/run-1/heartbeat",
				body: `{"lease_seconds":60,"legacy":true}`,
			},
			{
				name: "complete",
				path: "/api/agent/tasks/run-1/complete",
				body: `{"result":{},"legacy":true}`,
			},
			{
				name: "fail",
				path: "/api/agent/tasks/run-1/fail",
				body: `{"error":"failed","legacy":true}`,
			},
			{
				name: "release",
				path: "/api/agent/tasks/run-1/release",
				body: `{"reason":"retry","legacy":true}`,
			},
		} {
			t.Run("Should reject unknown fields on "+testCase.name, func(t *testing.T) {
				t.Parallel()

				recorder := performAgentKernelRequest(
					t,
					newTestRouter(t, newAgentTaskHandlers(t, &stubTaskManager{})),
					http.MethodPost,
					testCase.path,
					[]byte(testCase.body),
					agentKernelHeaders(),
				)
				if recorder.Code != http.StatusBadRequest ||
					!strings.Contains(recorder.Body.String(), "unknown_field") {
					t.Fatalf(
						"status = %d, want %d unknown-field response; body=%s",
						recorder.Code,
						http.StatusBadRequest,
						recorder.Body.String(),
					)
				}
			})
		}
	})

	t.Run("Should stale token maps conflict and redacts token", func(t *testing.T) {
		t.Parallel()

		recorder := performAgentKernelRequest(
			t,
			newTestRouter(t, newAgentTaskHandlers(t, &stubTaskManager{
				LookupActiveRunForSessionFn: func(
					context.Context,
					string,
					string,
				) (taskpkg.AutonomyLeaseHandle, error) {
					return taskpkg.AutonomyLeaseHandle{}, fmt.Errorf("%w: %s", taskpkg.ErrInvalidClaimToken, rawToken)
				},
			})),
			http.MethodPost,
			"/api/agent/tasks/run-1/release",
			[]byte(`{}`),
			agentKernelHeaders(),
		)
		if recorder.Code != http.StatusConflict {
			t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusConflict, recorder.Body.String())
		}
		if strings.Contains(recorder.Body.String(), rawToken) {
			t.Fatalf("response leaked raw token: %s", recorder.Body.String())
		}
	})

	t.Run("Should complete result rejects raw claim token before service", func(t *testing.T) {
		t.Parallel()

		recorder := performAgentKernelRequest(
			t,
			newTestRouter(t, newAgentTaskHandlers(t, &stubTaskManager{
				CompleteRunLeaseFn: func(context.Context, taskpkg.LeaseCompletion, taskpkg.ActorContext) (*taskpkg.Run, error) {
					t.Fatal("CompleteRunLease should not be called for raw claim_token result")
					return nil, errors.New("unexpected")
				},
			})),
			http.MethodPost,
			"/api/agent/tasks/run-1/complete",
			[]byte(`{"result":{"claim_token":"secret"}}`),
			agentKernelHeaders(),
		)
		if recorder.Code != http.StatusUnprocessableEntity {
			t.Fatalf(
				"status = %d, want %d; body=%s",
				recorder.Code,
				http.StatusUnprocessableEntity,
				recorder.Body.String(),
			)
		}
	})
}

func newAgentTaskHandlers(t *testing.T, tasks *stubTaskManager) *Handlers {
	t.Helper()
	return newTestHandlersWithRuntime(
		t,
		activeAgentSessionManager(t),
		stubObserver{},
		nil,
		tasks,
		nil,
		stubWorkspaceService{},
		nil,
		newTestHomePaths(t),
	)
}

func agentTaskRecord() taskpkg.Task {
	return taskpkg.Task{
		ID:          "task-1",
		Identifier:  "AUTO-1",
		Scope:       taskpkg.ScopeWorkspace,
		WorkspaceID: "ws-1",
		Title:       "Run autonomous task",
		Status:      taskpkg.TaskStatusInProgress,
		Priority:    taskpkg.PriorityHigh,
	}
}

func agentTaskRun(status taskpkg.RunStatus) taskpkg.Run {
	now := time.Date(2026, 4, 26, 10, 0, 0, 0, time.UTC)
	run := taskpkg.Run{
		ID:         "run-1",
		TaskID:     "task-1",
		Status:     status,
		Attempt:    1,
		SessionID:  "sess-agent",
		ClaimedBy:  &taskpkg.ActorIdentity{Kind: taskpkg.ActorKindAgentSession, Ref: "sess-agent"},
		QueuedAt:   now,
		ClaimedAt:  now,
		LeaseUntil: now.Add(5 * time.Minute),
	}
	run.SetNetworkState(udsTestLiveParticipation("ws-1", "builders"), "", "", "")
	return run
}

func containsString(values []string, expected string) bool {
	return slices.Contains(values, expected)
}
