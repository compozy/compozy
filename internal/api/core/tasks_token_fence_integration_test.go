//go:build integration

package core_test

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/compozy/agh/internal/api/contract"
	"github.com/compozy/agh/internal/api/testutil"
	taskpkg "github.com/compozy/agh/internal/task"
	"github.com/gin-gonic/gin"
)

type taskRunTokenFenceCapture struct {
	runID        string
	cancel       taskpkg.CancelRun
	actorContext taskpkg.ActorContext
}

func TestTaskRunTokenFenceHandlersHonorImmutableParticipationOwnershipIntegration(t *testing.T) {
	t.Parallel()

	t.Run("Should return conflicts for human complete and fail on token-fenced participation runs", func(t *testing.T) {
		t.Parallel()

		testCases := []struct {
			name             string
			path             string
			body             []byte
			rawToken         string
			wantErrorSuffix  string
			wantOriginRef    string
			buildTaskManager func(
				*testing.T,
				*taskRunTokenFenceCapture,
				string,
				string,
			) *testutil.StubTaskManager
		}{
			{
				name:            "Should reject human completion with a redacted token-fence conflict",
				path:            "/task-runs/run-2/complete",
				body:            []byte(`{"result":{"ok":true}}`),
				rawToken:        "agh_claim_complete-secret-123",
				wantErrorSuffix: "requires token-fenced completion with agh_claim_[REDACTED]",
				wantOriginRef:   "tasks.complete_run",
				buildTaskManager: func(
					t *testing.T,
					capture *taskRunTokenFenceCapture,
					runID string,
					rawToken string,
				) *testutil.StubTaskManager {
					t.Helper()
					return &testutil.StubTaskManager{
						CompleteRunFn: func(
							_ context.Context,
							gotRunID string,
							_ taskpkg.RunResult,
							actor taskpkg.ActorContext,
						) (*taskpkg.Run, error) {
							capture.runID = gotRunID
							capture.actorContext = actor
							return nil, fmt.Errorf(
								"%w: task run %q requires token-fenced completion with %s",
								taskpkg.ErrInvalidClaimToken,
								gotRunID,
								rawToken,
							)
						},
					}
				},
			},
			{
				name:            "Should reject human failure with a redacted token-fence conflict",
				path:            "/task-runs/run-2/fail",
				body:            []byte(`{"error":"boom"}`),
				rawToken:        "agh_claim_fail-secret-456",
				wantErrorSuffix: "requires token-fenced failure with agh_claim_[REDACTED]",
				wantOriginRef:   "tasks.fail_run",
				buildTaskManager: func(
					t *testing.T,
					capture *taskRunTokenFenceCapture,
					runID string,
					rawToken string,
				) *testutil.StubTaskManager {
					t.Helper()
					return &testutil.StubTaskManager{
						FailRunFn: func(
							_ context.Context,
							gotRunID string,
							_ taskpkg.RunFailure,
							actor taskpkg.ActorContext,
						) (*taskpkg.Run, error) {
							capture.runID = gotRunID
							capture.actorContext = actor
							return nil, fmt.Errorf(
								"%w: task run %q requires token-fenced failure with %s",
								taskpkg.ErrInvalidClaimToken,
								gotRunID,
								rawToken,
							)
						},
					}
				},
			},
		}

		for _, tc := range testCases {
			tc := tc
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				capture := &taskRunTokenFenceCapture{}
				fixture := newHandlerFixtureWithTasks(
					t,
					testutil.StubSessionManager{},
					testutil.StubObserver{},
					tc.buildTaskManager(t, capture, "run-2", tc.rawToken),
					testutil.StubWorkspaceService{},
					nil,
					nil,
				)
				fixture.Handlers.TaskActorContextResolver = func(_ *gin.Context, action string) (taskpkg.ActorContext, error) {
					return taskpkg.DeriveHumanActorContext("user-1", taskpkg.OriginKindHTTP, "tasks."+action)
				}

				resp := performRequest(t, fixture.Engine, http.MethodPost, tc.path, tc.body)
				if resp.Code != http.StatusConflict {
					t.Fatalf("status = %d, want %d; body=%s", resp.Code, http.StatusConflict, resp.Body.String())
				}

				var payload contract.ErrorPayload
				testutil.DecodeJSONResponse(t, resp, &payload)

				if strings.Contains(payload.Error, tc.rawToken) {
					t.Fatalf("payload.Error leaked raw claim token: %q", payload.Error)
				}
				if !strings.Contains(payload.Error, tc.wantErrorSuffix) {
					t.Fatalf("payload.Error = %q, want suffix %q", payload.Error, tc.wantErrorSuffix)
				}
				if capture.runID != "run-2" {
					t.Fatalf("runID = %q, want %q", capture.runID, "run-2")
				}
				if capture.actorContext.Actor.Ref != "user-1" {
					t.Fatalf("actorContext.Actor.Ref = %q, want %q", capture.actorContext.Actor.Ref, "user-1")
				}
				if capture.actorContext.Origin.Ref != tc.wantOriginRef {
					t.Fatalf("actorContext.Origin.Ref = %q, want %q", capture.actorContext.Origin.Ref, tc.wantOriginRef)
				}
			})
		}
	})

	t.Run(
		"Should preserve mixed-ownership bindings when human cancel overrides a token-fenced participation run",
		func(t *testing.T) {
			t.Parallel()

			now := time.Date(2026, 4, 28, 11, 0, 0, 0, time.UTC)
			networkSpec := testLiveParticipation("ws-alpha", "scope-direct-history")
			capture := &taskRunTokenFenceCapture{}
			fixture := newHandlerFixtureWithTasks(
				t,
				testutil.StubSessionManager{},
				testutil.StubObserver{},
				&testutil.StubTaskManager{
					CancelRunFn: func(
						_ context.Context,
						runID string,
						req taskpkg.CancelRun,
						actor taskpkg.ActorContext,
					) (*taskpkg.Run, error) {
						capture.runID = runID
						capture.cancel = req
						capture.actorContext = actor
						return &taskpkg.Run{
							ID:      runID,
							TaskID:  "task-1",
							Status:  taskpkg.TaskRunStatusCanceled,
							Attempt: 1,
							ClaimedBy: &taskpkg.ActorIdentity{
								Kind: taskpkg.ActorKindAgentSession,
								Ref:  "sess-history-mixed",
							},
							SessionID:       "sess-history-mixed",
							Origin:          actor.Origin,
							QueuedAt:        now.Add(-2 * time.Minute),
							StartedAt:       now.Add(-time.Minute),
							EndedAt:         now,
							Metadata:        req.Metadata,
							RunNetworkState: &taskpkg.RunNetworkState{NetworkSpec: networkSpec},
						}, nil
					},
				},
				testutil.StubWorkspaceService{},
				nil,
				nil,
			)
			fixture.Handlers.TaskActorContextResolver = func(_ *gin.Context, action string) (taskpkg.ActorContext, error) {
				return taskpkg.DeriveHumanActorContext("user-1", taskpkg.OriginKindHTTP, "tasks."+action)
			}

			resp := performRequest(
				t,
				fixture.Engine,
				http.MethodPost,
				"/task-runs/run-2/cancel",
				[]byte(`{"reason":"operator override","metadata":{"mode":"mixed-token-fence"}}`),
			)
			if resp.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d; body=%s", resp.Code, http.StatusOK, resp.Body.String())
			}

			var payload contract.TaskRunResponse
			testutil.DecodeJSONResponse(t, resp, &payload)

			if payload.Run.Status != taskpkg.TaskRunStatusCanceled {
				t.Fatalf("payload.Run.Status = %q, want %q", payload.Run.Status, taskpkg.TaskRunStatusCanceled)
			}
			if payload.Run.ClaimedBy == nil || payload.Run.ClaimedBy.Ref != "sess-history-mixed" {
				t.Fatalf("payload.Run.ClaimedBy = %#v, want sess-history-mixed", payload.Run.ClaimedBy)
			}
			if payload.Run.SessionID != "sess-history-mixed" {
				t.Fatalf("payload.Run.SessionID = %q, want %q", payload.Run.SessionID, "sess-history-mixed")
			}
			if payload.Run.ResolvedNetworkParticipation == nil ||
				*payload.Run.ResolvedNetworkParticipation != networkSpec {
				t.Fatalf(
					"payload.Run.ResolvedNetworkParticipation = %#v, want %#v",
					payload.Run.ResolvedNetworkParticipation,
					networkSpec,
				)
			}
			if payload.Run.Origin.Ref != "tasks.cancel_run" {
				t.Fatalf("payload.Run.Origin.Ref = %q, want %q", payload.Run.Origin.Ref, "tasks.cancel_run")
			}
			if string(payload.Run.Metadata) != `{"mode":"mixed-token-fence"}` {
				t.Fatalf(
					"payload.Run.Metadata = %s, want %s",
					string(payload.Run.Metadata),
					`{"mode":"mixed-token-fence"}`,
				)
			}
			if capture.runID != "run-2" {
				t.Fatalf("runID = %q, want %q", capture.runID, "run-2")
			}
			if capture.cancel.Reason != "operator override" {
				t.Fatalf("capture.cancel.Reason = %q, want %q", capture.cancel.Reason, "operator override")
			}
			if string(capture.cancel.Metadata) != `{"mode":"mixed-token-fence"}` {
				t.Fatalf(
					"capture.cancel.Metadata = %s, want %s",
					string(capture.cancel.Metadata),
					`{"mode":"mixed-token-fence"}`,
				)
			}
			if capture.actorContext.Actor.Ref != "user-1" {
				t.Fatalf("actorContext.Actor.Ref = %q, want %q", capture.actorContext.Actor.Ref, "user-1")
			}
			if capture.actorContext.Origin.Ref != "tasks.cancel_run" {
				t.Fatalf("actorContext.Origin.Ref = %q, want %q", capture.actorContext.Origin.Ref, "tasks.cancel_run")
			}
		},
	)
}
