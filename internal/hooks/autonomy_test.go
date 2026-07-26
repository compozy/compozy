package hooks

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/compozy/agh/internal/network/participation"
)

func TestAutonomyPayloadsCarryResolvedNetworkParticipation(t *testing.T) {
	t.Parallel()

	coordinatorPayloads := make(chan CoordinatorPreSpawnPayload, 1)
	taskRunPayloads := make(chan TaskRunEnqueuedPayload, 1)
	hooks := newTestHooks(
		t,
		WithNativeDeclarations([]HookDecl{
			{
				Name:         "coordinator-channel",
				Event:        HookCoordinatorPreSpawn,
				Mode:         HookModeSync,
				ExecutorKind: HookExecutorNative,
			},
			{
				Name:         "task-run-channel",
				Event:        HookTaskRunEnqueued,
				Mode:         HookModeSync,
				ExecutorKind: HookExecutorNative,
			},
		}),
		WithExecutorResolver(testExecutorResolver(map[string]Executor{
			"coordinator-channel": NewTypedNativeExecutor(
				func(
					_ context.Context,
					_ RegisteredHook,
					payload CoordinatorPreSpawnPayload,
				) (CoordinatorSpawnPatch, error) {
					coordinatorPayloads <- payload
					return CoordinatorSpawnPatch{}, nil
				},
			),
			"task-run-channel": NewTypedNativeExecutor(
				func(
					_ context.Context,
					_ RegisteredHook,
					payload TaskRunEnqueuedPayload,
				) (TaskRunObservationPatch, error) {
					taskRunPayloads <- payload
					return TaskRunObservationPatch{}, nil
				},
			),
		})),
	)
	if err := hooks.Rebuild(t.Context()); err != nil {
		t.Fatalf("Rebuild() error = %v", err)
	}

	channelID := "coord-ch-1"
	if _, err := hooks.DispatchCoordinatorPreSpawn(t.Context(), CoordinatorPreSpawnPayload{
		PayloadBase: PayloadBase{Event: HookCoordinatorPreSpawn, Timestamp: time.Now().UTC()},
		CoordinatorContext: CoordinatorContext{
			WorkspaceID: "ws-1",
			ResolvedNetworkParticipation: participation.CloneSpec(participation.Spec{
				ChannelID: channelID,
			}),
		},
	}); err != nil {
		t.Fatalf("DispatchCoordinatorPreSpawn() error = %v", err)
	}
	if _, err := hooks.DispatchTaskRunEnqueued(t.Context(), TaskRunEnqueuedPayload{
		PayloadBase: PayloadBase{Event: HookTaskRunEnqueued, Timestamp: time.Now().UTC()},
		TaskRunContext: TaskRunContext{
			TaskID:      "task-1",
			RunID:       "run-1",
			WorkspaceID: "ws-1",
			ResolvedNetworkParticipation: participation.CloneSpec(participation.Spec{
				ChannelID: channelID,
			}),
		},
	}); err != nil {
		t.Fatalf("DispatchTaskRunEnqueued() error = %v", err)
	}

	assertCoordinatorChannelPayload(t, coordinatorPayloads, channelID)
	assertTaskRunChannelPayload(t, taskRunPayloads, channelID)
}

func TestTaskRunPreClaimDenyAndNarrowCriteriaOnly(t *testing.T) {
	t.Parallel()

	t.Run("Should denies claim", func(t *testing.T) {
		t.Parallel()

		hooks := newTestHooks(
			t,
			WithNativeDeclarations([]HookDecl{{
				Name:         "deny-claim",
				Event:        HookTaskRunPreClaim,
				Mode:         HookModeSync,
				ExecutorKind: HookExecutorNative,
			}}),
			WithExecutorResolver(testExecutorResolver(map[string]Executor{
				"deny-claim": NewTypedNativeExecutor(
					func(
						_ context.Context,
						_ RegisteredHook,
						_ TaskRunPreClaimPayload,
					) (TaskRunPreClaimPatch, error) {
						return TaskRunPreClaimPatch{
							ControlPatch: ControlPatch{
								Deny:       true,
								DenyReason: "capacity gate",
							},
						}, nil
					},
				),
			})),
		)
		if err := hooks.Rebuild(t.Context()); err != nil {
			t.Fatalf("Rebuild() error = %v", err)
		}

		result, err := hooks.DispatchTaskRunPreClaim(t.Context(), baseTaskRunPreClaimPayload())
		if err == nil || !strings.Contains(err.Error(), string(HookTaskRunPreClaim)) {
			t.Fatalf("DispatchTaskRunPreClaim() error = %v, want denial error", err)
		}
		if !result.Denied || result.DenyReason != "capacity gate" {
			t.Fatalf("result denial = denied:%v reason:%q, want hook denial", result.Denied, result.DenyReason)
		}
	})

	t.Run("Should narrows claim criteria", func(t *testing.T) {
		t.Parallel()

		priorityMin := 30
		hooks := newTestHooks(
			t,
			WithNativeDeclarations([]HookDecl{{
				Name:         "narrow-claim",
				Event:        HookTaskRunPreClaim,
				Mode:         HookModeSync,
				ExecutorKind: HookExecutorNative,
			}}),
			WithExecutorResolver(testExecutorResolver(map[string]Executor{
				"narrow-claim": NewTypedNativeExecutor(
					func(
						_ context.Context,
						_ RegisteredHook,
						_ TaskRunPreClaimPayload,
					) (TaskRunPreClaimPatch, error) {
						return TaskRunPreClaimPatch{
							AddRequiredCapabilities: []string{"review", "gpu"},
							PriorityMin:             &priorityMin,
						}, nil
					},
				),
			})),
		)
		if err := hooks.Rebuild(t.Context()); err != nil {
			t.Fatalf("Rebuild() error = %v", err)
		}

		result, err := hooks.DispatchTaskRunPreClaim(t.Context(), baseTaskRunPreClaimPayload())
		if err != nil {
			t.Fatalf("DispatchTaskRunPreClaim() error = %v", err)
		}
		if got, want := result.Criteria.PriorityMin, priorityMin; got != want {
			t.Fatalf("PriorityMin = %d, want %d", got, want)
		}
		wantCaps := []string{"code", "review", "gpu"}
		if !reflect.DeepEqual(result.Criteria.RequiredCapabilities, wantCaps) {
			t.Fatalf("RequiredCapabilities = %#v, want %#v", result.Criteria.RequiredCapabilities, wantCaps)
		}
	})

	t.Run("Should rejects broadening criteria patch", func(t *testing.T) {
		t.Parallel()

		priorityMin := 10
		hooks := newTestHooks(
			t,
			WithNativeDeclarations([]HookDecl{{
				Name:         "broaden-claim",
				Event:        HookTaskRunPreClaim,
				Mode:         HookModeSync,
				ExecutorKind: HookExecutorNative,
			}}),
			WithExecutorResolver(testExecutorResolver(map[string]Executor{
				"broaden-claim": NewTypedNativeExecutor(
					func(
						_ context.Context,
						_ RegisteredHook,
						_ TaskRunPreClaimPayload,
					) (TaskRunPreClaimPatch, error) {
						return TaskRunPreClaimPatch{
							AddRequiredCapabilities: []string{" "},
							PriorityMin:             &priorityMin,
						}, nil
					},
				),
			})),
		)
		if err := hooks.Rebuild(t.Context()); err != nil {
			t.Fatalf("Rebuild() error = %v", err)
		}

		original := baseTaskRunPreClaimPayload()
		result, err := hooks.DispatchTaskRunPreClaim(t.Context(), original)
		if err != nil {
			t.Fatalf("DispatchTaskRunPreClaim() error = %v", err)
		}
		if !reflect.DeepEqual(result.Criteria, original.Criteria) {
			t.Fatalf("result.Criteria = %#v, want unchanged %#v", result.Criteria, original.Criteria)
		}
	})
}

func TestLoopControlHooksDenyAndFailOpen(t *testing.T) {
	t.Parallel()

	t.Run("Should deny generation pre as failed", func(t *testing.T) {
		t.Parallel()

		hooks := newTestHooks(
			t,
			WithNativeDeclarations([]HookDecl{{
				Name:         "deny-generation",
				Event:        HookLoopGenerationPre,
				Mode:         HookModeSync,
				ExecutorKind: HookExecutorNative,
			}}),
			WithExecutorResolver(testExecutorResolver(map[string]Executor{
				"deny-generation": NewTypedNativeExecutor(
					func(
						_ context.Context,
						_ RegisteredHook,
						_ LoopGenerationPrePayload,
					) (LoopGenerationPrePatch, error) {
						return LoopGenerationPrePatch{
							ControlPatch: ControlPatch{
								Deny:       true,
								DenyReason: "capacity gate",
							},
						}, nil
					},
				),
			})),
		)
		if err := hooks.Rebuild(t.Context()); err != nil {
			t.Fatalf("Rebuild() error = %v", err)
		}

		result, err := hooks.DispatchLoopGenerationPre(t.Context(), baseLoopGenerationPayload())
		if err == nil || !strings.Contains(err.Error(), string(HookLoopGenerationPre)) {
			t.Fatalf("DispatchLoopGenerationPre() error = %v, want denial error", err)
		}
		if !result.Denied || result.DenyReason != "capacity gate" {
			t.Fatalf("result denial = denied:%v reason:%q, want generation denial", result.Denied, result.DenyReason)
		}
	})

	t.Run("Should deny gate pre as blocked", func(t *testing.T) {
		t.Parallel()

		hooks := newTestHooks(
			t,
			WithNativeDeclarations([]HookDecl{{
				Name:         "deny-gate",
				Event:        HookLoopGatePre,
				Mode:         HookModeSync,
				ExecutorKind: HookExecutorNative,
			}}),
			WithExecutorResolver(testExecutorResolver(map[string]Executor{
				"deny-gate": NewTypedNativeExecutor(
					func(
						_ context.Context,
						_ RegisteredHook,
						_ LoopGatePrePayload,
					) (LoopGatePrePatch, error) {
						return LoopGatePrePatch{
							ControlPatch: ControlPatch{
								Deny:       true,
								DenyReason: "policy",
							},
						}, nil
					},
				),
			})),
		)
		if err := hooks.Rebuild(t.Context()); err != nil {
			t.Fatalf("Rebuild() error = %v", err)
		}

		result, err := hooks.DispatchLoopGatePre(t.Context(), baseLoopGatePayload())
		if err == nil || !strings.Contains(err.Error(), string(HookLoopGatePre)) {
			t.Fatalf("DispatchLoopGatePre() error = %v, want denial error", err)
		}
		if !result.Denied || result.DenyReason != "policy" {
			t.Fatalf("result denial = denied:%v reason:%q, want gate denial", result.Denied, result.DenyReason)
		}
	})

	t.Run("Should fail open for non-required broken loop hook", func(t *testing.T) {
		t.Parallel()

		ran := false
		hooks := newTestHooks(
			t,
			WithNativeDeclarations([]HookDecl{
				{
					Name:         "broken",
					Event:        HookLoopGenerationPre,
					Mode:         HookModeSync,
					ExecutorKind: HookExecutorNative,
					Priority:     10,
					PrioritySet:  true,
				},
				{
					Name:         "recorder",
					Event:        HookLoopGenerationPre,
					Mode:         HookModeSync,
					ExecutorKind: HookExecutorNative,
					Priority:     20,
					PrioritySet:  true,
				},
			}),
			WithExecutorResolver(testExecutorResolver(map[string]Executor{
				"broken": NewTypedNativeExecutor(
					func(
						_ context.Context,
						_ RegisteredHook,
						_ LoopGenerationPrePayload,
					) (LoopGenerationPrePatch, error) {
						return LoopGenerationPrePatch{}, errors.New("hook failed")
					},
				),
				"recorder": NewTypedNativeExecutor(
					func(
						_ context.Context,
						_ RegisteredHook,
						payload LoopGenerationPrePayload,
					) (LoopGenerationPrePatch, error) {
						if payload.LoopRunID == "loop-run-1" {
							ran = true
						}
						return LoopGenerationPrePatch{}, nil
					},
				),
			})),
		)
		if err := hooks.Rebuild(t.Context()); err != nil {
			t.Fatalf("Rebuild() error = %v", err)
		}

		if _, err := hooks.DispatchLoopGenerationPre(t.Context(), baseLoopGenerationPayload()); err != nil {
			t.Fatalf("DispatchLoopGenerationPre() error = %v, want fail-open nil", err)
		}
		if !ran {
			t.Fatal("recorder hook did not run after broken non-required hook")
		}
	})
}

func TestSpawnPreCreatePatchRejectsPermissionWidening(t *testing.T) {
	t.Parallel()

	hooks := newTestHooks(
		t,
		WithNativeDeclarations([]HookDecl{{
			Name:         "widen-spawn",
			Event:        HookSpawnPreCreate,
			Mode:         HookModeSync,
			ExecutorKind: HookExecutorNative,
		}}),
		WithExecutorResolver(testExecutorResolver(map[string]Executor{
			"widen-spawn": NewTypedNativeExecutor(
				func(
					_ context.Context,
					_ RegisteredHook,
					_ SpawnPreCreatePayload,
				) (SpawnCreatePatch, error) {
					return SpawnCreatePatch{
						ChildPermissions: &PermissionSet{
							Tools:           []string{"read", "write"},
							NetworkChannels: []string{"coord-main", "coord-other"},
						},
					}, nil
				},
			),
		})),
	)
	if err := hooks.Rebuild(t.Context()); err != nil {
		t.Fatalf("Rebuild() error = %v", err)
	}

	original := baseSpawnPreCreatePayload()
	result, err := hooks.DispatchSpawnPreCreate(t.Context(), original)
	if err != nil {
		t.Fatalf("DispatchSpawnPreCreate() error = %v", err)
	}
	if !reflect.DeepEqual(result.ChildPermissions, original.ChildPermissions) {
		t.Fatalf("ChildPermissions = %#v, want unchanged %#v", result.ChildPermissions, original.ChildPermissions)
	}
}

func TestSpawnPreCreateAllowsPermissionNarrowing(t *testing.T) {
	t.Parallel()

	ttlSeconds := int64(1800)
	role := "reviewer"
	hooks := newTestHooks(
		t,
		WithNativeDeclarations([]HookDecl{{
			Name:         "narrow-spawn",
			Event:        HookSpawnPreCreate,
			Mode:         HookModeSync,
			ExecutorKind: HookExecutorNative,
		}}),
		WithExecutorResolver(testExecutorResolver(map[string]Executor{
			"narrow-spawn": NewTypedNativeExecutor(
				func(
					_ context.Context,
					_ RegisteredHook,
					_ SpawnPreCreatePayload,
				) (SpawnCreatePatch, error) {
					return SpawnCreatePatch{
						SpawnRole:  &role,
						TTLSeconds: &ttlSeconds,
						ChildPermissions: &PermissionSet{
							Tools:           []string{"read"},
							NetworkChannels: []string{"coord-main"},
						},
					}, nil
				},
			),
		})),
	)
	if err := hooks.Rebuild(t.Context()); err != nil {
		t.Fatalf("Rebuild() error = %v", err)
	}

	result, err := hooks.DispatchSpawnPreCreate(t.Context(), baseSpawnPreCreatePayload())
	if err != nil {
		t.Fatalf("DispatchSpawnPreCreate() error = %v", err)
	}
	if got, want := result.SpawnRole, role; got != want {
		t.Fatalf("SpawnRole = %q, want %q", got, want)
	}
	if got, want := result.TTLSeconds, ttlSeconds; got != want {
		t.Fatalf("TTLSeconds = %d, want %d", got, want)
	}
	wantPermissions := PermissionSet{
		Tools:           []string{"read"},
		NetworkChannels: []string{"coord-main"},
	}
	if result.ChildPermissions == nil || !reflect.DeepEqual(*result.ChildPermissions, wantPermissions) {
		t.Fatalf("ChildPermissions = %#v, want %#v", result.ChildPermissions, wantPermissions)
	}
}

func TestAutonomyObservationDispatchMethodsNoop(t *testing.T) {
	t.Parallel()

	hooks := newTestHooks(t)
	now := time.Now().UTC()
	coordinator := CoordinatorLifecyclePayload{
		PayloadBase: PayloadBase{Timestamp: now},
		CoordinatorContext: CoordinatorContext{
			WorkspaceID:          "ws-1",
			CoordinatorSessionID: "coord-sess-1",
			ResolvedNetworkParticipation: participation.CloneSpec(participation.Spec{
				ChannelID: "coord-ch-1",
			}),
		},
	}
	taskRunLease := TaskRunLeasePayload{
		PayloadBase: PayloadBase{Timestamp: now},
		TaskRunContext: TaskRunContext{
			TaskID:      "task-1",
			RunID:       "run-1",
			WorkspaceID: "ws-1",
			ResolvedNetworkParticipation: participation.CloneSpec(participation.Spec{
				ChannelID: "coord-ch-1",
			}),
		},
	}
	spawn := SpawnLifecyclePayload{
		PayloadBase: PayloadBase{Timestamp: now},
		SpawnContext: SpawnContext{
			ParentSessionID: "parent-1",
			ChildSessionID:  "child-1",
			WorkspaceID:     "ws-1",
			ResolvedNetworkParticipation: participation.CloneSpec(participation.Spec{
				ChannelID: "coord-ch-1",
			}),
		},
	}

	if _, err := hooks.DispatchCoordinatorSpawned(
		t.Context(),
		withCoordinatorEvent(coordinator, HookCoordinatorSpawned),
	); err != nil {
		t.Fatalf("DispatchCoordinatorSpawned() error = %v", err)
	}
	if _, err := hooks.DispatchCoordinatorDecision(
		t.Context(),
		withCoordinatorEvent(coordinator, HookCoordinatorDecision),
	); err != nil {
		t.Fatalf("DispatchCoordinatorDecision() error = %v", err)
	}
	if _, err := hooks.DispatchCoordinatorStopped(
		t.Context(),
		withCoordinatorEvent(coordinator, HookCoordinatorStopped),
	); err != nil {
		t.Fatalf("DispatchCoordinatorStopped() error = %v", err)
	}
	if _, err := hooks.DispatchCoordinatorFailed(
		t.Context(),
		withCoordinatorEvent(coordinator, HookCoordinatorFailed),
	); err != nil {
		t.Fatalf("DispatchCoordinatorFailed() error = %v", err)
	}

	if _, err := hooks.DispatchTaskRunPostClaim(t.Context(), TaskRunPostClaimPayload{
		PayloadBase:    PayloadBase{Event: HookTaskRunPostClaim, Timestamp: now},
		TaskRunContext: taskRunLease.TaskRunContext,
		ClaimedAt:      now,
	}); err != nil {
		t.Fatalf("DispatchTaskRunPostClaim() error = %v", err)
	}
	if _, err := hooks.DispatchTaskRunLeaseExtended(
		t.Context(),
		withTaskRunLeaseEvent(taskRunLease, HookTaskRunLeaseExtended),
	); err != nil {
		t.Fatalf("DispatchTaskRunLeaseExtended() error = %v", err)
	}
	if _, err := hooks.DispatchTaskRunLeaseExpired(
		t.Context(),
		withTaskRunLeaseEvent(taskRunLease, HookTaskRunLeaseExpired),
	); err != nil {
		t.Fatalf("DispatchTaskRunLeaseExpired() error = %v", err)
	}
	if _, err := hooks.DispatchTaskRunLeaseRecovered(
		t.Context(),
		withTaskRunLeaseEvent(taskRunLease, HookTaskRunLeaseRecovered),
	); err != nil {
		t.Fatalf("DispatchTaskRunLeaseRecovered() error = %v", err)
	}
	if _, err := hooks.DispatchTaskRunReleased(
		t.Context(),
		withTaskRunLeaseEvent(taskRunLease, HookTaskRunReleased),
	); err != nil {
		t.Fatalf("DispatchTaskRunReleased() error = %v", err)
	}

	if _, err := hooks.DispatchSpawnCreated(t.Context(), withSpawnEvent(spawn, HookSpawnCreated)); err != nil {
		t.Fatalf("DispatchSpawnCreated() error = %v", err)
	}
	if _, err := hooks.DispatchSpawnParentStopped(
		t.Context(),
		withSpawnEvent(spawn, HookSpawnParentStopped),
	); err != nil {
		t.Fatalf("DispatchSpawnParentStopped() error = %v", err)
	}
	if _, err := hooks.DispatchSpawnTTLExpired(t.Context(), withSpawnEvent(spawn, HookSpawnTTLExpired)); err != nil {
		t.Fatalf("DispatchSpawnTTLExpired() error = %v", err)
	}
	if _, err := hooks.DispatchSpawnReaped(t.Context(), withSpawnEvent(spawn, HookSpawnReaped)); err != nil {
		t.Fatalf("DispatchSpawnReaped() error = %v", err)
	}
}

func baseTaskRunPreClaimPayload() TaskRunPreClaimPayload {
	return TaskRunPreClaimPayload{
		PayloadBase: PayloadBase{Event: HookTaskRunPreClaim, Timestamp: time.Now().UTC()},
		TaskRunContext: &TaskRunContext{
			TaskID:      "task-1",
			RunID:       "run-1",
			WorkspaceID: "ws-1",
			ResolvedNetworkParticipation: participation.CloneSpec(participation.Spec{
				ChannelID: "coord-ch-1",
			}),
		},
		Criteria: TaskRunClaimCriteria{
			WorkspaceID:          "ws-1",
			RequiredCapabilities: []string{"code"},
			PriorityMin:          20,
		},
	}
}

func baseLoopGenerationPayload() LoopGenerationPrePayload {
	return LoopGenerationPrePayload{
		PayloadBase: PayloadBase{Event: HookLoopGenerationPre, Timestamp: time.Now().UTC()},
		LoopContext: LoopContext{
			LoopRunID:   "loop-run-1",
			WorkspaceID: "ws-1",
			LoopName:    "daily-review",
			Generation:  1,
			TaskID:      "task-1",
			RunID:       "run-1",
			ResolvedNetworkParticipation: participation.CloneSpec(participation.Spec{
				ChannelID: "coord-ch-1",
			}),
		},
		Status: "running",
	}
}

func baseLoopGatePayload() LoopGatePrePayload {
	return LoopGatePrePayload{
		PayloadBase: PayloadBase{Event: HookLoopGatePre, Timestamp: time.Now().UTC()},
		LoopContext: LoopContext{
			LoopRunID:   "loop-run-1",
			WorkspaceID: "ws-1",
			LoopName:    "daily-review",
			Generation:  1,
			TaskID:      "task-1",
			RunID:       "run-1",
			ResolvedNetworkParticipation: participation.CloneSpec(participation.Spec{
				ChannelID: "coord-ch-1",
			}),
		},
		GateID: "contract",
		Status: "done",
	}
}

func baseSpawnPreCreatePayload() SpawnPreCreatePayload {
	parentPermissions := PermissionSet{
		Tools:           []string{"read", "write"},
		NetworkChannels: []string{"coord-main"},
	}
	childPermissions := parentPermissions
	return SpawnPreCreatePayload{
		PayloadBase: PayloadBase{Event: HookSpawnPreCreate, Timestamp: time.Now().UTC()},
		SpawnContext: SpawnContext{
			ParentSessionID: "parent-1",
			RootSessionID:   "root-1",
			WorkspaceID:     "ws-1",
			AgentName:       "worker",
			SpawnRole:       "coder",
			TTLSeconds:      3600,
			ResolvedNetworkParticipation: participation.CloneSpec(participation.Spec{
				ChannelID: "coord-ch-1",
			}),
		},
		ParentPermissions: &parentPermissions,
		ChildPermissions:  &childPermissions,
	}
}

func withCoordinatorEvent(payload CoordinatorLifecyclePayload, event HookEvent) CoordinatorLifecyclePayload {
	payload.Event = event
	return payload
}

func withTaskRunLeaseEvent(payload TaskRunLeasePayload, event HookEvent) TaskRunLeasePayload {
	payload.Event = event
	return payload
}

func withSpawnEvent(payload SpawnLifecyclePayload, event HookEvent) SpawnLifecyclePayload {
	payload.Event = event
	return payload
}

func assertCoordinatorChannelPayload(
	t *testing.T,
	payloads <-chan CoordinatorPreSpawnPayload,
	wantChannelID string,
) {
	t.Helper()

	select {
	case payload := <-payloads:
		if payload.ResolvedNetworkParticipation == nil ||
			payload.ResolvedNetworkParticipation.ChannelID != wantChannelID {
			t.Fatalf(
				"Coordinator ResolvedNetworkParticipation = %#v, want channel %q",
				payload.ResolvedNetworkParticipation,
				wantChannelID,
			)
		}
	default:
		t.Fatal("coordinator hook did not receive payload")
	}
}

func assertTaskRunChannelPayload(t *testing.T, payloads <-chan TaskRunEnqueuedPayload, wantChannelID string) {
	t.Helper()

	select {
	case payload := <-payloads:
		if payload.ResolvedNetworkParticipation.ChannelID != wantChannelID {
			t.Fatalf(
				"TaskRun ResolvedNetworkParticipation.ChannelID = %q, want %q",
				payload.ResolvedNetworkParticipation.ChannelID,
				wantChannelID,
			)
		}
	default:
		t.Fatal("task-run hook did not receive payload")
	}
}
