package task

import "github.com/compozy/compozy/internal/contracts"

func newService(options managerOptions) *Service {
	taskAuthorizer := scopedTaskResourceAuthorizer{workspaceAccess: options.workspaceAccess}
	service := &Service{
		store:                    options.store,
		sessions:                 options.sessions,
		runtimeViews:             options.runtimeViews,
		inspectReader:            options.inspectReader,
		eventObserver:            options.eventObserver,
		reviewObserver:           options.reviewObserver,
		taskHooks:                defaultTaskRunHooks(options.taskHooks),
		coordinatorRunner:        options.coordinatorRunner,
		coordinatorPostCommit:    options.coordinatorPostCommit,
		generationFinalizer:      options.generationFinalizer,
		coordinatorTimerArmer:    options.coordinatorTimerArmer,
		wakeNotifier:             defaultWakeNotifier(options.wakeNotifier),
		participationResolver:    options.participationResolver,
		taskAuthorizer:           taskAuthorizer,
		runReadAuthorizer:        taskRunReadAuthorizer{tasks: taskAuthorizer},
		coordinatorStatusOK:      options.coordinatorStatusOK,
		coordinatorHookOK:        options.coordinatorHookOK,
		profileValidation:        options.profileValidation,
		worktreeRefValidator:     options.worktreeRefValidator,
		forceRecovery:            normalizeForceRecoveryOptions(options.forceRecovery),
		now:                      options.now,
		newID:                    options.newID,
		cancelGracePeriod:        options.cancelGracePeriod,
		starvationAge:            options.starvationAge,
		blockRecurrenceLimit:     options.blockRecurrenceLimit,
		workspaceActiveRunCap:    options.workspaceActiveRunCap,
		governedRootActiveRunCap: options.governedRootActiveRunCap,
		workAdmission:            options.workAdmission,
		workspaceAccess:          options.workspaceAccess,
		resultBudget:             options.resultBudgetConfig.DefaultBudget,
		resultBudgetConfig:       options.resultBudgetConfig,
		forceRateLimiter:         newForceRunRateLimiter(),
		wakeEventIDs:             make(map[string]struct{}),
		wakeEventOrder:           make([]string, 0, wakeEventCacheMaxEntries),
		liveSubscribers:          make(map[uint64]*taskStreamSubscriber),
	}
	if registryStore, ok := options.store.(contracts.RegistryStore); ok {
		service.resultContracts = contracts.NewRegistry(registryStore)
	}
	if store, ok := options.store.(EventCommitObserverStore); ok {
		store.SetTaskEventCommitObserver(service)
	}
	return service
}
