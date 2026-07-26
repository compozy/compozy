package task

func newService(options managerOptions) *Service {
	taskAuthorizer := scopedTaskResourceAuthorizer{}
	service := &Service{
		store:                 options.store,
		sessions:              options.sessions,
		runtimeViews:          options.runtimeViews,
		inspectReader:         options.inspectReader,
		eventObserver:         options.eventObserver,
		reviewObserver:        options.reviewObserver,
		taskHooks:             defaultTaskRunHooks(options.taskHooks),
		coordinatorRunner:     options.coordinatorRunner,
		generationFinalizer:   options.generationFinalizer,
		wakeNotifier:          defaultWakeNotifier(options.wakeNotifier),
		participationResolver: options.participationResolver,
		taskAuthorizer:        taskAuthorizer,
		runReadAuthorizer:     taskRunReadAuthorizer{tasks: taskAuthorizer},
		coordinatorStatusOK:   options.coordinatorStatusOK,
		coordinatorHookOK:     options.coordinatorHookOK,
		profileValidation:     options.profileValidation,
		forceRecovery:         normalizeForceRecoveryOptions(options.forceRecovery),
		now:                   options.now,
		newID:                 options.newID,
		cancelGracePeriod:     options.cancelGracePeriod,
		starvationAge:         options.starvationAge,
		blockRecurrenceLimit:  options.blockRecurrenceLimit,
		workspaceActiveRunCap: options.workspaceActiveRunCap,
		workAdmission:         options.workAdmission,
		forceRateLimiter:      newForceRunRateLimiter(),
		wakeEventIDs:          make(map[string]struct{}),
		wakeEventOrder:        make([]string, 0, wakeEventCacheMaxEntries),
		liveSubscribers:       make(map[uint64]*taskStreamSubscriber),
	}
	if store, ok := options.store.(EventCommitObserverStore); ok {
		store.SetTaskEventCommitObserver(service)
	}
	return service
}
