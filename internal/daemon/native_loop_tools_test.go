package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/compozy/agh/internal/api/contract"
	core "github.com/compozy/agh/internal/api/core"
	looppkg "github.com/compozy/agh/internal/loop"
	"github.com/compozy/agh/internal/loop/dsl"
	goalpkg "github.com/compozy/agh/internal/loop/goal"
	"github.com/compozy/agh/internal/network/participation"
	"github.com/compozy/agh/internal/session"
	taskpkg "github.com/compozy/agh/internal/task"
	toolspkg "github.com/compozy/agh/internal/tools"
)

func TestDaemonNativeLoopTools(t *testing.T) {
	t.Parallel()

	t.Run("Should forward the bounded catalog query within caller workspace scope", func(t *testing.T) {
		t.Parallel()

		var capturedWorkspaceID string
		var capturedQuery looppkg.CatalogQuery
		lastRunCreatedAt := time.Date(2026, 7, 10, 11, 0, 0, 0, time.UTC)
		registry := newDaemonNativeRegistry(t, &daemonNativeToolsDeps{
			Loops: func() core.LoopService {
				return &nativeLoopServiceStub{
					listLoopsFn: func(
						_ context.Context,
						workspaceID string,
						query looppkg.CatalogQuery,
					) (contract.LoopsResponse, error) {
						capturedWorkspaceID = workspaceID
						capturedQuery = query
						return contract.LoopsResponse{
							Loops: []contract.LoopCatalogEntryPayload{{
								Name: "release",
								LastRun: &contract.LoopCatalogLastRunPayload{
									ID:        "run-release-latest",
									Status:    contract.LoopRunStatusRunning,
									CreatedAt: lastRunCreatedAt,
								},
							}},
							Facets: contract.LoopCatalogFacetsPayload{
								Kinds:      map[string]int{"workspace": 2},
								Categories: map[string]int{"delivery": 2},
								Statuses:   map[string]int{"running": 1},
							},
							Page: contract.CountedCursorPagePayload{
								NextCursor: "cursor-2",
								HasMore:    true,
								Total:      3,
								Limit:      1,
							},
						}, nil
					},
				}
			},
		}, nativeApproveAllPolicyInputs())

		result, err := registry.Call(
			t.Context(),
			toolspkg.Scope{WorkspaceID: "ws-alpha"},
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDLoopList,
				Input: json.RawMessage(
					`{"q":"release","kind":"workspace","category":"delivery","status":"running","sort":"name","cursor":"cursor-1","limit":1}`,
				),
			},
		)
		if err != nil {
			t.Fatalf("Registry.Call(loop_list) error = %v", err)
		}

		if capturedWorkspaceID != "ws-alpha" {
			t.Fatalf("workspaceID = %q, want ws-alpha", capturedWorkspaceID)
		}
		wantQuery := looppkg.CatalogQuery{
			Search:   "release",
			Kind:     looppkg.CatalogKindWorkspace,
			Category: "delivery",
			Status:   looppkg.StatusRunning,
			Sort:     looppkg.CatalogSortName,
			Cursor:   "cursor-1",
			Limit:    1,
		}
		if capturedQuery != wantQuery {
			t.Fatalf("query = %#v, want %#v", capturedQuery, wantQuery)
		}
		requireNativeStructuredContains(t, result, []byte(`"facets"`))
		requireNativeStructuredContains(t, result, []byte(`"id":"run-release-latest"`))
		requireNativeStructuredContains(t, result, []byte(`"status":"running"`))
		requireNativeStructuredContains(t, result, []byte(`"created_at":"2026-07-10T11:00:00Z"`))
		requireNativeStructuredContains(t, result, []byte(`"next_cursor":"cursor-2"`))
		requireNativeStructuredContains(t, result, []byte(`"total":3`))
		if result.Preview != "1 of 3 loops; more available" {
			t.Fatalf("preview = %q, want bounded continuation summary", result.Preview)
		}
	})

	t.Run("Should reject a loop catalog workspace outside caller scope", func(t *testing.T) {
		t.Parallel()

		listCalled := false
		registry := newDaemonNativeRegistry(t, &daemonNativeToolsDeps{
			Loops: func() core.LoopService {
				return &nativeLoopServiceStub{
					listLoopsFn: func(
						context.Context,
						string,
						looppkg.CatalogQuery,
					) (contract.LoopsResponse, error) {
						listCalled = true
						return contract.LoopsResponse{}, nil
					},
				}
			},
		}, nativeApproveAllPolicyInputs())

		_, err := registry.Call(
			t.Context(),
			toolspkg.Scope{WorkspaceID: "ws-alpha"},
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDLoopList,
				Input:  json.RawMessage(`{"workspace_id":"ws-beta"}`),
			},
		)

		var toolErr *toolspkg.ToolError
		if !errors.As(err, &toolErr) {
			t.Fatalf("Registry.Call(loop_list foreign workspace) error = %v, want ToolError", err)
		}
		if toolErr.Code != toolspkg.ErrorCodeDenied ||
			!slices.Contains(toolErr.ReasonCodes, toolspkg.ReasonScopeMismatch) {
			t.Fatalf("tool error = %#v, want scope mismatch denial", toolErr)
		}
		if listCalled {
			t.Fatal("ListLoops was called for a workspace outside caller scope")
		}
	})

	t.Run("Should route dry run through loop service with native tool actor", func(t *testing.T) {
		t.Parallel()

		var capturedStartKind dsl.StartKind
		var capturedActor taskpkg.ActorContext
		var capturedDry bool
		registry := newDaemonNativeRegistry(t, &daemonNativeToolsDeps{
			Loops: func() core.LoopService {
				return &nativeLoopServiceStub{
					runLoopFn: func(
						_ context.Context,
						workspaceID string,
						name string,
						request contract.RunLoopRequest,
						startKind dsl.StartKind,
						actor taskpkg.ActorContext,
						dry bool,
					) (contract.RunLoopResponse, error) {
						if workspaceID != "ws-alpha" || name != "release" {
							t.Fatalf("RunLoop target = %s/%s, want ws-alpha/release", workspaceID, name)
						}
						if request.Inputs["target"] != "prod" {
							t.Fatalf("RunLoop inputs = %#v, want target prod", request.Inputs)
						}
						if request.NetworkParticipation == nil ||
							request.NetworkParticipation.Mode == nil ||
							*request.NetworkParticipation.Mode != participation.ModeLocal {
							t.Fatalf(
								"RunLoop network participation = %#v, want local request",
								request.NetworkParticipation,
							)
						}
						capturedStartKind = startKind
						capturedActor = actor
						capturedDry = dry
						return contract.RunLoopResponse{
							DryRun: &contract.LoopPlanPayload{LoopName: "release", ResolvedInputs: request.Inputs},
						}, nil
					},
				}
			},
		}, nativeApproveAllPolicyInputs())

		result, err := registry.Call(
			t.Context(),
			toolspkg.Scope{SessionID: "sess-caller", WorkspaceID: "ws-alpha"},
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDLoopRun,
				Input: json.RawMessage(
					`{"workspace_id":"ws-alpha","name":"release","inputs":{"target":"prod"},` +
						`"network_participation":{"mode":"local"},"dry":true}`,
				),
			},
		)
		if err != nil {
			t.Fatalf("Registry.Call(loop_run) error = %v", err)
		}

		if capturedStartKind != dsl.StartNativeTool {
			t.Fatalf("startKind = %q, want native_tool", capturedStartKind)
		}
		if capturedActor.Actor.Kind != taskpkg.ActorKindAgentSession || capturedActor.Actor.Ref != "sess-caller" {
			t.Fatalf("actor = %#v, want agent session sess-caller", capturedActor.Actor)
		}
		if !capturedDry {
			t.Fatal("dry = false, want true")
		}
		requireNativeStructuredContains(t, result, []byte(`"dry_run"`))
		requireNativeStructuredContains(t, result, []byte(`"release"`))
	})

	t.Run("Should keep loop tools unavailable until service is ready", func(t *testing.T) {
		t.Parallel()

		var loopSvc core.LoopService
		registry := newDaemonNativeRegistry(t, &daemonNativeToolsDeps{
			Loops: func() core.LoopService { return loopSvc },
		}, nativeApproveAllPolicyInputs())
		scope := toolspkg.Scope{Operator: true}

		views, err := registry.OperatorProjection(t.Context(), scope)
		if err != nil {
			t.Fatalf("OperatorProjection(before service) error = %v", err)
		}
		requireNativeToolUnavailableReason(t, views, toolspkg.ToolIDLoopList)
		requireNativeToolUnavailableReason(t, views, toolspkg.ToolIDLoopApprove)

		loopSvc = &nativeLoopServiceStub{
			listLoopsFn: func(context.Context, string, looppkg.CatalogQuery) (contract.LoopsResponse, error) {
				return contract.LoopsResponse{}, nil
			},
		}
		views, err = registry.OperatorProjection(t.Context(), scope)
		if err != nil {
			t.Fatalf("OperatorProjection(after service) error = %v", err)
		}
		requireNativeToolAvailable(t, views, toolspkg.ToolIDLoopList)
	})

	t.Run("Should surface shared service self approval denial", func(t *testing.T) {
		t.Parallel()

		approveCalled := false
		registry := newDaemonNativeRegistry(t, &daemonNativeToolsDeps{
			Loops: func() core.LoopService {
				return &nativeLoopServiceStub{
					approveLoopRunFn: func(context.Context, string, string, contract.ApproveLoopRunRequest, taskpkg.ActorContext) error {
						approveCalled = true
						return taskpkg.ErrPermissionDenied
					},
				}
			},
		}, nativeApproveAllPolicyInputs())

		_, err := registry.Call(
			t.Context(),
			toolspkg.Scope{SessionID: "sess-author", WorkspaceID: "ws-alpha"},
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDLoopApprove,
				Input: json.RawMessage(
					`{"workspace_id":"ws-alpha","run_id":"looprun-1","gate_id":"human",` +
						`"decision":"approve","approval_token_hash":"sha256:` +
						strings.Repeat("a", 64) +
						`"}`,
				),
			},
		)

		var toolErr *toolspkg.ToolError
		if !errors.As(err, &toolErr) {
			t.Fatalf("Registry.Call(loop_approve) error = %v, want ToolError", err)
		}
		if !slices.Contains(toolErr.ReasonCodes, toolspkg.ReasonApprovalSelfDenied) ||
			!slices.Contains(toolErr.ReasonCodes, toolspkg.ReasonPolicyDenied) {
			t.Fatalf("ReasonCodes = %#v, want self approval and policy denial", toolErr.ReasonCodes)
		}
		if !approveCalled {
			t.Fatal("ApproveLoopRun was not called")
		}
	})

	t.Run("Should reject raw approval tokens without echoing them", func(t *testing.T) {
		t.Parallel()

		registry := newDaemonNativeRegistry(t, &daemonNativeToolsDeps{
			Loops: func() core.LoopService { return &nativeLoopServiceStub{} },
		}, nativeApproveAllPolicyInputs())

		_, err := registry.Call(
			t.Context(),
			toolspkg.Scope{SessionID: "sess-reviewer"},
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDLoopApprove,
				Input: json.RawMessage(
					`{"workspace_id":"ws-alpha","run_id":"looprun-1","gate_id":"human","decision":"approve","approval_token_hash":"raw-secret-token"}`,
				),
			},
		)

		var toolErr *toolspkg.ToolError
		if !errors.As(err, &toolErr) {
			t.Fatalf("Registry.Call(loop_approve raw token) error = %v, want ToolError", err)
		}
		if toolErr.Code != toolspkg.ErrorCodeInvalidInput ||
			!slices.Contains(toolErr.ReasonCodes, toolspkg.ReasonSchemaInvalid) {
			t.Fatalf("tool error = %#v, want invalid input schema error", toolErr)
		}
		if strings.Contains(toolErr.Error(), "raw-secret-token") {
			t.Fatalf("tool error leaked raw token: %v", toolErr)
		}
	})

	t.Run("Should expose a visible Goal through origin and active binding alias until clear", func(t *testing.T) {
		t.Parallel()

		visible := true
		aliasActive := true
		snapshot := &session.GoalSnapshot{
			RunID: "run-goal", NodeID: "goal", Objective: "ship safely",
			OriginSessionID: "session-origin", BoundSessionID: "session-bound",
			Status: "complete", RunStatus: "done", TurnsUsed: 2, TurnLimit: 4,
			ContractSummary: "objective satisfied", Context: session.GoalContextSnapshot{
				State: "unknown", NudgeRatio: 0.8,
			},
		}
		loopSvc := &nativeLoopServiceStub{
			getSessionGoalFn: func(_ context.Context, workspaceID string, sessionID string) (*session.GoalSnapshot, error) {
				if workspaceID != "ws-goal" || !visible || sessionID != "session-origin" {
					return nil, nil
				}
				return snapshot, nil
			},
			resolveActiveGoalOriginAliasFn: func(
				_ context.Context,
				workspaceID looppkg.WorkspaceID,
				sessionID string,
			) (string, bool, error) {
				if workspaceID == "ws-goal" && sessionID == "session-bound" && aliasActive {
					return "session-origin", true, nil
				}
				return "", false, nil
			},
		}
		registry := newDaemonNativeRegistry(t, &daemonNativeToolsDeps{
			Loops: func() core.LoopService { return loopSvc },
		}, nativeApproveAllPolicyInputs())

		for _, sessionID := range []string{"session-origin", "session-bound"} {
			scope := toolspkg.Scope{WorkspaceID: "ws-goal", SessionID: sessionID, Operator: true}
			views, err := registry.OperatorProjection(t.Context(), scope)
			if err != nil {
				t.Fatalf("OperatorProjection(%s) error = %v", sessionID, err)
			}
			requireNativeToolAvailable(t, views, toolspkg.ToolIDGoalGet)
			result, err := registry.Call(t.Context(), scope, toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDGoalGet,
				Input:  json.RawMessage(`{}`),
			})
			if err != nil {
				t.Fatalf("Registry.Call(goal_get %s) error = %v", sessionID, err)
			}
			requireNativeStructuredContains(t, result, []byte(`"run_id":"run-goal"`))
			requireNativeStructuredContains(t, result, []byte(`"live":false`))
		}

		aliasActive = false
		aliasViews, err := registry.OperatorProjection(
			t.Context(),
			toolspkg.Scope{WorkspaceID: "ws-goal", SessionID: "session-bound", Operator: true},
		)
		if err != nil {
			t.Fatalf("OperatorProjection(alias revoked) error = %v", err)
		}
		requireNativeToolUnavailableWithReason(
			t,
			aliasViews,
			toolspkg.ToolIDGoalGet,
			toolspkg.ReasonGoalNotActive,
		)

		visible = false
		originViews, err := registry.OperatorProjection(
			t.Context(),
			toolspkg.Scope{WorkspaceID: "ws-goal", SessionID: "session-origin", Operator: true},
		)
		if err != nil {
			t.Fatalf("OperatorProjection(cleared) error = %v", err)
		}
		requireNativeToolUnavailableWithReason(
			t,
			originViews,
			toolspkg.ToolIDGoalGet,
			toolspkg.ReasonGoalNotActive,
		)
	})

	t.Run("Should report only against the invocation-time prompt identity", func(t *testing.T) {
		t.Parallel()

		target := goalpkg.ToolReportTarget{
			Key: goalpkg.TurnKey{
				WorkspaceID: "ws-goal", LoopRunID: "run-goal", Generation: 2,
				NodeID: "goal", ItemIndex: 0,
			},
			ExpectedControlEpoch: 3, ExpectedBindingEpoch: 4,
			PromptID: "prompt-current", OriginSessionID: "session-origin",
			BoundSessionID: "session-bound",
		}
		var captured goalpkg.RecordToolReportRequest
		loopSvc := &nativeLoopServiceStub{
			findGoalReportTargetFn: func(
				_ context.Context,
				workspaceID looppkg.WorkspaceID,
				sessionID string,
			) (goalpkg.ToolReportTarget, bool, error) {
				if workspaceID != "ws-goal" || sessionID != "session-bound" {
					t.Fatalf("FindGoalReportTarget target = %s/%s", workspaceID, sessionID)
				}
				return target, true, nil
			},
			recordGoalReportFn: func(
				_ context.Context,
				request goalpkg.RecordToolReportRequest,
			) (goalpkg.ReportIntent, error) {
				captured = request
				return goalpkg.ReportIntent{
					PromptID: request.Target.PromptID, Status: request.Status,
					EvidenceRef: "sha256:evidence", BindingEpoch: request.Target.ExpectedBindingEpoch,
					ActorKind: request.ActorKind, ActorID: request.ActorID,
					RecordedAt: time.Date(2026, 7, 10, 20, 0, 0, 0, time.UTC),
				}, nil
			},
		}
		registry := newDaemonNativeRegistry(t, &daemonNativeToolsDeps{
			Loops: func() core.LoopService { return loopSvc },
		}, nativeApproveAllPolicyInputs())
		scope := toolspkg.Scope{WorkspaceID: "ws-goal", SessionID: "session-bound"}
		result, err := registry.Call(t.Context(), scope, toolspkg.CallRequest{
			ToolID: toolspkg.ToolIDGoalReport,
			Input:  json.RawMessage(`{"status":"blocked","evidence":"waiting on release approval"}`),
		})
		if err != nil {
			t.Fatalf("Registry.Call(goal_report) error = %v", err)
		}
		if captured.Target != target || captured.Status != "blocked" ||
			captured.Evidence != "waiting on release approval" {
			t.Fatalf("RecordGoalReport request = %#v", captured)
		}
		if captured.ActorKind != string(taskpkg.ActorKindAgentSession) || captured.ActorID != "session-bound" {
			t.Fatalf("RecordGoalReport actor = %s/%s", captured.ActorKind, captured.ActorID)
		}
		requireNativeStructuredContains(t, result, []byte(`"prompt_id":"prompt-current"`))
		requireNativeStructuredContains(t, result, []byte(`"evidence_ref":"sha256:evidence"`))

		findCalls := 0
		loopSvc.findGoalReportTargetFn = func(
			context.Context,
			looppkg.WorkspaceID,
			string,
		) (goalpkg.ToolReportTarget, bool, error) {
			findCalls++
			if findCalls == 1 {
				return target, true, nil
			}
			return goalpkg.ToolReportTarget{}, false, nil
		}
		captured = goalpkg.RecordToolReportRequest{}
		_, err = registry.Call(t.Context(), scope, toolspkg.CallRequest{
			ToolID: toolspkg.ToolIDGoalReport,
			Input:  json.RawMessage(`{"status":"complete"}`),
		})
		var toolErr *toolspkg.ToolError
		if !errors.As(err, &toolErr) ||
			!slices.Contains(toolErr.ReasonCodes, toolspkg.ReasonGoalNotActive) {
			t.Fatalf("Registry.Call(goal_report revoked) error = %#v", err)
		}
		if captured.Target.Key.LoopRunID != "" {
			t.Fatalf("RecordGoalReport called after revoke: %#v", captured)
		}
	})

	t.Run("Should return the canonical paginated Goal turn contract", func(t *testing.T) {
		t.Parallel()

		item := 1
		next := int64(9)
		var captured core.GoalTurnListQuery
		loopSvc := &nativeLoopServiceStub{
			listGoalTurnsFn: func(
				_ context.Context,
				workspaceID string,
				runID string,
				query core.GoalTurnListQuery,
			) (session.GoalTurnPage, error) {
				if workspaceID != "ws-goal" || runID != "run-goal" {
					t.Fatalf("ListGoalTurns target = %s/%s", workspaceID, runID)
				}
				captured = query
				return session.GoalTurnPage{
					Turns: []session.GoalTurn{{
						Seq: 9, Generation: 2, NodeID: "goal", ItemIndex: 1,
						Turn: 3, PromptAttempt: 0, SessionID: "session-bound",
						BindingHandle: "goal:handle", BindingEpoch: 4, PromptID: "prompt-9",
						BlockingIssues: []session.GoalBlockingIssue{}, ActorKind: "agent_session",
						ActorID: "session-bound", StartedAt: time.Date(2026, 7, 10, 20, 0, 0, 0, time.UTC),
					}},
					NextAfterSeq: &next,
				}, nil
			},
		}
		registry := newDaemonNativeRegistry(t, &daemonNativeToolsDeps{
			Loops: func() core.LoopService { return loopSvc },
		}, nativeApproveAllPolicyInputs())
		result, err := registry.Call(t.Context(), toolspkg.Scope{WorkspaceID: "ws-goal"}, toolspkg.CallRequest{
			ToolID: toolspkg.ToolIDLoopTurns,
			Input: json.RawMessage(
				`{"run_id":"run-goal","node":"goal","item":1,"after_seq":4,"limit":5}`,
			),
		})
		if err != nil {
			t.Fatalf("Registry.Call(loop_turns) error = %v", err)
		}
		if captured.NodeID != "goal" || captured.ItemIndex == nil || *captured.ItemIndex != item ||
			captured.AfterSeq != 4 || captured.Limit != 5 {
			t.Fatalf("ListGoalTurns query = %#v", captured)
		}
		requireNativeStructuredContains(t, result, []byte(`"prompt_id":"prompt-9"`))
		requireNativeStructuredContains(t, result, []byte(`"result_status":null`))
		requireNativeStructuredContains(t, result, []byte(`"next_after_seq":9`))
	})
}

type nativeLoopServiceStub struct {
	listLoopsFn            func(context.Context, string, looppkg.CatalogQuery) (contract.LoopsResponse, error)
	createLoopFn           func(context.Context, string, contract.CreateLoopRequest) (contract.LoopResponse, error)
	getLoopFn              func(context.Context, string, string) (contract.LoopResponse, error)
	patchLoopFn            func(context.Context, string, string, contract.PatchLoopRequest) (contract.LoopResponse, error)
	validateLoopFn         func(context.Context, string, string, contract.ValidateLoopRequest) (contract.LoopValidationResponse, error)
	deleteLoopFn           func(context.Context, string, string) error
	runLoopFn              func(context.Context, string, string, contract.RunLoopRequest, dsl.StartKind, taskpkg.ActorContext, bool) (contract.RunLoopResponse, error)
	getLoopConfigFn        func(context.Context, string, string) (contract.LoopConfigResponse, error)
	putLoopConfigFn        func(context.Context, string, string, contract.PutLoopConfigRequest) (contract.LoopConfigResponse, error)
	listLoopRunsFn         func(context.Context, string, core.LoopRunListQuery) (contract.LoopRunsResponse, error)
	getLoopRunFn           func(context.Context, string, string) (contract.LoopRunResponse, error)
	getSessionGoalFn       func(context.Context, string, string) (*session.GoalSnapshot, error)
	listGoalTurnsFn        func(context.Context, string, string, core.GoalTurnListQuery) (session.GoalTurnPage, error)
	findGoalReportTargetFn func(
		context.Context,
		looppkg.WorkspaceID,
		string,
	) (goalpkg.ToolReportTarget, bool, error)
	resolveActiveGoalOriginAliasFn func(context.Context, looppkg.WorkspaceID, string) (string, bool, error)
	recordGoalReportFn             func(context.Context, goalpkg.RecordToolReportRequest) (goalpkg.ReportIntent, error)
	stopLoopRunFn                  func(context.Context, string, string) error
	pauseLoopRunFn                 func(context.Context, string, string) error
	resumeLoopRunFn                func(context.Context, string, string) error
	approveLoopRunFn               func(context.Context, string, string, contract.ApproveLoopRunRequest, taskpkg.ActorContext) error
	listLoopRunEventsFn            func(context.Context, string, string, int64) ([]contract.LoopRunEventPayload, error)
}

var _ core.LoopService = (*nativeLoopServiceStub)(nil)

func (s *nativeLoopServiceStub) GetSessionGoal(
	ctx context.Context,
	workspaceID string,
	sessionID string,
) (*session.GoalSnapshot, error) {
	if s.getSessionGoalFn != nil {
		return s.getSessionGoalFn(ctx, workspaceID, sessionID)
	}
	return nil, errors.New("unexpected GetSessionGoal call")
}

func (s *nativeLoopServiceStub) ListGoalTurns(
	ctx context.Context,
	workspaceID string,
	runID string,
	query core.GoalTurnListQuery,
) (session.GoalTurnPage, error) {
	if s.listGoalTurnsFn != nil {
		return s.listGoalTurnsFn(ctx, workspaceID, runID, query)
	}
	return session.GoalTurnPage{}, errors.New("unexpected ListGoalTurns call")
}

func (s *nativeLoopServiceStub) findGoalReportTarget(
	ctx context.Context,
	workspaceID looppkg.WorkspaceID,
	sessionID string,
) (goalpkg.ToolReportTarget, bool, error) {
	if s.findGoalReportTargetFn != nil {
		return s.findGoalReportTargetFn(ctx, workspaceID, sessionID)
	}
	return goalpkg.ToolReportTarget{}, false, errors.New("unexpected FindGoalReportTarget call")
}

func (s *nativeLoopServiceStub) resolveActiveGoalOriginAlias(
	ctx context.Context,
	workspaceID looppkg.WorkspaceID,
	sessionID string,
) (string, bool, error) {
	if s.resolveActiveGoalOriginAliasFn != nil {
		return s.resolveActiveGoalOriginAliasFn(ctx, workspaceID, sessionID)
	}
	return "", false, errors.New("unexpected ResolveActiveGoalOriginAlias call")
}

func (s *nativeLoopServiceStub) recordGoalReport(
	ctx context.Context,
	request goalpkg.RecordToolReportRequest,
) (goalpkg.ReportIntent, error) {
	if s.recordGoalReportFn != nil {
		return s.recordGoalReportFn(ctx, request)
	}
	return goalpkg.ReportIntent{}, errors.New("unexpected RecordGoalReport call")
}

func requireNativeToolUnavailableWithReason(
	t *testing.T,
	views []toolspkg.ToolView,
	id toolspkg.ToolID,
	reason toolspkg.ReasonCode,
) {
	t.Helper()
	view := nativeToolViewByID(views, id)
	if view == nil || view.Availability.Available ||
		!slices.Contains(view.Availability.ReasonCodes, reason) {
		t.Fatalf("%s availability = %#v, want unavailable with %s", id, view, reason)
	}
}

func (s *nativeLoopServiceStub) ListLoops(
	ctx context.Context,
	workspaceID string,
	query looppkg.CatalogQuery,
) (contract.LoopsResponse, error) {
	if s.listLoopsFn != nil {
		return s.listLoopsFn(ctx, workspaceID, query)
	}
	return contract.LoopsResponse{}, errors.New("unexpected ListLoops call")
}

func (s *nativeLoopServiceStub) CreateLoop(
	ctx context.Context,
	workspaceID string,
	req contract.CreateLoopRequest,
) (contract.LoopResponse, error) {
	if s.createLoopFn != nil {
		return s.createLoopFn(ctx, workspaceID, req)
	}
	return contract.LoopResponse{}, errors.New("unexpected CreateLoop call")
}

func (s *nativeLoopServiceStub) GetLoop(
	ctx context.Context,
	workspaceID string,
	name string,
) (contract.LoopResponse, error) {
	if s.getLoopFn != nil {
		return s.getLoopFn(ctx, workspaceID, name)
	}
	return contract.LoopResponse{}, errors.New("unexpected GetLoop call")
}

func (s *nativeLoopServiceStub) PatchLoop(
	ctx context.Context,
	workspaceID string,
	name string,
	req contract.PatchLoopRequest,
) (contract.LoopResponse, error) {
	if s.patchLoopFn != nil {
		return s.patchLoopFn(ctx, workspaceID, name, req)
	}
	return contract.LoopResponse{}, errors.New("unexpected PatchLoop call")
}

func (s *nativeLoopServiceStub) ValidateLoop(
	ctx context.Context,
	workspaceID string,
	name string,
	req contract.ValidateLoopRequest,
) (contract.LoopValidationResponse, error) {
	if s.validateLoopFn != nil {
		return s.validateLoopFn(ctx, workspaceID, name, req)
	}
	return contract.LoopValidationResponse{}, errors.New("unexpected ValidateLoop call")
}

func (s *nativeLoopServiceStub) DeleteLoop(ctx context.Context, workspaceID string, name string) error {
	if s.deleteLoopFn != nil {
		return s.deleteLoopFn(ctx, workspaceID, name)
	}
	return errors.New("unexpected DeleteLoop call")
}

func (s *nativeLoopServiceStub) RunLoop(
	ctx context.Context,
	workspaceID string,
	name string,
	req contract.RunLoopRequest,
	startKind dsl.StartKind,
	actor taskpkg.ActorContext,
	dry bool,
) (contract.RunLoopResponse, error) {
	if s.runLoopFn != nil {
		return s.runLoopFn(ctx, workspaceID, name, req, startKind, actor, dry)
	}
	return contract.RunLoopResponse{}, errors.New("unexpected RunLoop call")
}

func (s *nativeLoopServiceStub) GetLoopConfig(
	ctx context.Context,
	workspaceID string,
	name string,
) (contract.LoopConfigResponse, error) {
	if s.getLoopConfigFn != nil {
		return s.getLoopConfigFn(ctx, workspaceID, name)
	}
	return contract.LoopConfigResponse{}, errors.New("unexpected GetLoopConfig call")
}

func (s *nativeLoopServiceStub) PutLoopConfig(
	ctx context.Context,
	workspaceID string,
	name string,
	req contract.PutLoopConfigRequest,
) (contract.LoopConfigResponse, error) {
	if s.putLoopConfigFn != nil {
		return s.putLoopConfigFn(ctx, workspaceID, name, req)
	}
	return contract.LoopConfigResponse{}, errors.New("unexpected PutLoopConfig call")
}

func (s *nativeLoopServiceStub) GetLoopAnnotations(
	context.Context,
	string,
	string,
) (contract.LoopAnnotationsResponse, error) {
	return contract.LoopAnnotationsResponse{}, errors.New("unexpected GetLoopAnnotations call")
}

func (s *nativeLoopServiceStub) PutLoopAnnotations(
	context.Context,
	string,
	string,
	contract.PutLoopAnnotationsRequest,
) (contract.LoopAnnotationsResponse, error) {
	return contract.LoopAnnotationsResponse{}, errors.New("unexpected PutLoopAnnotations call")
}

func (s *nativeLoopServiceStub) ListLoopRuns(
	ctx context.Context,
	workspaceID string,
	query core.LoopRunListQuery,
) (contract.LoopRunsResponse, error) {
	if s.listLoopRunsFn != nil {
		return s.listLoopRunsFn(ctx, workspaceID, query)
	}
	return contract.LoopRunsResponse{}, errors.New("unexpected ListLoopRuns call")
}

func (s *nativeLoopServiceStub) GetLoopRun(
	ctx context.Context,
	workspaceID string,
	runID string,
) (contract.LoopRunResponse, error) {
	if s.getLoopRunFn != nil {
		return s.getLoopRunFn(ctx, workspaceID, runID)
	}
	return contract.LoopRunResponse{}, errors.New("unexpected GetLoopRun call")
}

func (s *nativeLoopServiceStub) StopLoopRun(
	ctx context.Context,
	workspaceID string,
	runID string,
	_ taskpkg.ActorContext,
) error {
	if s.stopLoopRunFn != nil {
		return s.stopLoopRunFn(ctx, workspaceID, runID)
	}
	return errors.New("unexpected StopLoopRun call")
}

func (s *nativeLoopServiceStub) PauseLoopRun(
	ctx context.Context,
	workspaceID string,
	runID string,
	_ taskpkg.ActorContext,
) error {
	if s.pauseLoopRunFn != nil {
		return s.pauseLoopRunFn(ctx, workspaceID, runID)
	}
	return errors.New("unexpected PauseLoopRun call")
}

func (s *nativeLoopServiceStub) ResumeLoopRun(
	ctx context.Context,
	workspaceID string,
	runID string,
	_ taskpkg.ActorContext,
) error {
	if s.resumeLoopRunFn != nil {
		return s.resumeLoopRunFn(ctx, workspaceID, runID)
	}
	return errors.New("unexpected ResumeLoopRun call")
}

func (s *nativeLoopServiceStub) ApproveLoopRun(
	ctx context.Context,
	workspaceID string,
	runID string,
	req contract.ApproveLoopRunRequest,
	actor taskpkg.ActorContext,
) error {
	if s.approveLoopRunFn != nil {
		return s.approveLoopRunFn(ctx, workspaceID, runID, req, actor)
	}
	return errors.New("unexpected ApproveLoopRun call")
}

func (s *nativeLoopServiceStub) ListLoopRunEvents(
	ctx context.Context,
	workspaceID string,
	runID string,
	afterSeq int64,
) ([]contract.LoopRunEventPayload, error) {
	if s.listLoopRunEventsFn != nil {
		return s.listLoopRunEventsFn(ctx, workspaceID, runID, afterSeq)
	}
	return nil, errors.New("unexpected ListLoopRunEvents call")
}
