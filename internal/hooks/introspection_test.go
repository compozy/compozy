package hooks

import (
	"context"
	"testing"
)

func TestHooksCatalogFiltersByWorkspaceAndAgent(t *testing.T) {
	t.Parallel()

	t.Run("Should filter catalog entries by workspace and agent", func(t *testing.T) {
		t.Parallel()

		readOnly := true
		hooks := newTestHooks(
			t,
			WithConfigDeclarations([]HookDecl{
				{
					Name:  "matching-session",
					Event: HookSessionPostCreate,
					Mode:  HookModeSync,
					Matcher: HookMatcher{
						AgentName:     "coder",
						WorkspaceID:   "ws-alpha",
						WorkspaceRoot: "/workspace/alpha",
					},
					Command:  "/bin/sh",
					Args:     []string{"-c", "printf '{}'"},
					Metadata: map[string]string{"origin": "test"},
				},
				{
					Name:  "tool-hook",
					Event: HookToolPreCall,
					Mode:  HookModeSync,
					Matcher: HookMatcher{
						ToolReadOnly: &readOnly,
					},
					Command: "/bin/sh",
					Args:    []string{"-c", "printf '{}'"},
				},
				{
					Name:  "other-workspace",
					Event: HookSessionPostCreate,
					Mode:  HookModeSync,
					Matcher: HookMatcher{
						WorkspaceID: "ws-beta",
					},
					Command: "/bin/sh",
					Args:    []string{"-c", "printf '{}'"},
				},
			}),
		)

		if err := hooks.Rebuild(t.Context()); err != nil {
			t.Fatalf("Rebuild() error = %v", err)
		}

		entries, err := hooks.Catalog(CatalogFilter{
			AgentName:     "coder",
			WorkspaceID:   "ws-alpha",
			WorkspaceRoot: "/workspace/alpha",
		})
		if err != nil {
			t.Fatalf("Catalog() error = %v", err)
		}
		if got, want := len(entries), 2; got != want {
			t.Fatalf("len(entries) = %d, want %d", got, want)
		}
		if entries[0].Name != "matching-session" {
			t.Fatalf("entries[0].Name = %q, want matching-session", entries[0].Name)
		}
		if entries[0].Metadata["origin"] != "test" {
			t.Fatalf("entries[0].Metadata = %#v", entries[0].Metadata)
		}
		if entries[1].Name != "tool-hook" {
			t.Fatalf("entries[1].Name = %q, want tool-hook", entries[1].Name)
		}
		if entries[1].Matcher.ToolReadOnly == nil || !*entries[1].Matcher.ToolReadOnly {
			t.Fatalf("entries[1].Matcher.ToolReadOnly = %#v, want true", entries[1].Matcher.ToolReadOnly)
		}
	})
}

func TestAllEventDescriptorsReturnsFullTaxonomy(t *testing.T) {
	t.Parallel()

	descriptors := AllEventDescriptors()
	if got, want := len(descriptors), len(AllHookEvents()); got != want {
		t.Fatalf("len(descriptors) = %d, want %d", got, want)
	}

	byEvent := make(map[HookEvent]EventDescriptor, len(descriptors))
	for _, descriptor := range descriptors {
		byEvent[descriptor.Event] = descriptor
	}
	if descriptor := byEvent[HookMessageDelta]; descriptor.SyncEligible {
		t.Fatalf("message.delta SyncEligible = true, want false")
	}
	if descriptor := byEvent[HookPermissionRequest]; !descriptor.SyncEligible {
		t.Fatalf("permission.request SyncEligible = false, want true")
	}
	if descriptor := byEvent[HookAutomationJobPreFire]; descriptor.Family != HookEventFamilyAutomation ||
		!descriptor.SyncEligible ||
		descriptor.PatchSchema != "AutomationFirePatch" {
		t.Fatalf("automation.job.pre_fire descriptor = %#v, want automation sync fire patch schema", descriptor)
	}
	if descriptor := byEvent[HookAutomationRunFailed]; descriptor.Family != HookEventFamilyAutomation ||
		descriptor.SyncEligible ||
		descriptor.PatchSchema != "AutomationObservationPatch" {
		t.Fatalf("automation.run.failed descriptor = %#v, want automation async observation schema", descriptor)
	}
	if descriptor := byEvent[HookSandboxPrepare]; descriptor.Family != HookEventFamilySandbox ||
		!descriptor.SyncEligible ||
		descriptor.PayloadSchema != "SandboxPreparePayload" ||
		descriptor.PatchSchema != "SandboxPreparePatch" {
		t.Fatalf("sandbox.prepare descriptor = %#v, want sync sandbox prepare descriptor", descriptor)
	}
	if descriptor := byEvent[HookSandboxSyncAfter]; descriptor.Family != HookEventFamilySandbox ||
		descriptor.SyncEligible ||
		descriptor.PayloadSchema != "SandboxSyncAfterPayload" ||
		descriptor.PatchSchema != "SandboxSyncAfterPatch" {
		t.Fatalf("sandbox.sync.after descriptor = %#v, want async sync-after descriptor", descriptor)
	}
	autonomyDescriptors := map[HookEvent]struct {
		family       HookEventFamily
		payload      string
		patch        string
		syncEligible bool
	}{
		HookCoordinatorPreSpawn: {
			HookEventFamilyCoordinator,
			"CoordinatorPreSpawnPayload",
			"CoordinatorSpawnPatch",
			true,
		},
		HookCoordinatorSpawned: {
			HookEventFamilyCoordinator,
			"CoordinatorSpawnedPayload",
			"CoordinatorObservationPatch",
			true,
		},
		HookCoordinatorDecision: {
			HookEventFamilyCoordinator,
			"CoordinatorDecisionPayload",
			"CoordinatorObservationPatch",
			true,
		},
		HookCoordinatorStopped: {
			HookEventFamilyCoordinator,
			"CoordinatorStoppedPayload",
			"CoordinatorObservationPatch",
			true,
		},
		HookCoordinatorFailed: {
			HookEventFamilyCoordinator,
			"CoordinatorFailedPayload",
			"CoordinatorObservationPatch",
			true,
		},
		HookTaskBlocked:        {HookEventFamilyTask, "TaskBlockedPayload", "TaskObservationPatch", true},
		HookTaskUnblocked:      {HookEventFamilyTask, "TaskUnblockedPayload", "TaskObservationPatch", true},
		HookTaskNeedsAttention: {HookEventFamilyTask, "TaskNeedsAttentionPayload", "TaskObservationPatch", true},
		HookTaskRecovered:      {HookEventFamilyTask, "TaskRecoveredPayload", "TaskObservationPatch", true},
		HookTaskStatusChanged:  {HookEventFamilyTask, "TaskStatusChangedPayload", "TaskObservationPatch", false},
		HookTaskRunEnqueued:    {HookEventFamilyTaskRun, "TaskRunEnqueuedPayload", "TaskRunObservationPatch", true},
		HookTaskRunPreClaim:    {HookEventFamilyTaskRun, "TaskRunPreClaimPayload", "TaskRunPreClaimPatch", true},
		HookTaskRunPostClaim:   {HookEventFamilyTaskRun, "TaskRunPostClaimPayload", "TaskRunObservationPatch", true},
		HookTaskRunLeaseExtended: {
			HookEventFamilyTaskRun,
			"TaskRunLeaseExtendedPayload",
			"TaskRunObservationPatch",
			true,
		},
		HookTaskRunLeaseExpired: {
			HookEventFamilyTaskRun,
			"TaskRunLeaseExpiredPayload",
			"TaskRunObservationPatch",
			true,
		},
		HookTaskRunLeaseRecovered: {
			HookEventFamilyTaskRun,
			"TaskRunLeaseRecoveredPayload",
			"TaskRunObservationPatch",
			true,
		},
		HookTaskRunReleased:    {HookEventFamilyTaskRun, "TaskRunReleasedPayload", "TaskRunObservationPatch", true},
		HookTaskRunCompleted:   {HookEventFamilyTaskRun, "TaskRunCompletedPayload", "TaskRunObservationPatch", true},
		HookTaskRunFailed:      {HookEventFamilyTaskRun, "TaskRunFailedPayload", "TaskRunObservationPatch", true},
		HookSpawnPreCreate:     {HookEventFamilySpawn, "SpawnPreCreatePayload", "SpawnCreatePatch", true},
		HookSpawnCreated:       {HookEventFamilySpawn, "SpawnCreatedPayload", "SpawnObservationPatch", true},
		HookSpawnParentStopped: {HookEventFamilySpawn, "SpawnParentStoppedPayload", "SpawnObservationPatch", true},
		HookSpawnTTLExpired:    {HookEventFamilySpawn, "SpawnTTLExpiredPayload", "SpawnObservationPatch", true},
		HookSpawnReaped:        {HookEventFamilySpawn, "SpawnReapedPayload", "SpawnObservationPatch", true},
	}
	for event, want := range autonomyDescriptors {
		t.Run("Should describe "+string(event), func(t *testing.T) {
			t.Parallel()

			descriptor := byEvent[event]
			if descriptor.Family != want.family ||
				descriptor.SyncEligible != want.syncEligible ||
				descriptor.PayloadSchema != want.payload ||
				descriptor.PatchSchema != want.patch {
				t.Fatalf("%s descriptor = %#v, want family=%q payload=%q patch=%q sync=%v",
					event,
					descriptor,
					want.family,
					want.payload,
					want.patch,
					want.syncEligible,
				)
			}
		})
	}

	networkDescriptors := map[HookEvent]string{
		HookNetworkPeerJoined:       "NetworkPeerJoinedPayload",
		HookNetworkPeerLeft:         "NetworkPeerLeftPayload",
		HookNetworkThreadOpened:     "NetworkThreadOpenedPayload",
		HookNetworkDirectRoomOpened: "NetworkDirectRoomOpenedPayload",
		HookNetworkMessagePersisted: "NetworkMessagePersistedPayload",
		HookNetworkWorkOpened:       "NetworkWorkOpenedPayload",
		HookNetworkWorkTransitioned: "NetworkWorkTransitionedPayload",
		HookNetworkWorkClosed:       "NetworkWorkClosedPayload",
	}
	for event, wantPayload := range networkDescriptors {
		t.Run("Should describe "+string(event), func(t *testing.T) {
			t.Parallel()

			descriptor := byEvent[event]
			if descriptor.Family != HookEventFamilyNetwork ||
				descriptor.SyncEligible ||
				descriptor.PayloadSchema != wantPayload ||
				descriptor.PatchSchema != "NetworkObservationPatch" {
				t.Fatalf("%s descriptor = %#v, want async network payload=%q", event, descriptor, wantPayload)
			}
		})
	}

	windowManagerDescriptors := map[HookEvent]string{
		HookWindowManagerLayoutApplied:  "WindowManagerLayoutAppliedPayload",
		HookWindowManagerDesktopCreated: "WindowManagerDesktopCreatedPayload",
		HookWindowManagerDesktopDeleted: "WindowManagerDesktopDeletedPayload",
		HookWindowManagerWindowMoved:    "WindowManagerWindowMovedPayload",
	}
	for event, wantPayload := range windowManagerDescriptors {
		t.Run("Should describe "+string(event), func(t *testing.T) {
			t.Parallel()

			descriptor := byEvent[event]
			if descriptor.Family != HookEventFamilyWindowManager ||
				descriptor.SyncEligible ||
				descriptor.PayloadSchema != wantPayload ||
				descriptor.PatchSchema != "WindowManagerObservationPatch" {
				t.Fatalf("%s descriptor = %#v, want async window-manager payload=%q", event, descriptor, wantPayload)
			}
		})
	}

	t.Run("Should describe network.participation.pre_resolve as sync", func(t *testing.T) {
		t.Parallel()

		descriptor := byEvent[HookNetworkParticipationPreResolve]
		if descriptor.Family != HookEventFamilyNetwork ||
			!descriptor.SyncEligible ||
			descriptor.PayloadSchema != "NetworkParticipationPreResolvePayload" ||
			descriptor.PatchSchema != "NetworkParticipationPreResolvePatch" {
			t.Fatalf("pre_resolve descriptor = %#v", descriptor)
		}
	})

	t.Run("Should describe network.participation.resolved as async observation", func(t *testing.T) {
		t.Parallel()

		descriptor := byEvent[HookNetworkParticipationResolved]
		if descriptor.Family != HookEventFamilyNetwork ||
			descriptor.SyncEligible ||
			descriptor.PayloadSchema != "NetworkParticipationResolvedPayload" ||
			descriptor.PatchSchema != "NetworkParticipationResolvedPatch" {
			t.Fatalf("resolved descriptor = %#v", descriptor)
		}
	})
}

func TestHooksCatalogFiltersByEventSourceModeAndExposesExecutorKind(t *testing.T) {
	t.Parallel()

	t.Run("Should filter by event source and mode with executor metadata", func(t *testing.T) {
		t.Parallel()

		source := HookSourceConfig
		hooks := newTestHooks(
			t,
			WithNativeDeclarations([]HookDecl{{
				Name:         "native-tool",
				Event:        HookToolPreCall,
				Mode:         HookModeSync,
				ExecutorKind: HookExecutorNative,
			}}),
			WithConfigDeclarations([]HookDecl{
				{
					Name:    "config-tool-sync",
					Event:   HookToolPreCall,
					Mode:    HookModeSync,
					Command: "/bin/sh",
					Args:    []string{"-c", "printf '{}'"},
				},
				{
					Name:    "config-tool-async",
					Event:   HookToolPreCall,
					Mode:    HookModeAsync,
					Command: "/bin/sh",
					Args:    []string{"-c", "printf '{}'"},
				},
				{
					Name:    "config-session-sync",
					Event:   HookSessionPostCreate,
					Mode:    HookModeSync,
					Command: "/bin/sh",
					Args:    []string{"-c", "printf '{}'"},
				},
			}),
			WithExecutorResolver(testExecutorResolver(map[string]Executor{
				"native-tool": NewTypedNativeExecutor(
					func(_ context.Context, _ RegisteredHook, _ ToolPreCallPayload) (ToolCallPatch, error) {
						return ToolCallPatch{}, nil
					},
				),
			})),
		)

		if err := hooks.Rebuild(t.Context()); err != nil {
			t.Fatalf("Rebuild() error = %v", err)
		}

		entries, err := hooks.Catalog(CatalogFilter{
			Event:  HookToolPreCall,
			Source: &source,
			Mode:   HookModeSync,
		})
		if err != nil {
			t.Fatalf("Catalog() error = %v", err)
		}
		if got, want := len(entries), 1; got != want {
			t.Fatalf("len(entries) = %d, want %d", got, want)
		}
		if entries[0].Name != "config-tool-sync" {
			t.Fatalf("entries[0].Name = %q, want config-tool-sync", entries[0].Name)
		}
		if entries[0].ExecutorKind != HookExecutorSubprocess {
			t.Fatalf("entries[0].ExecutorKind = %q, want %q", entries[0].ExecutorKind, HookExecutorSubprocess)
		}

		nativeEntries, err := hooks.Catalog(CatalogFilter{Event: HookToolPreCall})
		if err != nil {
			t.Fatalf("Catalog(native) error = %v", err)
		}
		if got, want := len(nativeEntries), 3; got != want {
			t.Fatalf("len(nativeEntries) = %d, want %d", got, want)
		}
		if nativeEntries[0].Name != "native-tool" || nativeEntries[0].ExecutorKind != HookExecutorNative {
			t.Fatalf("nativeEntries[0] = %#v, want native executor metadata", nativeEntries[0])
		}
	})
}

func TestFilterEventDescriptorsSupportsFamilyAndSyncOnly(t *testing.T) {
	t.Parallel()

	t.Run("Should return only sync-eligible descriptors for the selected family", func(t *testing.T) {
		t.Parallel()

		descriptors := FilterEventDescriptors(EventFilter{
			Family:   HookEventFamilyTool,
			SyncOnly: true,
		})
		if len(descriptors) == 0 {
			t.Fatal("FilterEventDescriptors() returned no tool descriptors")
		}

		for _, descriptor := range descriptors {
			if descriptor.Family != HookEventFamilyTool {
				t.Fatalf("descriptor.Family = %q, want %q", descriptor.Family, HookEventFamilyTool)
			}
			if !descriptor.SyncEligible {
				t.Fatalf("descriptor.SyncEligible = false for %q, want true", descriptor.Event)
			}
		}
	})
}

func TestHookTelemetryHelpersExposeSessionIDAndSink(t *testing.T) {
	t.Parallel()

	t.Run("Should expose telemetry sink writer and session identity", func(t *testing.T) {
		t.Parallel()

		sink := &captureTelemetrySink{}
		hooks := NewHooks(WithTelemetrySink(sink))
		if hooks.telemetrySink != sink {
			t.Fatalf("telemetrySink = %#v, want %#v", hooks.telemetrySink, sink)
		}

		writer := &captureHookRunWriter{}
		ctx := WithHookRunWriter(context.Background(), writer)
		if HookRunWriterFromContext(ctx) != writer {
			t.Fatal("HookRunWriterFromContext() did not return attached writer")
		}

		payload := SessionPostCreatePayload{
			SessionContext: SessionContext{SessionID: "sess-1"},
		}
		if got := sessionIDFromPayload(payload); got != "sess-1" {
			t.Fatalf("sessionIDFromPayload() = %q, want sess-1", got)
		}
	})
}

func TestHooksCatalogAllowsNilExecutorInSnapshot(t *testing.T) {
	t.Parallel()

	t.Run("Should expose entries whose snapshot executor is nil", func(t *testing.T) {
		t.Parallel()

		hooks := &Hooks{
			snapshot: map[HookEvent][]*ResolvedHook{
				HookToolPreCall: {&ResolvedHook{
					RegisteredHook: RegisteredHook{
						Name:     "nil-executor",
						Event:    HookToolPreCall,
						Source:   HookSourceConfig,
						Mode:     HookModeSync,
						Priority: 500,
					},
					Decl: HookDecl{
						Name:  "nil-executor",
						Event: HookToolPreCall,
					},
				}},
			},
		}

		entries, err := hooks.Catalog(CatalogFilter{Event: HookToolPreCall})
		if err != nil {
			t.Fatalf("Catalog() error = %v", err)
		}
		if got, want := len(entries), 1; got != want {
			t.Fatalf("len(entries) = %d, want %d", got, want)
		}
		if entries[0].ExecutorKind != "" {
			t.Fatalf("entries[0].ExecutorKind = %q, want empty string for nil executor", entries[0].ExecutorKind)
		}
	})
}

type captureTelemetrySink struct{}

func (*captureTelemetrySink) WriteHookRecord(context.Context, string, HookRunRecord) error {
	return nil
}
