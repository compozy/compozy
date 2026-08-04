package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	core "github.com/compozy/compozy/internal/api/core"
	apitest "github.com/compozy/compozy/internal/api/testutil"
	looppkg "github.com/compozy/compozy/internal/loop"
	"github.com/compozy/compozy/internal/loop/dsl"
	goalpkg "github.com/compozy/compozy/internal/loop/goal"
	"github.com/compozy/compozy/internal/network/participation"
	"github.com/compozy/compozy/internal/session"
	taskpkg "github.com/compozy/compozy/internal/task"
	toolspkg "github.com/compozy/compozy/internal/tools"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
)

func TestDaemonNativeLoopTools(t *testing.T) {
	t.Parallel()

	t.Run("Should forward the bounded catalog query within caller workspace scope", func(t *testing.T) {
		t.Parallel()

		var capturedWorkspaceID string
		var capturedQuery looppkg.CatalogQuery
		lastRunCreatedAt := time.Date(2026, 7, 10, 11, 0, 0, 0, time.UTC)
		registry := newDaemonNativeRegistry(t, &daemonNativeToolsDeps{
			Sessions: nativeNetworkTestSessionManager("ws-alpha"),
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
			toolspkg.Scope{SessionID: "sess-alpha", WorkspaceID: "ws-alpha"},
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
			Sessions: nativeNetworkTestSessionManager("ws-alpha"),
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
			Workspaces: nativeNetworkTestWorkspaceService(t),
		}, nativeApproveAllPolicyInputs())

		_, err := registry.Call(
			t.Context(),
			toolspkg.Scope{SessionID: "sess-alpha", WorkspaceID: "ws-alpha"},
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDLoopList,
				Input:  json.RawMessage(`{"workspace":"ws-beta"}`),
			},
		)

		var toolErr *toolspkg.ToolError
		if !errors.As(err, &toolErr) {
			t.Fatalf("Registry.Call(loop_list foreign workspace) error = %v, want ToolError", err)
		}
		if toolErr.Code != toolspkg.ErrorCodeDenied ||
			!slices.Contains(toolErr.ReasonCodes, toolspkg.ReasonWorkspaceAccessDenied) {
			t.Fatalf("tool error = %#v, want workspace access denial", toolErr)
		}
		if listCalled {
			t.Fatal("ListLoops was called for a workspace outside caller scope")
		}
	})

	t.Run("Should preserve current version on stale native Loop publishes", func(t *testing.T) {
		t.Parallel()

		definition := loopAPITestDefinition(t, "release", 1, "Release safely")
		input, err := json.Marshal(map[string]any{
			"definition":       definition,
			"expected_version": 1,
		})
		if err != nil {
			t.Fatalf("json.Marshal(loop_create input) error = %v", err)
		}
		registry := newDaemonNativeRegistry(t, &daemonNativeToolsDeps{
			Sessions:   nativeNetworkTestSessionManager("ws-alpha"),
			Workspaces: nativeNetworkTestWorkspaceService(t),
			Loops: func() core.LoopService {
				return &nativeLoopServiceStub{
					patchLoopFn: func(
						context.Context,
						string,
						string,
						contract.PatchLoopRequest,
					) (contract.LoopResponse, error) {
						return contract.LoopResponse{}, &core.LoopVersionConflictError{CurrentVersion: 2}
					},
				}
			},
		}, nativeApproveAllPolicyInputs())

		_, err = registry.Call(
			t.Context(),
			toolspkg.Scope{SessionID: "sess-alpha", WorkspaceID: "ws-alpha"},
			toolspkg.CallRequest{ToolID: toolspkg.ToolIDLoopCreate, Input: input},
		)
		var toolErr *toolspkg.ToolError
		if !errors.As(err, &toolErr) || toolErr.Code != toolspkg.ErrorCodeConflict {
			t.Fatalf("Registry.Call(loop_create stale) error = %#v, want conflict ToolError", err)
		}
		if !slices.Contains(toolErr.ReasonCodes, toolspkg.ReasonLoopVersionConflict) {
			t.Fatalf("ReasonCodes = %#v, want loop_version_conflict", toolErr.ReasonCodes)
		}
		if toolErr.PartialResult == nil {
			t.Fatal("partial result = nil, want version conflict details")
		}
		var partial struct {
			VersionConflict struct {
				CurrentVersion int `json:"current_version"`
			} `json:"version_conflict"`
		}
		if err := json.Unmarshal(toolErr.PartialResult.Structured, &partial); err != nil {
			t.Fatalf("json.Unmarshal(partial result) error = %v", err)
		}
		if partial.VersionConflict.CurrentVersion != 2 {
			t.Fatalf("partial result = %#v, want version_conflict.current_version 2", partial)
		}
	})

	t.Run("Should deny deletion of immutable native Loop sources", func(t *testing.T) {
		t.Parallel()

		registry := newDaemonNativeRegistry(t, &daemonNativeToolsDeps{
			Sessions:   nativeNetworkTestSessionManager("ws-alpha"),
			Workspaces: nativeNetworkTestWorkspaceService(t),
			Loops: func() core.LoopService {
				return &nativeLoopServiceStub{
					deleteLoopFn: func(context.Context, string, string) error {
						return fmt.Errorf("%w: marketplace source", looppkg.ErrDefinitionReadOnly)
					},
				}
			},
		}, nativeApproveAllPolicyInputs())

		_, err := registry.Call(
			t.Context(),
			toolspkg.Scope{SessionID: "sess-alpha", WorkspaceID: "ws-alpha"},
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDLoopDelete,
				Input:  json.RawMessage(`{"name":"review-and-fix"}`),
			},
		)
		var toolErr *toolspkg.ToolError
		if !errors.As(err, &toolErr) || toolErr.Code != toolspkg.ErrorCodeDenied {
			t.Fatalf("Registry.Call(loop_delete read-only) error = %#v, want denied ToolError", err)
		}
		if !slices.Contains(toolErr.ReasonCodes, toolspkg.ReasonLoopSourceImmutable) ||
			slices.Contains(toolErr.ReasonCodes, toolspkg.ReasonSchemaInvalid) {
			t.Fatalf("ReasonCodes = %#v, want only Loop source immutability semantics", toolErr.ReasonCodes)
		}
	})

	t.Run("Should preserve Loop control reason codes in native errors", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			name         string
			toolID       toolspkg.ToolID
			input        json.RawMessage
			domainReason looppkg.ReasonCode
			wantReason   toolspkg.ReasonCode
		}{
			{
				name:         "Should preserve invalid status transitions for Pause",
				toolID:       toolspkg.ToolIDLoopPause,
				input:        json.RawMessage(`{"run_id":"looprun-paused"}`),
				domainReason: looppkg.ReasonCodeInvalidStatusTransition,
				wantReason:   toolspkg.ReasonLoopInvalidStatusTransition,
			},
			{
				name:         "Should preserve terminal run rejection for Kill",
				toolID:       toolspkg.ToolIDLoopKill,
				input:        json.RawMessage(`{"run_id":"looprun-terminal"}`),
				domainReason: looppkg.ReasonCodeTerminalRun,
				wantReason:   toolspkg.ReasonLoopTerminalRun,
			},
			{
				name:         "Should preserve invalid status transitions for Resume",
				toolID:       toolspkg.ToolIDLoopResume,
				input:        json.RawMessage(`{"run_id":"looprun-running"}`),
				domainReason: looppkg.ReasonCodeInvalidStatusTransition,
				wantReason:   toolspkg.ReasonLoopInvalidStatusTransition,
			},
			{
				name:   "Should preserve invalid status transitions for Approve",
				toolID: toolspkg.ToolIDLoopApprove,
				input: json.RawMessage(
					`{"run_id":"looprun-running","gate_id":"human","decision":"approve"}`,
				),
				domainReason: looppkg.ReasonCodeInvalidStatusTransition,
				wantReason:   toolspkg.ReasonLoopInvalidStatusTransition,
			},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				domainErr := &looppkg.ReasonError{Code: tc.domainReason, Err: looppkg.ErrInvalidTransition}
				registry := newDaemonNativeRegistry(t, &daemonNativeToolsDeps{
					Sessions:   nativeNetworkTestSessionManager("ws-alpha"),
					Workspaces: nativeNetworkTestWorkspaceService(t),
					Loops: func() core.LoopService {
						return &nativeLoopServiceStub{
							killLoopRunFn: func(context.Context, string, string) (contract.LoopMutationResponse, error) {
								return contract.LoopMutationResponse{}, domainErr
							},
							pauseLoopRunFn: func(context.Context, string, string) error {
								return domainErr
							},
							resumeLoopRunFn: func(context.Context, string, string) error {
								return domainErr
							},
							approveLoopRunFn: func(
								context.Context,
								string,
								string,
								contract.ApproveLoopRunRequest,
								taskpkg.ActorContext,
							) error {
								return domainErr
							},
						}
					},
				}, nativeApproveAllPolicyInputs())

				_, err := registry.Call(
					t.Context(),
					toolspkg.Scope{SessionID: "sess-alpha", WorkspaceID: "ws-alpha"},
					toolspkg.CallRequest{ToolID: tc.toolID, Input: tc.input},
				)
				var toolErr *toolspkg.ToolError
				if !errors.As(err, &toolErr) || toolErr.Code != toolspkg.ErrorCodeInvalidInput {
					t.Fatalf("Registry.Call(%s) error = %#v, want invalid-input ToolError", tc.toolID, err)
				}
				if !slices.Contains(toolErr.ReasonCodes, tc.wantReason) ||
					slices.Contains(toolErr.ReasonCodes, toolspkg.ReasonSchemaInvalid) {
					t.Fatalf(
						"ReasonCodes = %#v, want only %q Loop control semantics",
						toolErr.ReasonCodes,
						tc.wantReason,
					)
				}
				if !errors.Is(err, looppkg.ErrInvalidTransition) {
					t.Fatalf("Registry.Call(%s) error = %v, want wrapped ErrInvalidTransition", tc.toolID, err)
				}
			})
		}

		lifecycleErr := &looppkg.ReasonError{
			Code: looppkg.ReasonCodeAlreadyDecided,
			Err:  looppkg.ErrTransitionConflict,
			Meta: map[string]string{
				looppkg.ReasonMetaActualState:        "quarantined",
				looppkg.ReasonMetaAllowedTransitions: "requeue,cancel,kill",
				looppkg.ReasonMetaWinnerActorID:      "operator:alice",
			},
		}
		mapped := nativeLoopReasonToolError(toolspkg.ToolIDLoopNodeRequeue, lifecycleErr)
		var toolErr *toolspkg.ToolError
		if !errors.As(mapped, &toolErr) || toolErr.Code != toolspkg.ErrorCodeConflict ||
			!errors.Is(mapped, toolspkg.ErrToolConflict) ||
			!slices.Contains(toolErr.ReasonCodes, toolspkg.ReasonLoopAlreadyDecided) ||
			toolErr.PartialResult == nil {
			t.Fatalf("native lifecycle error = %#v", mapped)
		}
		var partial contract.ErrorPayload
		if err := json.Unmarshal(toolErr.PartialResult.Structured, &partial); err != nil {
			t.Fatalf("json.Unmarshal(native lifecycle partial result) error = %v", err)
		}
		if partial.Code != string(looppkg.ReasonCodeAlreadyDecided) ||
			partial.Details[looppkg.ReasonMetaActualState] != "quarantined" ||
			partial.Details[looppkg.ReasonMetaWinnerActorID] != "operator:alice" {
			t.Fatalf("native lifecycle partial result = %#v", partial)
		}
	})

	t.Run("Should expose all Loop lifecycle operations through native tools with shared response shapes", func(
		t *testing.T,
	) {
		t.Parallel()

		requestedAt := time.Date(2026, 8, 2, 18, 0, 0, 0, time.UTC)
		provenance := &contract.LoopControlProvenancePayload{
			ActorKind: "session", ActorID: "sess-alpha", Reason: "operator request", RequestedAt: requestedAt,
		}
		var calls []string
		loopSvc := &nativeLoopServiceStub{
			cancelLoopRunFn: func(_ context.Context, workspaceID string, runID string) (contract.LoopMutationResponse, error) {
				calls = append(calls, "run.cancel")
				if workspaceID != "ws-alpha" || runID != "run-1" {
					t.Fatalf("CancelLoopRun target = %s/%s", workspaceID, runID)
				}
				return contract.LoopMutationResponse{
					OK: true, RunID: runID, Status: string(contract.LoopRunStatusCanceled), Provenance: provenance,
				}, nil
			},
			killLoopRunFn: func(_ context.Context, _ string, runID string) (contract.LoopMutationResponse, error) {
				calls = append(calls, "run.kill")
				return contract.LoopMutationResponse{
					OK: true, RunID: runID, Status: string(contract.LoopRunStatusCanceled), Provenance: provenance,
				}, nil
			},
			listLoopNodesFn: func(
				_ context.Context,
				workspaceID string,
				query core.LoopNodeListQuery,
			) (contract.LoopNodeInventoryResponse, error) {
				calls = append(calls, "nodes.list")
				if workspaceID != "ws-alpha" || query.State != "waiting" || query.Limit != 5 {
					t.Fatalf("ListLoopNodes scope/query = %s/%#v", workspaceID, query)
				}
				return contract.LoopNodeInventoryResponse{Items: []contract.LoopNodeInventoryItem{{
					State:      contract.LoopNodeInventoryWaiting,
					LoopRunID:  "run-1",
					LoopName:   "release",
					Generation: 2,
					NodeID:     "approval",
					StateAt:    requestedAt,
				}}}, nil
			},
			pauseLoopNodeFn: func(
				_ context.Context,
				workspaceID string,
				runID string,
				nodeID string,
				req contract.LoopNodePauseRequest,
				_ taskpkg.ActorContext,
			) (contract.LoopMutationResponse, error) {
				calls = append(calls, "node.pause")
				if workspaceID != "ws-alpha" || runID != "run-1" || nodeID != "approval" ||
					req.Mode != "drain" || req.Reason != "inspect" {
					t.Fatalf("PauseLoopNode request = %s/%s/%s %#v", workspaceID, runID, nodeID, req)
				}
				return nativeLoopNodeMutationResponse(runID, nodeID, provenance), nil
			},
			resumeLoopNodeFn: func(
				_ context.Context,
				_ string,
				runID string,
				nodeID string,
				req contract.LoopNodeResumeRequest,
				_ taskpkg.ActorContext,
			) (contract.LoopMutationResponse, error) {
				calls = append(calls, "node.resume")
				validPayload := string(req.Payload) == `{"approved":true}`
				if req.Mode != "plain" || req.ItemIndex == nil || *req.ItemIndex != 0 || !validPayload {
					t.Fatalf("ResumeLoopNode request = %#v", req)
				}
				return nativeLoopNodeMutationResponse(runID, nodeID, provenance), nil
			},
			cancelLoopNodeFn: nativeLoopNodeMutationRecorder(t, &calls, "node.cancel", provenance),
			killLoopNodeFn:   nativeLoopNodeMutationRecorder(t, &calls, "node.kill", provenance),
			requeueLoopNodeFn: nativeLoopNodeMutationRecorder(
				t, &calls, "node.requeue", provenance,
			),
		}
		registry := newDaemonNativeRegistry(t, &daemonNativeToolsDeps{
			Sessions:   nativeNetworkTestSessionManager("ws-alpha"),
			Workspaces: nativeNetworkTestWorkspaceService(t),
			Loops:      func() core.LoopService { return loopSvc },
		}, nativeApproveAllPolicyInputs())

		cases := []struct {
			id        toolspkg.ToolID
			input     string
			inventory bool
		}{
			{id: toolspkg.ToolIDLoopCancel, input: `{"run_id":"run-1"}`},
			{id: toolspkg.ToolIDLoopKill, input: `{"run_id":"run-1"}`},
			{id: toolspkg.ToolIDLoopNodes, input: `{"state":"waiting","limit":5}`, inventory: true},
			{
				id:    toolspkg.ToolIDLoopNodePause,
				input: `{"run_id":"run-1","node_id":"approval","mode":"drain","reason":"inspect"}`,
			},
			{
				id:    toolspkg.ToolIDLoopNodeResume,
				input: `{"run_id":"run-1","node_id":"approval","mode":"plain","item_index":0,"payload":{"approved":true}}`,
			},
			{
				id:    toolspkg.ToolIDLoopNodeCancel,
				input: `{"run_id":"run-1","node_id":"approval","reason":"inspect"}`,
			},
			{
				id:    toolspkg.ToolIDLoopNodeKill,
				input: `{"run_id":"run-1","node_id":"approval","reason":"inspect"}`,
			},
			{
				id:    toolspkg.ToolIDLoopNodeRequeue,
				input: `{"run_id":"run-1","node_id":"approval","reason":"inspect"}`,
			},
		}
		for _, tc := range cases {
			result, err := registry.Call(
				t.Context(),
				toolspkg.Scope{SessionID: "sess-alpha", WorkspaceID: "ws-alpha"},
				toolspkg.CallRequest{ToolID: tc.id, Input: json.RawMessage(tc.input)},
			)
			if err != nil {
				t.Fatalf("Registry.Call(%s) error = %v", tc.id, err)
			}
			if tc.inventory {
				var response contract.LoopNodeInventoryResponse
				if err := json.Unmarshal(result.Structured, &response); err != nil || len(response.Items) != 1 {
					t.Fatalf("%s structured result = %s, error = %v", tc.id, result.Structured, err)
				}
				continue
			}
			var response contract.LoopMutationResponse
			if err := json.Unmarshal(result.Structured, &response); err != nil {
				t.Fatalf("json.Unmarshal(%s result) error = %v", tc.id, err)
			}
			if !response.OK || response.RunID != "run-1" || response.Provenance == nil {
				t.Fatalf("%s structured result = %#v", tc.id, response)
			}
		}

		wantCalls := []string{
			"run.cancel", "run.kill", "nodes.list", "node.pause", "node.resume",
			"node.cancel", "node.kill", "node.requeue",
		}
		if !slices.Equal(calls, wantCalls) {
			t.Fatalf("lifecycle calls = %#v, want %#v", calls, wantCalls)
		}
	})

	t.Run("Should route dry run through loop service with native tool actor", func(t *testing.T) {
		t.Parallel()

		var capturedStartKind dsl.StartKind
		var capturedActor taskpkg.ActorContext
		var capturedDry bool
		registry := newDaemonNativeRegistry(t, &daemonNativeToolsDeps{
			Sessions: nativeNetworkTestSessionManager("ws-alpha"),
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
							DryRun: &contract.LoopPlanPayload{
								LoopName:       "release",
								ResolvedInputs: request.Inputs,
								InputOrigins: map[string]contract.LoopInputOrigin{
									"target": contract.LoopInputOriginRun,
								},
							},
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
					`{"workspace":"ws-alpha","name":"release","inputs":{"target":"prod"},` +
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
		requireNativeStructuredContains(t, result, []byte(`"input_origins":{"target":"run"}`))
	})

	t.Run("Should preserve the persisted run web URL in structured output", func(t *testing.T) {
		t.Parallel()

		registry := newDaemonNativeRegistry(t, &daemonNativeToolsDeps{
			Sessions: nativeNetworkTestSessionManager("ws-alpha"),
			Loops: func() core.LoopService {
				return &nativeLoopServiceStub{
					runLoopFn: func(
						context.Context,
						string,
						string,
						contract.RunLoopRequest,
						dsl.StartKind,
						taskpkg.ActorContext,
						bool,
					) (contract.RunLoopResponse, error) {
						return contract.RunLoopResponse{
							Run:    &contract.LoopRunPayload{ID: "run-release"},
							WebURL: "http://127.0.0.1:43127/loop-runs/run-release",
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
				Input:  json.RawMessage(`{"name":"release"}`),
			},
		)
		if err != nil {
			t.Fatalf("Registry.Call(loop_run persisted) error = %v", err)
		}
		requireNativeStructuredContains(
			t,
			result,
			[]byte(`"web_url":"http://127.0.0.1:43127/loop-runs/run-release"`),
		)
	})

	t.Run("Should return typed input-default diagnostics for native submissions", func(t *testing.T) {
		t.Parallel()

		registry := newDaemonNativeRegistry(t, &daemonNativeToolsDeps{
			Sessions: nativeNetworkTestSessionManager("ws-alpha"),
			Loops: func() core.LoopService {
				return &nativeLoopServiceStub{
					runLoopFn: func(
						context.Context,
						string,
						string,
						contract.RunLoopRequest,
						dsl.StartKind,
						taskpkg.ActorContext,
						bool,
					) (contract.RunLoopResponse, error) {
						return contract.RunLoopResponse{}, &looppkg.InputDefaultError{
							Loop:   "review-and-fix",
							Key:    "unknown",
							Reason: looppkg.InputDefaultReasonUnknownInput,
							Err:    errors.New("input is not declared"),
						}
					},
				}
			},
		}, nativeApproveAllPolicyInputs())

		_, err := registry.Call(
			t.Context(),
			toolspkg.Scope{SessionID: "sess-caller", WorkspaceID: "ws-alpha"},
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDLoopRun,
				Input:  json.RawMessage(`{"name":"review-and-fix","inputs":{"unknown":true},"dry":true}`),
			},
		)
		var toolErr *toolspkg.ToolError
		if !errors.As(err, &toolErr) || toolErr.Code != toolspkg.ErrorCodeInvalidInput {
			t.Fatalf("Registry.Call(loop_run input default) error = %#v, want invalid-input ToolError", err)
		}
		if toolErr.PartialResult == nil {
			t.Fatalf("loop_run input-default ToolError = %#v, want structured partial result", toolErr)
		}
		structured := string(toolErr.PartialResult.Structured)
		for _, want := range []string{
			`"input_default"`,
			`"loop":"review-and-fix"`,
			`"key":"unknown"`,
			`"reason":"unknown_input"`,
		} {
			if !strings.Contains(structured, want) {
				t.Fatalf("loop_run input-default structured payload = %s, want %s", structured, want)
			}
		}
	})

	t.Run("Should return structured runtime validation diagnostics", func(t *testing.T) {
		t.Parallel()

		definition := loopAPITestDefinition(t, "release", 1, "Release safely")
		input, err := json.Marshal(map[string]any{
			"workspace":  "ws-alpha",
			"name":       "release",
			"definition": definition,
		})
		if err != nil {
			t.Fatalf("json.Marshal(loop_validate input) error = %v", err)
		}
		registry := newDaemonNativeRegistry(t, &daemonNativeToolsDeps{
			Sessions: nativeNetworkTestSessionManager("ws-alpha"),
			Loops: func() core.LoopService {
				return &nativeLoopServiceStub{
					validateLoopFn: func(
						context.Context,
						string,
						string,
						contract.ValidateLoopRequest,
					) (contract.LoopValidationResponse, error) {
						return contract.LoopValidationResponse{}, looppkg.NewRuntimeValidationError(
							looppkg.RuntimeValidationItem{
								TaskID: "task_06",
								Field:  "model",
								Value:  "unknown-model",
								Reason: "unknown_model",
							},
						)
					},
				}
			},
		}, nativeApproveAllPolicyInputs())

		result, err := registry.Call(
			t.Context(),
			toolspkg.Scope{SessionID: "sess-alpha", WorkspaceID: "ws-alpha"},
			toolspkg.CallRequest{ToolID: toolspkg.ToolIDLoopValidate, Input: input},
		)
		if err != nil {
			t.Fatalf("Registry.Call(loop_validate) error = %v", err)
		}
		requireNativeStructuredContains(t, result, []byte(`"valid":false`))
		requireNativeStructuredContains(t, result, []byte(`"runtime_validation"`))
		requireNativeStructuredContains(t, result, []byte(`"task_id":"task_06"`))
		requireNativeStructuredContains(t, result, []byte(`"reason":"unknown_model"`))
	})

	t.Run("Should reject retired runtime keys before calling the loop service", func(t *testing.T) {
		t.Parallel()

		validateCalled := false
		registry := newDaemonNativeRegistry(t, &daemonNativeToolsDeps{
			Sessions: nativeNetworkTestSessionManager("ws-alpha"),
			Loops: func() core.LoopService {
				return &nativeLoopServiceStub{
					validateLoopFn: func(
						context.Context,
						string,
						string,
						contract.ValidateLoopRequest,
					) (contract.LoopValidationResponse, error) {
						validateCalled = true
						return contract.LoopValidationResponse{}, nil
					},
				}
			},
		}, nativeApproveAllPolicyInputs())

		_, err := registry.Call(
			t.Context(),
			toolspkg.Scope{SessionID: "sess-alpha", WorkspaceID: "ws-alpha"},
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDLoopValidate,
				Input:  json.RawMessage(`{"definition":{"contract":{"model_defaults":{}}}}`),
			},
		)

		var toolErr *toolspkg.ToolError
		if !errors.As(err, &toolErr) {
			t.Fatalf("Registry.Call(loop_validate retired key) error = %v, want ToolError", err)
		}
		if toolErr.Code != toolspkg.ErrorCodeInvalidInput ||
			!strings.Contains(toolErr.Error(), "definition.contract.model_defaults") ||
			!strings.Contains(toolErr.Error(), "MIGRATION_GUIDE.md#per-task-runtime-selection") {
			t.Fatalf("tool error = %#v, want retired-key migration guidance", toolErr)
		}
		if validateCalled {
			t.Fatal("ValidateLoop was called after native retired-key preflight failed")
		}
	})

	t.Run("Should distinguish retired criterion model from nested runtime model", func(t *testing.T) {
		t.Parallel()

		valid := json.RawMessage(`{
			"definition": {
				"contract": {
					"runtime_defaults": {"judge": {"model": "judge-model"}},
					"verification": [{"runtime": {"model": "criterion-model"}}]
				}
			}
		}`)
		if path, found := retiredNativeLoopRuntimePath(valid); found {
			t.Fatalf("retiredNativeLoopRuntimePath(valid runtime) = %q, true; want false", path)
		}
		retired := json.RawMessage(
			`{"definition":{"contract":{"verification":[{"model":"gpt-5.4"}]}}}`,
		)
		path, found := retiredNativeLoopRuntimePath(retired)
		if !found || path != "definition.contract.verification[0].model" {
			t.Fatalf("retiredNativeLoopRuntimePath(retired model) = %q, %t", path, found)
		}
	})

	t.Run("Should preserve resolved runtime provenance in native status", func(t *testing.T) {
		t.Parallel()

		var capturedWorkspaceID string
		registry := newDaemonNativeRegistry(t, &daemonNativeToolsDeps{
			Sessions: nativeNetworkTestSessionManager("ws-alpha"),
			Workspaces: apitest.StubWorkspaceService{ResolveFn: func(
				_ context.Context,
				ref string,
			) (workspacepkg.ResolvedWorkspace, error) {
				switch ref {
				case "registry-alpha", "stable-alpha":
					return workspacepkg.ResolvedWorkspace{
						Workspace:   workspacepkg.Workspace{ID: "registry-alpha"},
						WorkspaceID: "stable-alpha",
					}, nil
				default:
					return workspacepkg.ResolvedWorkspace{}, workspacepkg.ErrWorkspaceNotFound
				}
			}},
			Loops: func() core.LoopService {
				return &nativeLoopServiceStub{
					getLoopRunFn: func(
						_ context.Context,
						workspaceID string,
						_ string,
					) (contract.LoopRunResponse, error) {
						capturedWorkspaceID = workspaceID
						return contract.LoopRunResponse{
							Run: contract.LoopRunPayload{ID: "run-1", Status: contract.LoopRunStatusRunning},
							Generations: []contract.LoopGenerationPayload{{
								Generation: 1,
								Outputs: []contract.LoopGenerationOutput{{
									NodeID: "worker",
									Status: "completed",
									ResolvedRuntime: &contract.LoopResolvedRuntime{
										Provider: "openai",
										Model:    "gpt-5.4",
										Source: contract.LoopRuntimeProvenance{
											Provider: "default",
											Model:    "frontmatter",
										},
									},
								}},
							}},
						}, nil
					},
				}
			},
		}, nativeApproveAllPolicyInputs())

		result, err := registry.Call(
			t.Context(),
			toolspkg.Scope{Operator: true, WorkspaceID: "registry-alpha"},
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDLoopStatus,
				Input:  json.RawMessage(`{"run_id":"run-1"}`),
			},
		)
		if err != nil {
			t.Fatalf("Registry.Call(loop_status) error = %v", err)
		}
		if capturedWorkspaceID != "registry-alpha" {
			t.Fatalf("Loop service workspace ID = %q, want registry-alpha", capturedWorkspaceID)
		}
		requireNativeStructuredContains(t, result, []byte(`"resolved_runtime"`))
		requireNativeStructuredContains(t, result, []byte(`"model":"gpt-5.4"`))
		requireNativeStructuredContains(t, result, []byte(`"model":"frontmatter"`))
	})

	t.Run("Should keep loop tools unavailable until service is ready", func(t *testing.T) {
		t.Parallel()

		var loopSvc core.LoopService
		registry := newDaemonNativeRegistry(t, &daemonNativeToolsDeps{
			Sessions:   nativeNetworkTestSessionManager("ws-alpha"),
			Workspaces: nativeNetworkTestWorkspaceService(t),
			Loops:      func() core.LoopService { return loopSvc },
		}, nativeApproveAllPolicyInputs())
		scope := toolspkg.Scope{SessionID: "sess-alpha", WorkspaceID: "ws-alpha"}
		views, err := registry.OperatorProjection(t.Context(), scope)
		if err != nil {
			t.Fatalf("OperatorProjection(before service) error = %v", err)
		}
		readinessToolIDs := nativeToolIDsWithPrefixes(views, "loop_", "goal_")
		loopToolIDs := nativeToolIDsWithPrefixes(views, "loop_")
		if len(readinessToolIDs) == 0 || len(loopToolIDs) == 0 {
			t.Fatal("OperatorProjection(before service) contains no Loop or Goal tools")
		}
		for _, id := range readinessToolIDs {
			requireNativeToolUnavailableReason(t, views, id)
		}

		loopSvc = &nativeLoopServiceStub{
			listLoopsFn: func(context.Context, string, looppkg.CatalogQuery) (contract.LoopsResponse, error) {
				return contract.LoopsResponse{}, nil
			},
		}
		views, err = registry.OperatorProjection(t.Context(), scope)
		if err != nil {
			t.Fatalf("OperatorProjection(after service) error = %v", err)
		}
		for _, id := range loopToolIDs {
			requireNativeToolAvailable(t, views, id)
		}
	})

	t.Run("Should surface shared service self approval denial", func(t *testing.T) {
		t.Parallel()

		approveCalled := false
		registry := newDaemonNativeRegistry(t, &daemonNativeToolsDeps{
			Sessions: nativeNetworkTestSessionManager("ws-alpha"),
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
					`{"workspace":"ws-alpha","run_id":"looprun-1","gate_id":"human",` +
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
			Sessions: nativeNetworkTestSessionManager("ws-alpha"),
			Loops:    func() core.LoopService { return &nativeLoopServiceStub{} },
		}, nativeApproveAllPolicyInputs())

		_, err := registry.Call(
			t.Context(),
			toolspkg.Scope{SessionID: "sess-reviewer"},
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDLoopApprove,
				Input: json.RawMessage(
					`{"workspace":"ws-alpha","run_id":"looprun-1","gate_id":"human","decision":"approve","approval_token_hash":"raw-secret-token"}`,
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
			Sessions:   nativeNetworkTestSessionManager("ws-alpha"),
			Loops:      func() core.LoopService { return loopSvc },
			Workspaces: nativeNetworkTestWorkspaceService(t),
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
			Sessions: nativeNetworkTestSessionManager("ws-goal"),
			Loops:    func() core.LoopService { return loopSvc },
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
			Sessions: nativeNetworkTestSessionManager("ws-goal"),
			Loops:    func() core.LoopService { return loopSvc },
		}, nativeApproveAllPolicyInputs())
		result, err := registry.Call(
			t.Context(),
			toolspkg.Scope{SessionID: "sess-goal", WorkspaceID: "ws-goal"},
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDLoopTurns,
				Input: json.RawMessage(
					`{"run_id":"run-goal","node":"goal","item":1,"after_seq":4,"limit":5}`,
				),
			},
		)
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

func nativeToolIDsWithPrefixes(views []toolspkg.ToolView, prefixes ...string) []toolspkg.ToolID {
	ids := make([]toolspkg.ToolID, 0)
	for index := range views {
		view := &views[index]
		name := strings.TrimPrefix(string(view.Descriptor.ID), "compozy__")
		for _, prefix := range prefixes {
			if strings.HasPrefix(name, prefix) {
				ids = append(ids, view.Descriptor.ID)
				break
			}
		}
	}
	return ids
}

func nativeLoopNodeMutationResponse(
	runID string,
	nodeID string,
	provenance *contract.LoopControlProvenancePayload,
) contract.LoopMutationResponse {
	return contract.LoopMutationResponse{
		OK: true, RunID: runID, NodeID: nodeID, Provenance: provenance,
		Control: &contract.LoopNodeControlPayload{LoopRunID: runID, NodeID: nodeID},
	}
}

func nativeLoopNodeMutationRecorder(
	t *testing.T,
	calls *[]string,
	name string,
	provenance *contract.LoopControlProvenancePayload,
) func(
	context.Context,
	string,
	string,
	string,
	contract.LoopNodeMutationRequest,
	taskpkg.ActorContext,
) (contract.LoopMutationResponse, error) {
	t.Helper()
	return func(
		_ context.Context,
		workspaceID string,
		runID string,
		nodeID string,
		req contract.LoopNodeMutationRequest,
		_ taskpkg.ActorContext,
	) (contract.LoopMutationResponse, error) {
		*calls = append(*calls, name)
		if workspaceID != "ws-alpha" || runID != "run-1" || nodeID != "approval" || req.Reason != "inspect" {
			t.Fatalf("%s request = %s/%s/%s %#v", name, workspaceID, runID, nodeID, req)
		}
		return nativeLoopNodeMutationResponse(runID, nodeID, provenance), nil
	}
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
	cancelLoopRunFn                func(context.Context, string, string) (contract.LoopMutationResponse, error)
	killLoopRunFn                  func(context.Context, string, string) (contract.LoopMutationResponse, error)
	pauseLoopRunFn                 func(context.Context, string, string) error
	resumeLoopRunFn                func(context.Context, string, string) error
	approveLoopRunFn               func(context.Context, string, string, contract.ApproveLoopRunRequest, taskpkg.ActorContext) error
	listLoopRunEventsFn            func(context.Context, string, string, int64) ([]contract.LoopRunEventPayload, error)
	listLoopNodesFn                func(context.Context, string, core.LoopNodeListQuery) (contract.LoopNodeInventoryResponse, error)
	pauseLoopNodeFn                func(context.Context, string, string, string, contract.LoopNodePauseRequest, taskpkg.ActorContext) (contract.LoopMutationResponse, error)
	resumeLoopNodeFn               func(context.Context, string, string, string, contract.LoopNodeResumeRequest, taskpkg.ActorContext) (contract.LoopMutationResponse, error)
	cancelLoopNodeFn               func(context.Context, string, string, string, contract.LoopNodeMutationRequest, taskpkg.ActorContext) (contract.LoopMutationResponse, error)
	killLoopNodeFn                 func(context.Context, string, string, string, contract.LoopNodeMutationRequest, taskpkg.ActorContext) (contract.LoopMutationResponse, error)
	requeueLoopNodeFn              func(context.Context, string, string, string, contract.LoopNodeMutationRequest, taskpkg.ActorContext) (contract.LoopMutationResponse, error)
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

func (s *nativeLoopServiceStub) GetLoopInputDefaults(
	context.Context,
	string,
	string,
	contract.LoopInputDefaultsScope,
) (contract.LoopInputDefaultsResponse, error) {
	return contract.LoopInputDefaultsResponse{}, errors.New("unexpected GetLoopInputDefaults call")
}

func (s *nativeLoopServiceStub) GetLoopInputDefault(
	context.Context,
	string,
	string,
	string,
	contract.LoopInputDefaultsScope,
) (contract.LoopInputDefaultResponse, error) {
	return contract.LoopInputDefaultResponse{}, errors.New("unexpected GetLoopInputDefault call")
}

func (s *nativeLoopServiceStub) PutLoopInputDefaults(
	context.Context,
	string,
	string,
	contract.PutLoopInputDefaultsRequest,
) (contract.LoopInputDefaultsResponse, error) {
	return contract.LoopInputDefaultsResponse{}, errors.New("unexpected PutLoopInputDefaults call")
}

func (s *nativeLoopServiceStub) PutLoopInputDefault(
	context.Context,
	string,
	string,
	string,
	contract.PutLoopInputDefaultRequest,
) (contract.LoopInputDefaultResponse, error) {
	return contract.LoopInputDefaultResponse{}, errors.New("unexpected PutLoopInputDefault call")
}

func (s *nativeLoopServiceStub) DeleteLoopInputDefault(
	context.Context,
	string,
	string,
	string,
	contract.LoopInputDefaultsScope,
) (contract.DeleteLoopInputDefaultResponse, error) {
	return contract.DeleteLoopInputDefaultResponse{}, errors.New("unexpected DeleteLoopInputDefault call")
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

func (s *nativeLoopServiceStub) CancelLoopRun(
	ctx context.Context,
	workspaceID string,
	runID string,
	_ taskpkg.ActorContext,
) (contract.LoopMutationResponse, error) {
	if s.cancelLoopRunFn != nil {
		return s.cancelLoopRunFn(ctx, workspaceID, runID)
	}
	return contract.LoopMutationResponse{}, errors.New("unexpected CancelLoopRun call")
}

func (s *nativeLoopServiceStub) KillLoopRun(
	ctx context.Context,
	workspaceID string,
	runID string,
	_ taskpkg.ActorContext,
) (contract.LoopMutationResponse, error) {
	if s.killLoopRunFn != nil {
		return s.killLoopRunFn(ctx, workspaceID, runID)
	}
	return contract.LoopMutationResponse{}, errors.New("unexpected KillLoopRun call")
}

func (s *nativeLoopServiceStub) ListLoopNodes(
	ctx context.Context,
	workspaceID string,
	query core.LoopNodeListQuery,
) (contract.LoopNodeInventoryResponse, error) {
	if s.listLoopNodesFn != nil {
		return s.listLoopNodesFn(ctx, workspaceID, query)
	}
	return contract.LoopNodeInventoryResponse{}, errors.New("unexpected ListLoopNodes call")
}

func (s *nativeLoopServiceStub) PauseLoopNode(
	ctx context.Context,
	workspaceID string,
	runID string,
	nodeID string,
	req contract.LoopNodePauseRequest,
	actor taskpkg.ActorContext,
) (contract.LoopMutationResponse, error) {
	if s.pauseLoopNodeFn != nil {
		return s.pauseLoopNodeFn(ctx, workspaceID, runID, nodeID, req, actor)
	}
	return contract.LoopMutationResponse{}, errors.New("unexpected PauseLoopNode call")
}

func (s *nativeLoopServiceStub) ResumeLoopNode(
	ctx context.Context,
	workspaceID string,
	runID string,
	nodeID string,
	req contract.LoopNodeResumeRequest,
	actor taskpkg.ActorContext,
) (contract.LoopMutationResponse, error) {
	if s.resumeLoopNodeFn != nil {
		return s.resumeLoopNodeFn(ctx, workspaceID, runID, nodeID, req, actor)
	}
	return contract.LoopMutationResponse{}, errors.New("unexpected ResumeLoopNode call")
}

func (s *nativeLoopServiceStub) CancelLoopNode(
	ctx context.Context,
	workspaceID string,
	runID string,
	nodeID string,
	req contract.LoopNodeMutationRequest,
	actor taskpkg.ActorContext,
) (contract.LoopMutationResponse, error) {
	if s.cancelLoopNodeFn != nil {
		return s.cancelLoopNodeFn(ctx, workspaceID, runID, nodeID, req, actor)
	}
	return contract.LoopMutationResponse{}, errors.New("unexpected CancelLoopNode call")
}

func (s *nativeLoopServiceStub) KillLoopNode(
	ctx context.Context,
	workspaceID string,
	runID string,
	nodeID string,
	req contract.LoopNodeMutationRequest,
	actor taskpkg.ActorContext,
) (contract.LoopMutationResponse, error) {
	if s.killLoopNodeFn != nil {
		return s.killLoopNodeFn(ctx, workspaceID, runID, nodeID, req, actor)
	}
	return contract.LoopMutationResponse{}, errors.New("unexpected KillLoopNode call")
}

func (s *nativeLoopServiceStub) RequeueLoopNode(
	ctx context.Context,
	workspaceID string,
	runID string,
	nodeID string,
	req contract.LoopNodeMutationRequest,
	actor taskpkg.ActorContext,
) (contract.LoopMutationResponse, error) {
	if s.requeueLoopNodeFn != nil {
		return s.requeueLoopNodeFn(ctx, workspaceID, runID, nodeID, req, actor)
	}
	return contract.LoopMutationResponse{}, errors.New("unexpected RequeueLoopNode call")
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
