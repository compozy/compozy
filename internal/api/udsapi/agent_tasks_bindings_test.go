package udsapi

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/compozy/agh/internal/api/contract"
	"github.com/compozy/agh/internal/session"
	taskpkg "github.com/compozy/agh/internal/task"
)

type agentTaskLeaseBindingCapture struct {
	actorContext taskpkg.ActorContext
	heartbeat    taskpkg.LeaseHeartbeat
	completion   taskpkg.LeaseCompletion
	failure      taskpkg.LeaseFailure
	release      taskpkg.LeaseRelease
}

func TestAgentTaskResponsesProjectImmutableParticipationSnapshots(t *testing.T) {
	t.Parallel()

	const participationChannel = "scope-uds-participation-reclaim-081526"

	t.Run("Should project immutable participation snapshots in claim-next responses", func(t *testing.T) {
		t.Parallel()

		rawToken := "agh_claim_UDSHISTORY123"
		leaseUntil := time.Date(2026, 4, 28, 8, 16, 0, 0, time.UTC)
		claimHash, err := taskpkg.ClaimTokenHash(rawToken)
		if err != nil {
			t.Fatalf("ClaimTokenHash() error = %v", err)
		}

		var seenCriteria taskpkg.ClaimCriteria
		handlers := newParticipationAgentTaskHandlers(t, participationChannel, &stubTaskManager{
			ClaimNextRunFn: func(
				_ context.Context,
				criteria taskpkg.ClaimCriteria,
				actor taskpkg.ActorContext,
			) (*taskpkg.ClaimResult, error) {
				seenCriteria = criteria
				if actor.Actor.Ref != "sess-agent" || actor.Origin.Ref != "agent.task.next" {
					t.Fatalf("actor = %#v, want sess-agent claim actor", actor)
				}
				run := agentTaskRun(taskpkg.TaskRunStatusClaimed)
				run.LeaseUntil = leaseUntil
				run.ClaimTokenHash = claimHash
				run.SetNetworkState(udsTestLiveParticipation("ws-1", participationChannel), "", "", "")
				taskRecord := agentTaskRecord()
				return &taskpkg.ClaimResult{
					Task:       &taskRecord,
					Run:        run,
					ClaimToken: rawToken,
					LeaseUntil: leaseUntil,
					CoordinationChannel: &taskpkg.CoordinationChannelMetadata{
						ID:                  participationChannel,
						DisplayName:         "Participation reclaim lane",
						WorkspaceID:         "ws-1",
						TaskID:              "task-1",
						RunID:               "run-1",
						AllowedMessageKinds: []string{"status", "result"},
						LastActivityAt:      leaseUntil.Add(-time.Minute),
					},
				}, nil
			},
		})
		handlers.AgentContextService = agentContextServiceFunc(
			func(_ context.Context, info *session.Info) (contract.AgentContextPayload, error) {
				if info.ID != "sess-agent" || info.NetworkParticipation.ChannelID != participationChannel {
					t.Fatalf("ContextForSession info = %#v, want sess-agent on %s", info, participationChannel)
				}
				return contract.AgentContextPayload{
					Capabilities: contract.AgentCapabilitySectionPayload{
						Capabilities: []contract.AgentCapabilityPayload{{ID: "manual"}},
					},
				}, nil
			},
		)

		recorder := performAgentKernelRequest(
			t,
			newTestRouter(t, handlers),
			http.MethodPost,
			"/api/agent/tasks/claim-next",
			[]byte(`{"workspace_id":"ws-1","required_capabilities":["triage"],"lease_seconds":90}`),
			agentKernelHeaders(),
		)
		if recorder.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
		}

		var response contract.AgentTaskClaimResponse
		decodeJSONResponse(t, recorder, &response)
		if strings.Contains(recorder.Body.String(), rawToken) ||
			strings.Contains(recorder.Body.String(), `"claim_token"`) {
			t.Fatalf("claim response leaked raw token: %s", recorder.Body.String())
		}

		if response.Claim.Lease.SessionID != "sess-agent" ||
			response.Claim.Lease.ResolvedNetworkParticipation == nil ||
			response.Claim.Lease.ResolvedNetworkParticipation.ChannelID != participationChannel ||
			response.Claim.Run.ResolvedNetworkParticipation == nil ||
			response.Claim.Run.ResolvedNetworkParticipation.ChannelID != participationChannel {
			t.Fatalf("claim response = %#v, want projected participation session/channel bindings", response.Claim)
		}
		if response.Claim.CoordinationChannel == nil ||
			response.Claim.CoordinationChannel.ID != participationChannel ||
			response.Claim.Run.CoordinationChannel == nil ||
			response.Claim.Run.CoordinationChannel.ID != participationChannel {
			t.Fatalf("claim response = %#v, want participation coordination metadata", response.Claim)
		}
		if seenCriteria.ClaimerSessionID != "sess-agent" ||
			seenCriteria.ParticipationChannel != "" ||
			seenCriteria.LeaseDuration != 90*time.Second {
			t.Fatalf("criteria = %#v, want immutable participation caller fencing", seenCriteria)
		}
		wantParticipation := udsTestLiveParticipation("ws-1", participationChannel)
		if seenCriteria.CallerNetworkParticipation == nil ||
			*seenCriteria.CallerNetworkParticipation != wantParticipation {
			t.Fatalf(
				"criteria.CallerNetworkParticipation = %#v, want %#v",
				seenCriteria.CallerNetworkParticipation,
				wantParticipation,
			)
		}
	})

	t.Run("Should project immutable participation snapshots in lease mutation responses", func(t *testing.T) {
		t.Parallel()

		testCases := []struct {
			name          string
			path          string
			body          []byte
			buildManager  func(*testing.T, *agentTaskLeaseBindingCapture, time.Time) *stubTaskManager
			assertCapture func(*testing.T, *agentTaskLeaseBindingCapture)
			wantStatus    taskpkg.RunStatus
		}{
			{
				name: "Should project immutable participation snapshots in heartbeat responses",
				path: "/api/agent/tasks/run-1/heartbeat",
				body: []byte(`{"lease_seconds":60}`),
				buildManager: func(
					t *testing.T,
					capture *agentTaskLeaseBindingCapture,
					now time.Time,
				) *stubTaskManager {
					t.Helper()
					return &stubTaskManager{
						LookupActiveRunForSessionFn: func(
							_ context.Context,
							sessionID string,
							runID string,
						) (taskpkg.AutonomyLeaseHandle, error) {
							return agentTaskLeaseHandleForTest(
								t,
								sessionID,
								runID,
								"agh_claim_UDSHB123",
								now.Add(time.Minute),
							), nil
						},
						HeartbeatRunLeaseFn: func(
							_ context.Context,
							heartbeat taskpkg.LeaseHeartbeat,
							actor taskpkg.ActorContext,
						) (*taskpkg.Run, error) {
							capture.heartbeat = heartbeat
							capture.actorContext = actor
							run := agentTaskRun(taskpkg.TaskRunStatusClaimed)
							run.SetNetworkState(udsTestLiveParticipation("ws-1", participationChannel), "", "", "")
							run.LeaseUntil = now.Add(time.Minute)
							return &run, nil
						},
					}
				},
				assertCapture: func(t *testing.T, capture *agentTaskLeaseBindingCapture) {
					t.Helper()
					if capture.heartbeat.RunID != "run-1" ||
						capture.heartbeat.ClaimToken != "agh_claim_UDSHB123" ||
						capture.heartbeat.LeaseDuration != time.Minute {
						t.Fatalf("heartbeat = %#v, want run-1 + claim token + 60s", capture.heartbeat)
					}
					if capture.actorContext.Actor.Ref != "sess-agent" ||
						capture.actorContext.Origin.Ref != "agent.task.heartbeat" {
						t.Fatalf("actorContext = %#v", capture.actorContext)
					}
				},
				wantStatus: taskpkg.TaskRunStatusClaimed,
			},
			{
				name: "Should project immutable participation snapshots in release responses",
				path: "/api/agent/tasks/run-1/release",
				body: []byte(`{"reason":"handoff"}`),
				buildManager: func(
					t *testing.T,
					capture *agentTaskLeaseBindingCapture,
					_ time.Time,
				) *stubTaskManager {
					t.Helper()
					return &stubTaskManager{
						LookupActiveRunForSessionFn: func(
							_ context.Context,
							sessionID string,
							runID string,
						) (taskpkg.AutonomyLeaseHandle, error) {
							return agentTaskLeaseHandleForTest(
								t,
								sessionID,
								runID,
								"agh_claim_UDSREL123",
								time.Date(2026, 4, 28, 8, 18, 0, 0, time.UTC),
							), nil
						},
						ReleaseRunLeaseFn: func(
							_ context.Context,
							release taskpkg.LeaseRelease,
							actor taskpkg.ActorContext,
						) (*taskpkg.Run, error) {
							capture.release = release
							capture.actorContext = actor
							run := agentTaskRun(taskpkg.TaskRunStatusQueued)
							run.SetNetworkState(udsTestLiveParticipation("ws-1", participationChannel), "", "", "")
							return &run, nil
						},
					}
				},
				assertCapture: func(t *testing.T, capture *agentTaskLeaseBindingCapture) {
					t.Helper()
					if capture.release.RunID != "run-1" ||
						capture.release.ClaimToken != "agh_claim_UDSREL123" ||
						capture.release.Reason != "handoff" {
						t.Fatalf("release = %#v, want run-1 + claim token + handoff", capture.release)
					}
					if capture.actorContext.Actor.Ref != "sess-agent" ||
						capture.actorContext.Origin.Ref != "agent.task.release" {
						t.Fatalf("actorContext = %#v", capture.actorContext)
					}
				},
				wantStatus: taskpkg.TaskRunStatusQueued,
			},
			{
				name: "Should project immutable participation snapshots in complete responses",
				path: "/api/agent/tasks/run-1/complete",
				body: []byte(`{"result":{"status":"done","mode":"uds-history"}}`),
				buildManager: func(
					t *testing.T,
					capture *agentTaskLeaseBindingCapture,
					_ time.Time,
				) *stubTaskManager {
					t.Helper()
					return &stubTaskManager{
						LookupActiveRunForSessionFn: func(
							_ context.Context,
							sessionID string,
							runID string,
						) (taskpkg.AutonomyLeaseHandle, error) {
							return agentTaskLeaseHandleForTest(
								t,
								sessionID,
								runID,
								"agh_claim_UDSCOMP123",
								time.Date(2026, 4, 28, 8, 18, 0, 0, time.UTC),
							), nil
						},
						CompleteRunLeaseFn: func(
							_ context.Context,
							completion taskpkg.LeaseCompletion,
							actor taskpkg.ActorContext,
						) (*taskpkg.Run, error) {
							capture.completion = completion
							capture.actorContext = actor
							run := agentTaskRun(taskpkg.TaskRunStatusCompleted)
							run.SetNetworkState(udsTestLiveParticipation("ws-1", participationChannel), "", "", "")
							run.Result = completion.Result.Value
							return &run, nil
						},
					}
				},
				assertCapture: func(t *testing.T, capture *agentTaskLeaseBindingCapture) {
					t.Helper()
					if capture.completion.RunID != "run-1" ||
						capture.completion.ClaimToken != "agh_claim_UDSCOMP123" ||
						string(capture.completion.Result.Value) != `{"status":"done","mode":"uds-history"}` {
						t.Fatalf("completion = %#v, want preserved completion payload", capture.completion)
					}
					if capture.actorContext.Actor.Ref != "sess-agent" ||
						capture.actorContext.Origin.Ref != "agent.task.complete" {
						t.Fatalf("actorContext = %#v", capture.actorContext)
					}
				},
				wantStatus: taskpkg.TaskRunStatusCompleted,
			},
			{
				name: "Should project immutable participation snapshots in fail responses",
				path: "/api/agent/tasks/run-1/fail",
				body: []byte(`{"error":"boom","metadata":{"step":"reclaim"}}`),
				buildManager: func(
					t *testing.T,
					capture *agentTaskLeaseBindingCapture,
					_ time.Time,
				) *stubTaskManager {
					t.Helper()
					return &stubTaskManager{
						LookupActiveRunForSessionFn: func(
							_ context.Context,
							sessionID string,
							runID string,
						) (taskpkg.AutonomyLeaseHandle, error) {
							return agentTaskLeaseHandleForTest(
								t,
								sessionID,
								runID,
								"agh_claim_UDSFAIL123",
								time.Date(2026, 4, 28, 8, 18, 0, 0, time.UTC),
							), nil
						},
						FailRunLeaseFn: func(
							_ context.Context,
							failure taskpkg.LeaseFailure,
							actor taskpkg.ActorContext,
						) (*taskpkg.Run, error) {
							capture.failure = failure
							capture.actorContext = actor
							run := agentTaskRun(taskpkg.TaskRunStatusFailed)
							run.SetNetworkState(udsTestLiveParticipation("ws-1", participationChannel), "", "", "")
							run.Error = failure.Failure.Error
							run.Metadata = failure.Failure.Metadata
							return &run, nil
						},
					}
				},
				assertCapture: func(t *testing.T, capture *agentTaskLeaseBindingCapture) {
					t.Helper()
					if capture.failure.RunID != "run-1" ||
						capture.failure.ClaimToken != "agh_claim_UDSFAIL123" ||
						capture.failure.Failure.Error != "boom" ||
						string(capture.failure.Failure.Metadata) != `{"step":"reclaim"}` {
						t.Fatalf("failure = %#v, want preserved failure payload", capture.failure)
					}
					if capture.actorContext.Actor.Ref != "sess-agent" ||
						capture.actorContext.Origin.Ref != "agent.task.fail" {
						t.Fatalf("actorContext = %#v", capture.actorContext)
					}
				},
				wantStatus: taskpkg.TaskRunStatusFailed,
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				capture := &agentTaskLeaseBindingCapture{}
				handlers := newParticipationAgentTaskHandlers(
					t,
					participationChannel,
					tc.buildManager(t, capture, time.Date(2026, 4, 28, 8, 17, 0, 0, time.UTC)),
				)
				recorder := performAgentKernelRequest(
					t,
					newTestRouter(t, handlers),
					http.MethodPost,
					tc.path,
					tc.body,
					agentKernelHeaders(),
				)
				if recorder.Code != http.StatusOK {
					t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
				}

				var response contract.AgentTaskLeaseResponse
				decodeJSONResponse(t, recorder, &response)

				if response.Lease.RunID != "run-1" ||
					response.Lease.Status != tc.wantStatus ||
					response.Lease.SessionID != "sess-agent" ||
					response.Lease.ResolvedNetworkParticipation == nil ||
					response.Lease.ResolvedNetworkParticipation.ChannelID != participationChannel {
					t.Fatalf("lease = %#v, want projected participation session/channel bindings", response.Lease)
				}

				tc.assertCapture(t, capture)
			})
		}
	})
}

func newParticipationAgentTaskHandlers(t *testing.T, channelID string, tasks *stubTaskManager) *Handlers {
	t.Helper()

	return newTestHandlersWithRuntime(
		t,
		stubSessionManager{
			StatusFn: func(_ context.Context, id string) (*session.Info, error) {
				if id != "sess-agent" {
					return nil, session.ErrSessionNotFound
				}
				now := time.Date(2026, 4, 28, 8, 15, 0, 0, time.UTC)
				return &session.Info{
					ID:                   "sess-agent",
					Name:                 "worker",
					AgentName:            "coder",
					Provider:             "test-provider",
					WorkspaceID:          "ws-1",
					Workspace:            "/workspace/project",
					NetworkParticipation: udsTestLiveParticipation("ws-1", channelID),
					Type:                 session.SessionTypeUser,
					State:                session.StateActive,
					CreatedAt:            now,
					UpdatedAt:            now,
				}, nil
			},
		},
		stubObserver{},
		nil,
		tasks,
		nil,
		stubWorkspaceService{},
		nil,
		newTestHomePaths(t),
	)
}

func agentTaskLeaseHandleForTest(
	t *testing.T,
	sessionID string,
	runID string,
	rawToken string,
	leaseUntil time.Time,
) taskpkg.AutonomyLeaseHandle {
	t.Helper()
	if sessionID != "sess-agent" || runID != "run-1" {
		t.Fatalf("LookupActiveRunForSession(session=%q, run=%q), want sess-agent/run-1", sessionID, runID)
	}
	hash, err := taskpkg.ClaimTokenHash(rawToken)
	if err != nil {
		t.Fatalf("ClaimTokenHash() error = %v", err)
	}
	return taskpkg.AutonomyLeaseHandle{
		RunID:          runID,
		TaskID:         "task-1",
		WorkspaceID:    "ws-1",
		SessionID:      sessionID,
		Status:         taskpkg.TaskRunStatusClaimed,
		ClaimToken:     rawToken,
		ClaimTokenHash: hash,
		LeaseUntil:     leaseUntil,
	}
}
