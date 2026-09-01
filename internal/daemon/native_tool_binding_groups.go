package daemon

import (
	"maps"

	toolspkg "github.com/compozy/compozy/internal/tools"
)

func (n *daemonNativeTools) registryToolBindings(
	availability toolspkg.NativeAvailabilityFunc,
) map[toolspkg.ToolID]nativeToolBinding {
	return map[toolspkg.ToolID]nativeToolBinding{
		toolspkg.ToolIDToolList: {
			call:         n.toolList,
			availability: availability,
		},
		toolspkg.ToolIDToolSearch: {
			call:         n.toolSearch,
			availability: availability,
		},
		toolspkg.ToolIDToolInfo: {
			call:         n.toolInfo,
			availability: availability,
		},
	}
}

func (n *daemonNativeTools) skillToolBindings(
	availability toolspkg.NativeAvailabilityFunc,
) map[toolspkg.ToolID]nativeToolBinding {
	return map[toolspkg.ToolID]nativeToolBinding{
		toolspkg.ToolIDSkillList: {
			call:         n.skillList,
			availability: availability,
		},
		toolspkg.ToolIDSkillSearch: {
			call:         n.skillSearch,
			availability: availability,
		},
		toolspkg.ToolIDSkillView: {
			call:         n.skillView,
			availability: availability,
		},
	}
}

func (n *daemonNativeTools) sessionToolBindings(
	availability toolspkg.NativeAvailabilityFunc,
	catalogAvailability toolspkg.NativeAvailabilityFunc,
	runtimeAvailability toolspkg.NativeAvailabilityFunc,
) map[toolspkg.ToolID]nativeToolBinding {
	return map[toolspkg.ToolID]nativeToolBinding{
		toolspkg.ToolIDSessionList: {
			call:         n.sessionList,
			availability: catalogAvailability,
		},
		toolspkg.ToolIDCommandList: {
			call:         n.commandList,
			availability: availability,
		},
		toolspkg.ToolIDSessionArchive: {
			call:         n.sessionArchive,
			availability: catalogAvailability,
		},
		toolspkg.ToolIDSessionUnarchive: {
			call:         n.sessionUnarchive,
			availability: catalogAvailability,
		},
		toolspkg.ToolIDSessionRename: {
			call:         n.sessionRename,
			availability: catalogAvailability,
		},
		toolspkg.ToolIDSessionCreate: {
			call:         n.sessionCreate,
			availability: n.sessionCreateAvailability(),
		},
		toolspkg.ToolIDSessionPrompt: {
			call:         n.sessionPrompt,
			availability: availability,
		},
		toolspkg.ToolIDSessionRewind: {
			call:         n.sessionRewind,
			availability: availability,
		},
		toolspkg.ToolIDSessionRuntimeSet: {
			call:         n.sessionRuntimeSet,
			availability: runtimeAvailability,
		},
		toolspkg.ToolIDSessionRuntimeClear: {
			call:         n.sessionRuntimeClear,
			availability: runtimeAvailability,
		},
		toolspkg.ToolIDSessionInputsList: {
			call:         n.sessionInputsList,
			availability: availability,
		},
		toolspkg.ToolIDSessionInputReplace: {
			call:         n.sessionInputReplace,
			availability: availability,
		},
		toolspkg.ToolIDSessionInputCancel: {
			call:         n.sessionInputCancel,
			availability: availability,
		},
		toolspkg.ToolIDSessionInputPromote: {
			call:         n.sessionInputPromote,
			availability: availability,
		},
		toolspkg.ToolIDSessionStatus: {
			call:         n.sessionStatus,
			availability: availability,
		},
		toolspkg.ToolIDSessionHistory: {
			call:         n.sessionHistory,
			availability: availability,
		},
		toolspkg.ToolIDSessionEvents: {
			call:         n.sessionEvents,
			availability: availability,
		},
		toolspkg.ToolIDSessionDescribe: {
			call:         n.sessionDescribe,
			availability: availability,
		},
	}
}

func (n *daemonNativeTools) authoredContextToolBindings(
	healthAvailability toolspkg.NativeAvailabilityFunc,
	statusAvailability toolspkg.NativeAvailabilityFunc,
	wakeAvailability toolspkg.NativeAvailabilityFunc,
) map[toolspkg.ToolID]nativeToolBinding {
	return map[toolspkg.ToolID]nativeToolBinding{
		toolspkg.ToolIDSessionHealth: {
			call:         n.sessionHealth,
			availability: healthAvailability,
		},
		toolspkg.ToolIDAgentHeartbeatStatus: {
			call:         n.agentHeartbeatStatus,
			availability: statusAvailability,
		},
		toolspkg.ToolIDAgentHeartbeatWake: {
			call:         n.agentHeartbeatWake,
			availability: wakeAvailability,
		},
	}
}

func (n *daemonNativeTools) worktreeToolBindings(
	availability toolspkg.NativeAvailabilityFunc,
) map[toolspkg.ToolID]nativeToolBinding {
	return map[toolspkg.ToolID]nativeToolBinding{
		toolspkg.ToolIDWorktreeList: {
			call: n.worktreeList, availability: availability,
		},
		toolspkg.ToolIDWorktreeInspect: {
			call: n.worktreeInspect, availability: availability,
		},
		toolspkg.ToolIDWorktreeCreate: {
			call: n.worktreeCreate, availability: availability,
		},
		toolspkg.ToolIDWorktreeRemove: {
			call: n.worktreeRemove, availability: availability,
		},
	}
}

func (n *daemonNativeTools) memoryToolBindings(
	availability toolspkg.NativeAvailabilityFunc,
) map[toolspkg.ToolID]nativeToolBinding {
	return map[toolspkg.ToolID]nativeToolBinding{
		toolspkg.ToolIDMemoryList: {
			call:         n.memoryList,
			availability: availability,
		},
		toolspkg.ToolIDMemoryShow: {
			call:         n.memoryShow,
			availability: availability,
		},
		toolspkg.ToolIDMemorySearch: {
			call:         n.memorySearch,
			availability: availability,
		},
		toolspkg.ToolIDMemoryPropose: {
			call:         n.memoryPropose,
			availability: availability,
		},
		toolspkg.ToolIDMemoryNote: {
			call:         n.memoryNote,
			availability: availability,
		},
	}
}

func (n *daemonNativeTools) observeToolBindings(
	availability toolspkg.NativeAvailabilityFunc,
) map[toolspkg.ToolID]nativeToolBinding {
	return map[toolspkg.ToolID]nativeToolBinding{
		toolspkg.ToolIDListLogs: {
			call:         n.listLogs,
			availability: availability,
		},
		toolspkg.ToolIDObserveMetrics: {
			call:         n.observeMetrics,
			availability: availability,
		},
		toolspkg.ToolIDObserveSearch: {
			call:         n.observeSearch,
			availability: availability,
		},
	}
}

func (n *daemonNativeTools) bridgeToolBindings(
	availability toolspkg.NativeAvailabilityFunc,
) map[toolspkg.ToolID]nativeToolBinding {
	return map[toolspkg.ToolID]nativeToolBinding{
		toolspkg.ToolIDBridgesList: {
			call:         n.bridgesList,
			availability: availability,
		},
		toolspkg.ToolIDBridgesStatus: {
			call:         n.bridgesStatus,
			availability: availability,
		},
	}
}

func (n *daemonNativeTools) taskToolBindings(
	availability toolspkg.NativeAvailabilityFunc,
	notificationAvailability toolspkg.NativeAvailabilityFunc,
) map[toolspkg.ToolID]nativeToolBinding {
	bindings := map[toolspkg.ToolID]nativeToolBinding{
		toolspkg.ToolIDTaskList: {
			call:         n.taskList,
			availability: availability,
		},
		toolspkg.ToolIDTaskRead: {
			call:         n.taskRead,
			availability: availability,
		},
		toolspkg.ToolIDTaskCreate: {
			call:         n.taskCreate,
			availability: availability,
		},
		toolspkg.ToolIDTaskChildCreate: {
			call:         n.taskChildCreate,
			availability: availability,
		},
		toolspkg.ToolIDTaskUpdate: {
			call:         n.taskUpdate,
			availability: availability,
		},
		toolspkg.ToolIDTaskCancel: {
			call:         n.taskCancel,
			availability: availability,
		},
		toolspkg.ToolIDTaskBlock: {
			call:         n.taskBlock,
			availability: availability,
		},
		toolspkg.ToolIDTaskUnblock: {
			call:         n.taskUnblock,
			availability: availability,
		},
		toolspkg.ToolIDTaskBlocks: {
			call:         n.taskBlocks,
			availability: availability,
		},
		toolspkg.ToolIDTaskRecover: {
			call:         n.taskRecover,
			availability: availability,
		},
		toolspkg.ToolIDTaskExecutionProfileGet: {
			call:         n.taskExecutionProfileGet,
			availability: availability,
		},
		toolspkg.ToolIDTaskExecutionProfileSet: {
			call:         n.taskExecutionProfileSet,
			availability: availability,
		},
		toolspkg.ToolIDTaskWorktreePolicySet: {
			call:         n.taskWorktreePolicySet,
			availability: availability,
		},
		toolspkg.ToolIDTaskExecutionProfileDelete: {
			call:         n.taskExecutionProfileDelete,
			availability: availability,
		},
	}
	mergeNativeToolBindings(bindings, n.taskRunToolBindings(availability))
	mergeNativeToolBindings(bindings, n.taskNotificationToolBindings(notificationAvailability))
	mergeNativeToolBindings(bindings, n.taskNetworkToolBindings(availability))
	return bindings
}

func mergeNativeToolBindings(
	dst map[toolspkg.ToolID]nativeToolBinding,
	src map[toolspkg.ToolID]nativeToolBinding,
) {
	maps.Copy(dst, src)
}

func (n *daemonNativeTools) taskNotificationToolBindings(
	availability toolspkg.NativeAvailabilityFunc,
) map[toolspkg.ToolID]nativeToolBinding {
	return map[toolspkg.ToolID]nativeToolBinding{
		toolspkg.ToolIDTaskNotificationSubscribe: {
			call:         n.taskNotificationSubscribe,
			availability: availability,
		},
		toolspkg.ToolIDTaskNotificationList: {
			call:         n.taskNotificationList,
			availability: availability,
		},
		toolspkg.ToolIDTaskNotificationShow: {
			call:         n.taskNotificationShow,
			availability: availability,
		},
		toolspkg.ToolIDTaskNotificationDelete: {
			call:         n.taskNotificationDelete,
			availability: availability,
		},
	}
}

func (n *daemonNativeTools) taskNetworkToolBindings(
	availability toolspkg.NativeAvailabilityFunc,
) map[toolspkg.ToolID]nativeToolBinding {
	return map[toolspkg.ToolID]nativeToolBinding{
		toolspkg.ToolIDTaskPromoteFromThread: {
			call:         n.taskPromoteFromThread,
			availability: availability,
		},
		toolspkg.ToolIDTaskFanOutRuns: {
			call:         n.taskFanOutRuns,
			availability: availability,
		},
	}
}

func (n *daemonNativeTools) autonomyToolBindings(
	availability toolspkg.NativeAvailabilityFunc,
) map[toolspkg.ToolID]nativeToolBinding {
	return map[toolspkg.ToolID]nativeToolBinding{
		toolspkg.ToolIDTaskRunClaimNext: {
			call:         n.autonomyClaimNext,
			availability: availability,
		},
		toolspkg.ToolIDTaskRunHeartbeat: {
			call:         n.autonomyHeartbeat,
			availability: availability,
		},
		toolspkg.ToolIDTaskRunComplete: {
			call:         n.autonomyComplete,
			availability: availability,
		},
		toolspkg.ToolIDTaskRunFail: {
			call:         n.autonomyFail,
			availability: availability,
		},
		toolspkg.ToolIDTaskRunRelease: {
			call:         n.autonomyRelease,
			availability: availability,
		},
		toolspkg.ToolIDTaskRunReviewSubmit: {
			call:         n.submitRunReview,
			availability: n.submitRunReviewAvailability,
		},
	}
}

func (n *daemonNativeTools) configToolBindings(
	availability toolspkg.NativeAvailabilityFunc,
) map[toolspkg.ToolID]nativeToolBinding {
	return map[toolspkg.ToolID]nativeToolBinding{
		toolspkg.ToolIDConfigShow: {
			call:         n.configShow,
			availability: availability,
		},
		toolspkg.ToolIDConfigList: {
			call:         n.configList,
			availability: availability,
		},
		toolspkg.ToolIDConfigGet: {
			call:         n.configGet,
			availability: availability,
		},
		toolspkg.ToolIDConfigSet: {
			call:         n.configSet,
			availability: availability,
		},
		toolspkg.ToolIDConfigUnset: {
			call:         n.configUnset,
			availability: availability,
		},
		toolspkg.ToolIDConfigDiff: {
			call:         n.configDiff,
			availability: availability,
		},
		toolspkg.ToolIDConfigPath: {
			call:         n.configPath,
			availability: availability,
		},
	}
}

func (n *daemonNativeTools) hookToolBindings(
	readAvailability toolspkg.NativeAvailabilityFunc,
	mutationAvailability toolspkg.NativeAvailabilityFunc,
) map[toolspkg.ToolID]nativeToolBinding {
	return map[toolspkg.ToolID]nativeToolBinding{
		toolspkg.ToolIDHooksList: {
			call:         n.hooksList,
			availability: readAvailability,
		},
		toolspkg.ToolIDHooksInfo: {
			call:         n.hooksInfo,
			availability: readAvailability,
		},
		toolspkg.ToolIDHooksEvents: {
			call:         n.hooksEvents,
			availability: readAvailability,
		},
		toolspkg.ToolIDHooksRuns: {
			call:         n.hooksRuns,
			availability: readAvailability,
		},
		toolspkg.ToolIDHooksCreate: {
			call:         n.hooksCreate,
			availability: mutationAvailability,
		},
		toolspkg.ToolIDHooksUpdate: {
			call:         n.hooksUpdate,
			availability: mutationAvailability,
		},
		toolspkg.ToolIDHooksDelete: {
			call:         n.hooksDelete,
			availability: mutationAvailability,
		},
		toolspkg.ToolIDHooksEnable: {
			call:         n.hooksEnable,
			availability: mutationAvailability,
		},
		toolspkg.ToolIDHooksDisable: {
			call:         n.hooksDisable,
			availability: mutationAvailability,
		},
	}
}
