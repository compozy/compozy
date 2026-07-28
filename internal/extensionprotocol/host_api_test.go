package protocol

import "testing"

func TestAllHostAPIMethodsReturnsCanonicalWireOrder(t *testing.T) {
	t.Parallel()

	t.Run("Should return canonical wire order", func(t *testing.T) {
		t.Parallel()

		want := []HostAPIMethod{
			HostAPIMethodSessionsList,
			HostAPIMethodSessionsCreate,
			HostAPIMethodSessionsPrompt,
			HostAPIMethodSessionsStop,
			HostAPIMethodSessionsStatus,
			HostAPIMethodSessionsEvents,
			HostAPIMethodSessionsSoulRefresh,
			HostAPIMethodSessionsHealthGet,
			HostAPIMethodSessionsStatusGet,
			HostAPIMethodSandboxList,
			HostAPIMethodSandboxInfo,
			HostAPIMethodSandboxExec,
			HostAPIMethodMemoryRecall,
			HostAPIMethodMemoryStore,
			HostAPIMethodMemoryForget,
			HostAPIMethodObserveHealth,
			HostAPIMethodListLogs,
			HostAPIMethodSkillsList,
			HostAPIMethodModelsList,
			HostAPIMethodModelsRefresh,
			HostAPIMethodModelsStatus,
			HostAPIMethodAgentsSoulGet,
			HostAPIMethodAgentsSoulValidate,
			HostAPIMethodAgentsSoulPut,
			HostAPIMethodAgentsSoulDelete,
			HostAPIMethodAgentsSoulHistory,
			HostAPIMethodAgentsSoulRollback,
			HostAPIMethodAgentsHeartbeatGet,
			HostAPIMethodAgentsHeartbeatValidate,
			HostAPIMethodAgentsHeartbeatPut,
			HostAPIMethodAgentsHeartbeatDelete,
			HostAPIMethodAgentsHeartbeatHistory,
			HostAPIMethodAgentsHeartbeatRollback,
			HostAPIMethodAgentsHeartbeatStatus,
			HostAPIMethodAgentsHeartbeatWake,
			HostAPIMethodAutomationJobs,
			HostAPIMethodAutomationJobsGet,
			HostAPIMethodAutomationJobsCreate,
			HostAPIMethodAutomationJobsUpdate,
			HostAPIMethodAutomationJobsDelete,
			HostAPIMethodAutomationJobsTrigger,
			HostAPIMethodAutomationJobsRuns,
			HostAPIMethodAutomationTriggers,
			HostAPIMethodAutomationTriggersGet,
			HostAPIMethodAutomationTriggersCreate,
			HostAPIMethodAutomationTriggersUpdate,
			HostAPIMethodAutomationTriggersDelete,
			HostAPIMethodAutomationTriggersRuns,
			HostAPIMethodAutomationTriggersFire,
			HostAPIMethodAutomationRuns,
			HostAPIMethodTasks,
			HostAPIMethodTasksGet,
			HostAPIMethodTasksTimeline,
			HostAPIMethodTasksTree,
			HostAPIMethodTasksDashboard,
			HostAPIMethodTasksInbox,
			HostAPIMethodTasksCreate,
			HostAPIMethodTasksUpdate,
			HostAPIMethodTasksCancel,
			HostAPIMethodTasksRuns,
			HostAPIMethodTasksRunsGet,
			HostAPIMethodTasksRunsEnqueue,
			HostAPIMethodTasksRunsStart,
			HostAPIMethodTasksRunsAttachSession,
			HostAPIMethodTasksRunsComplete,
			HostAPIMethodTasksRunsFail,
			HostAPIMethodTasksRunsCancel,
			HostAPIMethodNetworkStatus,
			HostAPIMethodNetworkUsage,
			HostAPIMethodNetworkChannels,
			HostAPIMethodNetworkPeers,
			HostAPIMethodNetworkThreads,
			HostAPIMethodNetworkThreadGet,
			HostAPIMethodNetworkThreadMessages,
			HostAPIMethodNetworkDirects,
			HostAPIMethodNetworkDirectResolve,
			HostAPIMethodNetworkDirectMessages,
			HostAPIMethodNetworkWorkGet,
			HostAPIMethodNetworkSend,
			HostAPIMethodResourcesList,
			HostAPIMethodResourcesGet,
			HostAPIMethodResourcesSnapshot,
			HostAPIMethodBridgesInstancesList,
			HostAPIMethodBridgesMessagesIngest,
			HostAPIMethodBridgesInstancesGet,
			HostAPIMethodBridgesInstancesReportState,
			HostAPIMethodClarifyAsk,
		}

		got := AllHostAPIMethods()
		if len(got) != len(want) {
			t.Fatalf("len(AllHostAPIMethods()) = %d, want %d", len(got), len(want))
		}
		for idx := range want {
			if got[idx] != want[idx] {
				t.Fatalf("AllHostAPIMethods()[%d] = %q, want %q", idx, got[idx], want[idx])
			}
		}
	})
}

func TestCapabilityServiceMethodsShouldIncludeModelSourceMethod(t *testing.T) {
	t.Parallel()

	t.Run("Should include model source method", func(t *testing.T) {
		t.Parallel()

		got := CapabilityServiceMethods([]string{CapabilityProvideModelSource})
		want := []string{string(ExtensionServiceMethodModelsList)}
		if len(got) != len(want) {
			t.Fatalf("len(CapabilityServiceMethods(model.source)) = %d, want %d", len(got), len(want))
		}
		for idx := range want {
			if got[idx] != want[idx] {
				t.Fatalf("CapabilityServiceMethods(model.source)[%d] = %q, want %q", idx, got[idx], want[idx])
			}
		}
	})
}

func TestCapabilityServiceMethodsShouldIncludeWatchPollMethod(t *testing.T) {
	t.Parallel()

	t.Run("Should include watch poll method", func(t *testing.T) {
		t.Parallel()

		got := CapabilityServiceMethods([]string{CapabilityProvideWatchSource})
		want := []string{string(ExtensionServiceMethodWatchPoll)}
		if len(got) != len(want) {
			t.Fatalf("len(CapabilityServiceMethods(loop.watch_source)) = %d, want %d", len(got), len(want))
		}
		for idx := range want {
			if got[idx] != want[idx] {
				t.Fatalf("CapabilityServiceMethods(loop.watch_source)[%d] = %q, want %q", idx, got[idx], want[idx])
			}
		}
	})
}

func TestCapabilityServiceMethodsShouldIncludeBridgeTargetSnapshotMethod(t *testing.T) {
	t.Parallel()

	t.Run("Should include bridge delivery and target snapshot methods", func(t *testing.T) {
		t.Parallel()

		got := CapabilityServiceMethods([]string{CapabilityProvideBridgeAdapter})
		want := []string{
			string(ExtensionServiceMethodBridgesDeliver),
			string(ExtensionServiceMethodBridgeTargets),
		}
		if len(got) != len(want) {
			t.Fatalf("len(CapabilityServiceMethods(bridge.adapter)) = %d, want %d", len(got), len(want))
		}
		for idx := range want {
			if got[idx] != want[idx] {
				t.Fatalf("CapabilityServiceMethods(bridge.adapter)[%d] = %q, want %q", idx, got[idx], want[idx])
			}
		}
	})
}

func TestCapabilityServiceMethodsShouldNormalizeModelSourceProvides(t *testing.T) {
	t.Parallel()

	t.Run("Should normalize model source provides", func(t *testing.T) {
		t.Parallel()

		got := CapabilityServiceMethods([]string{
			" ",
			CapabilityProvideModelSource,
			CapabilityProvideModelSource,
			"unknown.provide",
		})
		want := []string{string(ExtensionServiceMethodModelsList)}
		if len(got) != len(want) {
			t.Fatalf("len(CapabilityServiceMethods()) = %d, want %d", len(got), len(want))
		}
		if got[0] != want[0] {
			t.Fatalf("CapabilityServiceMethods()[0] = %q, want %q", got[0], want[0])
		}
		if got := CapabilityServiceMethods(nil); got != nil {
			t.Fatalf("CapabilityServiceMethods(nil) = %#v, want nil", got)
		}
	})
}
