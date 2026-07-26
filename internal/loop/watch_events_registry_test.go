package loop_test

import (
	"reflect"
	"slices"
	"testing"

	"github.com/compozy/agh/internal/hooks"
	"github.com/compozy/agh/internal/loop"
)

const (
	watchEventsTestTaskStream       = "task_events"
	watchEventsTestLoopStream       = "loop_run_events"
	watchEventsTestAutomationStream = "automation_runs"
	watchEventsTestNetworkStream    = "network_timeline_log"
	watchEventsTestObserveStream    = "event_summaries"
	watchEventsTestSessionStream    = "session_events"
	watchEventsTestParticipation    = "resolved_network_participation"

	loopRunEventTestStatusChanged = "status_changed"
	loopRunEventTestNodeSucceeded = "node_succeeded"
	loopRunEventTestNodeFailed    = "node_failed"
)

func TestSupportedWatchEventsShouldExposeSupportedContracts(t *testing.T) {
	t.Parallel()

	t.Run("Should expose exactly the shipped post-state watch event contracts", func(t *testing.T) {
		t.Parallel()

		contracts := loop.SupportedWatchEvents()
		if len(contracts) != 22 {
			t.Fatalf("SupportedWatchEvents() len = %d, want 22", len(contracts))
		}
		expected := map[hooks.HookEvent]struct {
			stream      string
			ledgerTypes []string
		}{
			hooks.HookTaskStatusChanged: {
				stream:      watchEventsTestTaskStream,
				ledgerTypes: []string{string(hooks.HookTaskStatusChanged)},
			},
			hooks.HookTaskBlocked: {
				stream:      watchEventsTestTaskStream,
				ledgerTypes: []string{string(hooks.HookTaskBlocked)},
			},
			hooks.HookTaskUnblocked: {
				stream:      watchEventsTestTaskStream,
				ledgerTypes: []string{string(hooks.HookTaskUnblocked)},
			},
			hooks.HookTaskNeedsAttention: {
				stream:      watchEventsTestTaskStream,
				ledgerTypes: []string{string(hooks.HookTaskNeedsAttention)},
			},
			hooks.HookTaskRecovered: {
				stream:      watchEventsTestTaskStream,
				ledgerTypes: []string{string(hooks.HookTaskRecovered)},
			},
			hooks.HookTaskRunCompleted: {
				stream:      watchEventsTestTaskStream,
				ledgerTypes: []string{string(hooks.HookTaskRunCompleted)},
			},
			hooks.HookTaskRunFailed: {
				stream:      watchEventsTestTaskStream,
				ledgerTypes: []string{string(hooks.HookTaskRunFailed)},
			},
			hooks.HookLoopTerminal: {
				stream:      watchEventsTestLoopStream,
				ledgerTypes: []string{loopRunEventTestStatusChanged},
			},
			hooks.HookLoopNodeTerminal: {
				stream:      watchEventsTestLoopStream,
				ledgerTypes: []string{loopRunEventTestNodeSucceeded, loopRunEventTestNodeFailed},
			},
			hooks.HookAutomationRunCompleted: {
				stream:      watchEventsTestAutomationStream,
				ledgerTypes: []string{string(hooks.HookAutomationRunCompleted)},
			},
			hooks.HookAutomationRunFailed: {
				stream:      watchEventsTestAutomationStream,
				ledgerTypes: []string{string(hooks.HookAutomationRunFailed)},
			},
			hooks.HookNetworkMessagePersisted: {
				stream:      watchEventsTestNetworkStream,
				ledgerTypes: []string{string(hooks.HookNetworkMessagePersisted)},
			},
			hooks.HookNetworkThreadOpened: {
				stream:      watchEventsTestNetworkStream,
				ledgerTypes: []string{string(hooks.HookNetworkThreadOpened)},
			},
			hooks.HookNetworkDirectRoomOpened: {
				stream:      watchEventsTestNetworkStream,
				ledgerTypes: []string{string(hooks.HookNetworkDirectRoomOpened)},
			},
			hooks.HookNetworkWorkOpened: {
				stream:      watchEventsTestNetworkStream,
				ledgerTypes: []string{string(hooks.HookNetworkWorkOpened)},
			},
			hooks.HookNetworkWorkTransitioned: {
				stream:      watchEventsTestNetworkStream,
				ledgerTypes: []string{string(hooks.HookNetworkWorkTransitioned)},
			},
			hooks.HookNetworkWorkClosed: {
				stream:      watchEventsTestNetworkStream,
				ledgerTypes: []string{string(hooks.HookNetworkWorkClosed)},
			},
			hooks.HookCoordinatorSpawned: {
				stream:      watchEventsTestObserveStream,
				ledgerTypes: []string{string(hooks.HookCoordinatorSpawned)},
			},
			hooks.HookCoordinatorDecision: {
				stream:      watchEventsTestObserveStream,
				ledgerTypes: []string{string(hooks.HookCoordinatorDecision)},
			},
			hooks.HookCoordinatorStopped: {
				stream:      watchEventsTestObserveStream,
				ledgerTypes: []string{string(hooks.HookCoordinatorStopped)},
			},
			hooks.HookCoordinatorFailed: {
				stream:      watchEventsTestObserveStream,
				ledgerTypes: []string{string(hooks.HookCoordinatorFailed)},
			},
			hooks.HookEventPostRecord: {
				stream:      watchEventsTestSessionStream,
				ledgerTypes: []string{string(hooks.HookEventPostRecord)},
			},
		}
		catalog := hookCatalogForTest()
		for kind, contract := range contracts {
			if _, ok := catalog[kind]; !ok {
				t.Fatalf("SupportedWatchEvents()[%q] is not in AllHookEvents()", kind)
			}
			if kind == hooks.HookEventPostRecord {
				if !slices.Equal(contract.RequiredVars, []string{"session_id"}) {
					t.Fatalf(
						"SupportedWatchEvents()[%q].RequiredVars = %#v, want session_id",
						kind,
						contract.RequiredVars,
					)
				}
			} else if len(contract.RequiredVars) != 0 {
				t.Fatalf(
					"SupportedWatchEvents()[%q].RequiredVars = %#v, want empty",
					kind,
					contract.RequiredVars,
				)
			}
			if len(contract.PayloadFields) == 0 {
				t.Fatalf("SupportedWatchEvents()[%q].PayloadFields is empty", kind)
			}
			want, ok := expected[kind]
			if !ok {
				t.Fatalf("SupportedWatchEvents() contains unexpected kind %q", kind)
			}
			if contract.Stream != want.stream {
				t.Fatalf(
					"SupportedWatchEvents()[%q].Stream = %q, want %q",
					kind,
					contract.Stream,
					want.stream,
				)
			}
			if !reflect.DeepEqual(contract.LedgerTypes, want.ledgerTypes) {
				t.Fatalf(
					"SupportedWatchEvents()[%q].LedgerTypes = %#v, want %#v",
					kind,
					contract.LedgerTypes,
					want.ledgerTypes,
				)
			}
		}
		for _, preStateKind := range []hooks.HookEvent{
			hooks.HookAutomationJobPreFire,
			hooks.HookEventPreRecord,
			hooks.HookCoordinatorPreSpawn,
			hooks.HookTaskRunPreClaim,
			hooks.HookLoopGenerationPre,
			hooks.HookLoopGatePre,
		} {
			if _, ok := contracts[preStateKind]; ok {
				t.Fatalf("SupportedWatchEvents() contains pre-state hook %q", preStateKind)
			}
		}
		if _, ok := contracts[hooks.HookAutomationJobPreFire]; ok {
			t.Fatal("SupportedWatchEvents() contains automation.job.pre_fire, want unsupported")
		}
		if _, ok := contracts[hooks.HookAutomationJobPostFire]; ok {
			t.Fatal("SupportedWatchEvents() contains automation.job.post_fire, want unsupported")
		}
		if _, ok := contracts[hooks.HookNetworkPeerJoined]; ok {
			t.Fatal("SupportedWatchEvents() contains network.peer.joined, want unsupported")
		}
		if _, ok := contracts[hooks.HookNetworkPeerLeft]; ok {
			t.Fatal("SupportedWatchEvents() contains network.peer.left, want unsupported")
		}
		if _, ok := contracts[hooks.HookCoordinatorPreSpawn]; ok {
			t.Fatal("SupportedWatchEvents() contains coordinator.pre_spawn, want unsupported")
		}
		if _, ok := contracts[hooks.HookEventPreRecord]; ok {
			t.Fatal("SupportedWatchEvents() contains event.pre_record, want unsupported")
		}
		statusChanged := contracts[hooks.HookTaskStatusChanged]
		if !slices.Contains(statusChanged.PayloadFields, "to_status") {
			t.Fatalf(
				"task.status_changed PayloadFields = %#v, want to_status",
				statusChanged.PayloadFields,
			)
		}
		nodeTerminal := contracts[hooks.HookLoopNodeTerminal]
		if !slices.Contains(nodeTerminal.LedgerTypes, loopRunEventTestNodeSucceeded) ||
			!slices.Contains(nodeTerminal.LedgerTypes, loopRunEventTestNodeFailed) {
			t.Fatalf(
				"loop.node.terminal LedgerTypes = %#v, want succeeded and failed",
				nodeTerminal.LedgerTypes,
			)
		}
		networkWork := contracts[hooks.HookNetworkWorkTransitioned]
		if !slices.Contains(networkWork.PayloadFields, "work_state") {
			t.Fatalf(
				"network.work.transitioned PayloadFields = %#v, want work_state",
				networkWork.PayloadFields,
			)
		}
		coordinatorStopped := contracts[hooks.HookCoordinatorStopped]
		if !slices.Contains(coordinatorStopped.PayloadFields, "stop_reason") {
			t.Fatalf(
				"coordinator.stopped PayloadFields = %#v, want stop_reason",
				coordinatorStopped.PayloadFields,
			)
		}
		for _, kind := range []hooks.HookEvent{
			hooks.HookCoordinatorSpawned,
			hooks.HookCoordinatorDecision,
			hooks.HookCoordinatorStopped,
			hooks.HookCoordinatorFailed,
		} {
			if !slices.Contains(contracts[kind].PayloadFields, watchEventsTestParticipation) {
				t.Fatalf(
					"%s PayloadFields = %#v, want %s",
					kind,
					contracts[kind].PayloadFields,
					watchEventsTestParticipation,
				)
			}
		}
		eventPostRecord := contracts[hooks.HookEventPostRecord]
		if slices.Contains(eventPostRecord.PayloadFields, "content") {
			t.Fatalf("event.post_record PayloadFields = %#v, want content excluded", eventPostRecord.PayloadFields)
		}
		if !slices.Contains(eventPostRecord.PayloadFields, "sequence") ||
			!slices.Contains(eventPostRecord.PayloadFields, "record_type") {
			t.Fatalf(
				"event.post_record PayloadFields = %#v, want sequence and record_type",
				eventPostRecord.PayloadFields,
			)
		}
	})
}

func hookCatalogForTest() map[hooks.HookEvent]struct{} {
	catalog := make(map[hooks.HookEvent]struct{}, len(hooks.AllHookEvents()))
	for _, event := range hooks.AllHookEvents() {
		catalog[event] = struct{}{}
	}
	return catalog
}
