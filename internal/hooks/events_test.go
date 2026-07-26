package hooks

import "testing"

const expectedHookEventCount = 90

func TestAllHookEvents(t *testing.T) {
	t.Parallel()

	events := AllHookEvents()
	// Assert the exact count so accidental taxonomy additions/removals are caught explicitly.
	if len(events) != expectedHookEventCount {
		t.Fatalf("len(AllHookEvents()) = %d, want %d", len(events), expectedHookEventCount)
	}

	seen := make(map[HookEvent]struct{}, len(events))
	for _, event := range events {
		if event == "" {
			t.Fatal("AllHookEvents() contains an empty event")
		}
		if err := event.Validate(); err != nil {
			t.Fatalf("event.Validate() error = %v", err)
		}
		if _, ok := seen[event]; ok {
			t.Fatalf("AllHookEvents() contains duplicate event %q", event)
		}
		seen[event] = struct{}{}
	}
}

func TestSyncEligibleClassification(t *testing.T) {
	t.Parallel()

	asyncOnly := map[HookEvent]struct{}{
		HookMessageDelta:                 {},
		HookEventPreRecord:               {},
		HookEventPostRecord:              {},
		HookAutomationJobPostFire:        {},
		HookAutomationTriggerPostFire:    {},
		HookAutomationRunCompleted:       {},
		HookAutomationRunFailed:          {},
		HookSandboxReady:                 {},
		HookSandboxSyncAfter:             {},
		HookPermissionResolved:           {},
		HookPermissionDenied:             {},
		HookAgentSoulSnapshotResolved:    {},
		HookAgentSoulMutationAfter:       {},
		HookAgentHeartbeatPolicyResolved: {},
		HookAgentHeartbeatWakeAfter:      {},
		HookSessionHealthUpdateAfter:     {},
		HookNetworkPeerJoined:            {},
		HookNetworkPeerLeft:              {},
		HookNetworkThreadOpened:          {},
		HookNetworkDirectRoomOpened:      {},
		HookNetworkMessagePersisted:      {},
		HookNetworkWorkOpened:            {},
		HookNetworkWorkTransitioned:      {},
		HookNetworkWorkClosed:            {},
		HookNetworkParticipationResolved: {},
		HookSessionMessagePersisted:      {},
		HookTaskStatusChanged:            {},
		HookLoopStarted:                  {},
		HookLoopGenerationPost:           {},
		HookLoopGatePost:                 {},
		HookLoopNodeTerminal:             {},
		HookLoopTerminal:                 {},
		HookWindowManagerLayoutApplied:   {},
		HookWindowManagerDesktopCreated:  {},
		HookWindowManagerDesktopDeleted:  {},
		HookWindowManagerWindowMoved:     {},
	}

	if !HookSessionPreCreate.SyncEligible() {
		t.Fatal("HookSessionPreCreate.SyncEligible() = false, want true")
	}
	if HookMessageDelta.SyncEligible() {
		t.Fatal("HookMessageDelta.SyncEligible() = true, want false")
	}

	for _, event := range AllHookEvents() {
		_, wantAsyncOnly := asyncOnly[event]
		got := event.SyncEligible()
		if wantAsyncOnly && got {
			t.Fatalf("%s.SyncEligible() = true, want false", event)
		}
		if !wantAsyncOnly && !got {
			t.Fatalf("%s.SyncEligible() = false, want true", event)
		}
	}
}

func TestLoopHookEventsHaveExpectedFamiliesAndSyncEligibility(t *testing.T) {
	t.Parallel()

	expected := map[HookEvent]bool{
		HookLoopStarted:        false,
		HookLoopGenerationPre:  true,
		HookLoopGenerationPost: false,
		HookLoopGatePre:        true,
		HookLoopGatePost:       false,
		HookLoopNodeTerminal:   false,
		HookLoopTerminal:       false,
	}
	seen := make(map[HookEvent]struct{}, len(AllHookEvents()))
	for _, event := range AllHookEvents() {
		seen[event] = struct{}{}
	}
	for event, syncEligible := range expected {
		if _, ok := seen[event]; !ok {
			t.Fatalf("AllHookEvents() missing %q", event)
		}
		if got := event.Family(); got != HookEventFamilyLoop {
			t.Fatalf("%s.Family() = %q, want %q", event, got, HookEventFamilyLoop)
		}
		if got := event.SyncEligible(); got != syncEligible {
			t.Fatalf("%s.SyncEligible() = %v, want %v", event, got, syncEligible)
		}
	}
}

func TestHookEventFamilyAndInvalidValidation(t *testing.T) {
	t.Parallel()

	if got := HookToolPostCall.Family(); got != HookEventFamilyTool {
		t.Fatalf("HookToolPostCall.Family() = %q, want %q", got, HookEventFamilyTool)
	}

	var invalid HookEvent = "nope.invalid"
	if got := invalid.Family(); got != "" {
		t.Fatalf("invalid.Family() = %q, want empty string", got)
	}
	if err := invalid.Validate(); err == nil {
		t.Fatal("invalid.Validate() error = nil, want non-nil")
	}
}

func TestNetworkHookEventsHaveExpectedFamiliesAndSyncEligibility(t *testing.T) {
	t.Parallel()

	expected := []HookEvent{
		HookNetworkPeerJoined,
		HookNetworkPeerLeft,
		HookNetworkThreadOpened,
		HookNetworkDirectRoomOpened,
		HookNetworkMessagePersisted,
		HookNetworkWorkOpened,
		HookNetworkWorkTransitioned,
		HookNetworkWorkClosed,
	}
	seen := make(map[HookEvent]struct{}, len(AllHookEvents()))
	for _, event := range AllHookEvents() {
		seen[event] = struct{}{}
	}
	for _, event := range expected {
		if _, ok := seen[event]; !ok {
			t.Fatalf("AllHookEvents() missing %q", event)
		}
		if got := event.Family(); got != HookEventFamilyNetwork {
			t.Fatalf("%s.Family() = %q, want %q", event, got, HookEventFamilyNetwork)
		}
		if event.SyncEligible() {
			t.Fatalf("%s.SyncEligible() = true, want false", event)
		}
	}
}

func TestWindowManagerHookEventsHaveExpectedFamilyAndSyncEligibility(t *testing.T) {
	t.Parallel()

	t.Run("Should classify every window-manager event as async-only", func(t *testing.T) {
		t.Parallel()

		expected := []HookEvent{
			HookWindowManagerLayoutApplied,
			HookWindowManagerDesktopCreated,
			HookWindowManagerDesktopDeleted,
			HookWindowManagerWindowMoved,
		}
		seen := make(map[HookEvent]struct{}, len(AllHookEvents()))
		for _, event := range AllHookEvents() {
			seen[event] = struct{}{}
		}
		for _, event := range expected {
			if _, ok := seen[event]; !ok {
				t.Fatalf("AllHookEvents() missing %q", event)
			}
			if got := event.Family(); got != HookEventFamilyWindowManager {
				t.Fatalf("%s.Family() = %q, want %q", event, got, HookEventFamilyWindowManager)
			}
			if event.SyncEligible() {
				t.Fatalf("%s.SyncEligible() = true, want false", event)
			}
		}
	})
}

func TestAutonomyHookEventsHaveExpectedFamiliesAndSyncEligibility(t *testing.T) {
	t.Parallel()

	expected := map[HookEvent]struct {
		family       HookEventFamily
		syncEligible bool
	}{
		HookCoordinatorPreSpawn:   {HookEventFamilyCoordinator, true},
		HookCoordinatorSpawned:    {HookEventFamilyCoordinator, true},
		HookCoordinatorDecision:   {HookEventFamilyCoordinator, true},
		HookCoordinatorStopped:    {HookEventFamilyCoordinator, true},
		HookCoordinatorFailed:     {HookEventFamilyCoordinator, true},
		HookTaskBlocked:           {HookEventFamilyTask, true},
		HookTaskUnblocked:         {HookEventFamilyTask, true},
		HookTaskNeedsAttention:    {HookEventFamilyTask, true},
		HookTaskRecovered:         {HookEventFamilyTask, true},
		HookTaskStatusChanged:     {HookEventFamilyTask, false},
		HookTaskRunEnqueued:       {HookEventFamilyTaskRun, true},
		HookTaskRunPreClaim:       {HookEventFamilyTaskRun, true},
		HookTaskRunPostClaim:      {HookEventFamilyTaskRun, true},
		HookTaskRunLeaseExtended:  {HookEventFamilyTaskRun, true},
		HookTaskRunLeaseExpired:   {HookEventFamilyTaskRun, true},
		HookTaskRunLeaseRecovered: {HookEventFamilyTaskRun, true},
		HookTaskRunReleased:       {HookEventFamilyTaskRun, true},
		HookTaskRunCompleted:      {HookEventFamilyTaskRun, true},
		HookTaskRunFailed:         {HookEventFamilyTaskRun, true},
		HookSpawnPreCreate:        {HookEventFamilySpawn, true},
		HookSpawnCreated:          {HookEventFamilySpawn, true},
		HookSpawnParentStopped:    {HookEventFamilySpawn, true},
		HookSpawnTTLExpired:       {HookEventFamilySpawn, true},
		HookSpawnReaped:           {HookEventFamilySpawn, true},
	}
	seen := make(map[HookEvent]struct{}, len(AllHookEvents()))
	for _, event := range AllHookEvents() {
		seen[event] = struct{}{}
	}
	for event, want := range expected {
		if _, ok := seen[event]; !ok {
			t.Fatalf("AllHookEvents() missing %q", event)
		}
		if got := event.Family(); got != want.family {
			t.Fatalf("%s.Family() = %q, want %q", event, got, want.family)
		}
		if got := event.SyncEligible(); got != want.syncEligible {
			t.Fatalf("%s.SyncEligible() = %v, want %v", event, got, want.syncEligible)
		}
	}
}

func TestSchedulerObservabilityNamesAreNotHookEvents(t *testing.T) {
	t.Parallel()

	for _, event := range []HookEvent{
		"scheduler.wake",
		"scheduler.no_match",
		"scheduler.recovery",
		"task.run.scheduler_wake",
	} {
		if err := event.Validate(); err == nil {
			t.Fatalf("%q validated as a hook event, want absent from taxonomy", event)
		}
	}
}
