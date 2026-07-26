package daemon

import (
	"maps"

	toolspkg "github.com/compozy/agh/internal/tools"
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
) map[toolspkg.ToolID]nativeToolBinding {
	return map[toolspkg.ToolID]nativeToolBinding{
		toolspkg.ToolIDSessionList: {
			call:         n.sessionList,
			availability: catalogAvailability,
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

func (n *daemonNativeTools) workspaceToolBindings(
	availability toolspkg.NativeAvailabilityFunc,
	describeAvailability toolspkg.NativeAvailabilityFunc,
	agentCreateAvailability toolspkg.NativeAvailabilityFunc,
) map[toolspkg.ToolID]nativeToolBinding {
	return map[toolspkg.ToolID]nativeToolBinding{
		toolspkg.ToolIDWorkspaceList: {
			call:         n.workspaceList,
			availability: availability,
		},
		toolspkg.ToolIDWorkspaceInfo: {
			call:         n.workspaceInfo,
			availability: availability,
		},
		toolspkg.ToolIDWorkspaceDescribe: {
			call:         n.workspaceDescribe,
			availability: describeAvailability,
		},
		toolspkg.ToolIDAgentCreate: {
			call:         n.agentCreate,
			availability: agentCreateAvailability,
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
		toolspkg.ToolIDTaskRunList: {
			call:         n.taskRunList,
			availability: availability,
		},
		toolspkg.ToolIDTaskRunReviewRequest: {
			call:         n.taskRunReviewRequest,
			availability: availability,
		},
		toolspkg.ToolIDTaskRunReviewList: {
			call:         n.taskRunReviewList,
			availability: availability,
		},
		toolspkg.ToolIDTaskRunReviewShow: {
			call:         n.taskRunReviewShow,
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
		toolspkg.ToolIDTaskExecutionProfileDelete: {
			call:         n.taskExecutionProfileDelete,
			availability: availability,
		},
	}
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
