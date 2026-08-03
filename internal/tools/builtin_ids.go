package tools

const (
	// BuiltinSourceOwner is the source owner for daemon-compiled Compozy tools.
	BuiltinSourceOwner = "daemon"
)

const (
	// ToolIDToolList lists tools in the caller's effective registry projection.
	ToolIDToolList ToolID = "compozy__tool_list"
	// ToolIDToolSearch searches tools in the caller's effective registry projection.
	ToolIDToolSearch ToolID = "compozy__tool_search"
	// ToolIDToolInfo reads one tool descriptor and diagnostics view.
	ToolIDToolInfo ToolID = "compozy__tool_info"
	// ToolIDToolApprovalsSet sets one explicit wider native-tool approval decision.
	ToolIDToolApprovalsSet ToolID = "compozy__tool_approvals_set"
	// ToolIDToolApprovalsList lists durable native-tool approval decisions in one workspace.
	ToolIDToolApprovalsList ToolID = "compozy__tool_approvals_list"
	// ToolIDToolApprovalsRevoke revokes one durable native-tool approval decision.
	ToolIDToolApprovalsRevoke ToolID = "compozy__tool_approvals_revoke"
	// ToolIDClarify asks the user one bounded session-scoped question.
	ToolIDClarify ToolID = "compozy__clarify"
	// ToolIDSkillList lists skills through the existing skill registry.
	ToolIDSkillList ToolID = "compozy__skill_list"
	// ToolIDSkillSearch searches skills through the existing skill registry.
	ToolIDSkillSearch ToolID = "compozy__skill_search"
	// ToolIDSkillView reads one skill and its verified body.
	ToolIDSkillView ToolID = "compozy__skill_view"
	// ToolIDNetworkPeers lists visible network peers.
	ToolIDNetworkPeers ToolID = "compozy__network_peers"
	// ToolIDNetworkStatus reads daemon-owned network runtime status.
	ToolIDNetworkStatus ToolID = "compozy__network_status"
	// ToolIDNetworkUsage reads bounded workspace-scoped network wake usage.
	ToolIDNetworkUsage ToolID = "compozy__network_usage"
	// ToolIDNetworkChannels lists active Compozy network channels.
	ToolIDNetworkChannels ToolID = "compozy__network_channels"
	// ToolIDNetworkInbox reads queued inbound network messages for one local session.
	ToolIDNetworkInbox ToolID = "compozy__network_inbox"
	// ToolIDNetworkSend sends one network message through the existing network manager.
	ToolIDNetworkSend ToolID = "compozy__network_send"
	// ToolIDNetworkChannelCreate registers one Compozy network channel with a stated purpose.
	ToolIDNetworkChannelCreate ToolID = "compozy__network_channel_create"
	// ToolIDNetworkChannelUpdate updates one Compozy network channel delivery policy.
	ToolIDNetworkChannelUpdate ToolID = "compozy__network_channel_update"
	// ToolIDNetworkSubscriptions lists Compozy network delivery preferences.
	ToolIDNetworkSubscriptions ToolID = "compozy__network_subscriptions"
	// ToolIDNetworkSubscribe sets one Compozy network delivery preference to full delivery.
	ToolIDNetworkSubscribe ToolID = "compozy__network_subscribe"
	// ToolIDNetworkMute mutes one Compozy network delivery preference.
	ToolIDNetworkMute ToolID = "compozy__network_mute"
	// ToolIDNetworkUnmute removes one Compozy network delivery preference.
	ToolIDNetworkUnmute ToolID = "compozy__network_unmute"
	// ToolIDNetworkThreads lists public network thread summaries.
	ToolIDNetworkThreads ToolID = "compozy__network_threads"
	// ToolIDNetworkThreadMessages reads messages in one public network thread.
	ToolIDNetworkThreadMessages ToolID = "compozy__network_thread_messages"
	// ToolIDNetworkDirects lists direct-room summaries.
	ToolIDNetworkDirects ToolID = "compozy__network_directs"
	// ToolIDNetworkDirectResolve creates or returns one deterministic direct room.
	ToolIDNetworkDirectResolve ToolID = "compozy__network_direct_resolve"
	// ToolIDNetworkDirectMessages reads messages in one direct room.
	ToolIDNetworkDirectMessages ToolID = "compozy__network_direct_messages"
	// ToolIDNetworkWork reads one network work lifecycle row.
	ToolIDNetworkWork ToolID = "compozy__network_work"
	// ToolIDSessionList lists runtime sessions.
	ToolIDSessionList ToolID = "compozy__session_list"
	// ToolIDSessionCreate accepts one unbound logical user session.
	ToolIDSessionCreate ToolID = "compozy__session_create"
	// ToolIDSessionPrompt submits one prompt with an optional runtime snapshot.
	ToolIDSessionPrompt ToolID = "compozy__session_prompt"
	// ToolIDSessionStatus reads one runtime session snapshot.
	ToolIDSessionStatus ToolID = "compozy__session_status"
	// ToolIDSessionHistory reads grouped turn history for one session.
	ToolIDSessionHistory ToolID = "compozy__session_history"
	// ToolIDSessionEvents reads persisted events for one session.
	ToolIDSessionEvents ToolID = "compozy__session_events"
	// ToolIDSessionDescribe reads a composite read-only session description.
	ToolIDSessionDescribe ToolID = "compozy__session_describe"
	// ToolIDSessionHealth reads metadata-only session health and wake eligibility.
	ToolIDSessionHealth ToolID = "compozy__session_health"
	// ToolIDAgentHeartbeatStatus reads resolved Heartbeat policy, wake state, health, and wake audit.
	ToolIDAgentHeartbeatStatus ToolID = "compozy__agent_heartbeat_status"
	// ToolIDAgentHeartbeatWake requests one managed advisory Heartbeat wake decision.
	ToolIDAgentHeartbeatWake ToolID = "compozy__agent_heartbeat_wake"
	// ToolIDWorkspaceList lists registered workspaces.
	ToolIDWorkspaceList ToolID = "compozy__workspace_list"
	// ToolIDWorkspaceInfo reads one registered workspace record.
	ToolIDWorkspaceInfo ToolID = "compozy__workspace_info"
	// ToolIDWorkspaceDescribe reads one resolved workspace detail projection.
	ToolIDWorkspaceDescribe ToolID = "compozy__workspace_describe"
	// ToolIDAgentCreate authors one AGENT.md definition at global or workspace scope.
	ToolIDAgentCreate ToolID = "compozy__agent_create"
	// ToolIDProviderModelsList lists the daemon provider model catalog.
	ToolIDProviderModelsList ToolID = "compozy__provider_models_list"
	// ToolIDProviderModelsRefresh refreshes one or more provider model catalog sources.
	ToolIDProviderModelsRefresh ToolID = "compozy__provider_models_refresh"
	// ToolIDProviderModelsStatus reads provider model catalog source status.
	ToolIDProviderModelsStatus ToolID = "compozy__provider_models_status"
	// ToolIDProviderModelsCurate mutates one provider model's global curation metadata.
	ToolIDProviderModelsCurate ToolID = "compozy__provider_models_curate"
	// ToolIDMemoryList lists memory headers visible for a scope.
	ToolIDMemoryList ToolID = "compozy__memory_list"
	// ToolIDMemoryShow reads one memory document through the current memory store.
	ToolIDMemoryShow ToolID = "compozy__memory_show"
	// ToolIDMemorySearch recalls memory documents through the active memory provider.
	ToolIDMemorySearch ToolID = "compozy__memory_search"
	// ToolIDMemoryPropose submits a controller-backed memory proposal.
	ToolIDMemoryPropose ToolID = "compozy__memory_propose"
	// ToolIDMemoryNote records a controller-backed ad-hoc memory note.
	ToolIDMemoryNote ToolID = "compozy__memory_note"
	// ToolIDMemoryHealth reads Memory v2 health and derived catalog state.
	ToolIDMemoryHealth ToolID = "compozy__memory_health"
	// ToolIDMemoryScopeShow reports effective Memory v2 scope resolution.
	ToolIDMemoryScopeShow ToolID = "compozy__memory_scope_show"
	// ToolIDMemoryAdminHistory lists Memory v2 operation history without reusing the removed legacy ID.
	ToolIDMemoryAdminHistory ToolID = "compozy__memory_admin_history"
	// ToolIDMemoryReindex rebuilds Memory v2 derived indexes.
	ToolIDMemoryReindex ToolID = "compozy__memory_reindex"
	// ToolIDMemoryPromote promotes one Memory v2 entry across scopes.
	ToolIDMemoryPromote ToolID = "compozy__memory_promote"
	// ToolIDMemoryReset resets derived Memory v2 state.
	ToolIDMemoryReset ToolID = "compozy__memory_reset"
	// ToolIDMemoryReload invalidates future Memory v2 snapshots.
	ToolIDMemoryReload ToolID = "compozy__memory_reload"
	// ToolIDMemoryDecisionsList lists Memory v2 controller decisions.
	ToolIDMemoryDecisionsList ToolID = "compozy__memory_decisions_list"
	// ToolIDMemoryDecisionsShow reads one Memory v2 controller decision.
	ToolIDMemoryDecisionsShow ToolID = "compozy__memory_decisions_show"
	// ToolIDMemoryDecisionsRevert reverts one applied Memory v2 controller decision.
	ToolIDMemoryDecisionsRevert ToolID = "compozy__memory_decisions_revert"
	// ToolIDMemoryRecallTrace reads one materialized Memory v2 recall trace.
	ToolIDMemoryRecallTrace ToolID = "compozy__memory_recall_trace"
	// ToolIDMemoryDreamStatus reads live Memory v2 dreaming status.
	ToolIDMemoryDreamStatus ToolID = "compozy__memory_dream_status"
	// ToolIDMemoryDreamList lists Memory v2 dreaming run records.
	ToolIDMemoryDreamList ToolID = "compozy__memory_dream_list"
	// ToolIDMemoryDreamShow reads one Memory v2 dreaming run record.
	ToolIDMemoryDreamShow ToolID = "compozy__memory_dream_show"
	// ToolIDMemoryDreamTrigger triggers Memory v2 dream consolidation.
	ToolIDMemoryDreamTrigger ToolID = "compozy__memory_dream_trigger"
	// ToolIDMemoryDreamRetry retries Memory v2 dream consolidation.
	ToolIDMemoryDreamRetry ToolID = "compozy__memory_dream_retry"
	// ToolIDMemoryDailyList lists Memory v2 daily operation logs.
	ToolIDMemoryDailyList ToolID = "compozy__memory_daily_list"
	// ToolIDMemoryExtractorStatus reads Memory v2 extractor queue status.
	ToolIDMemoryExtractorStatus ToolID = "compozy__memory_extractor_status"
	// ToolIDMemoryExtractorFailures lists Memory v2 extractor failures.
	ToolIDMemoryExtractorFailures ToolID = "compozy__memory_extractor_failures"
	// ToolIDMemoryExtractorRetry retries Memory v2 extractor failures.
	ToolIDMemoryExtractorRetry ToolID = "compozy__memory_extractor_retry"
	// ToolIDMemoryExtractorDrain drains the Memory v2 extractor queue.
	ToolIDMemoryExtractorDrain ToolID = "compozy__memory_extractor_drain"
	// ToolIDMemoryProviderList lists Memory v2 providers.
	ToolIDMemoryProviderList ToolID = "compozy__memory_provider_list"
	// ToolIDMemoryProviderGet reads one Memory v2 provider.
	ToolIDMemoryProviderGet ToolID = "compozy__memory_provider_get"
	// ToolIDMemoryProviderSelect selects the active Memory v2 provider.
	ToolIDMemoryProviderSelect ToolID = "compozy__memory_provider_select"
	// ToolIDMemoryProviderEnable enables one Memory v2 provider.
	ToolIDMemoryProviderEnable ToolID = "compozy__memory_provider_enable"
	// ToolIDMemoryProviderDisable disables one Memory v2 provider.
	ToolIDMemoryProviderDisable ToolID = "compozy__memory_provider_disable"
	// ToolIDMemorySessionLedger reads one materialized Memory v2 session ledger.
	ToolIDMemorySessionLedger ToolID = "compozy__memory_session_ledger"
	// ToolIDMemorySessionReplay replays one materialized Memory v2 session ledger.
	ToolIDMemorySessionReplay ToolID = "compozy__memory_session_replay"
	// ToolIDMemorySessionsPrune prunes Memory v2 session ledgers.
	ToolIDMemorySessionsPrune ToolID = "compozy__memory_sessions_prune"
	// ToolIDMemorySessionsRepair repairs Memory v2 session ledgers.
	ToolIDMemorySessionsRepair ToolID = "compozy__memory_sessions_repair"
	// ToolIDListLogs reads redacted runtime logs.
	ToolIDListLogs ToolID = "compozy__logs"
	// ToolIDToolArtifactRead pages one retained oversized tool result.
	ToolIDToolArtifactRead ToolID = "compozy__tool_artifact_read"
	// ToolIDObserveMetrics reads daemon observability health and metrics.
	ToolIDObserveMetrics ToolID = "compozy__observe_metrics"
	// ToolIDObserveSearch searches redacted observability events.
	ToolIDObserveSearch ToolID = "compozy__observe_search"
	// ToolIDBridgesList lists bridge instances without secret bindings.
	ToolIDBridgesList ToolID = "compozy__bridges_list"
	// ToolIDBridgesStatus reads bridge status and health without credentials.
	ToolIDBridgesStatus ToolID = "compozy__bridges_status"
	// ToolIDTaskList lists task summaries through the task service.
	ToolIDTaskList ToolID = "compozy__task_list"
	// ToolIDTaskRead reads one task view through the task service.
	ToolIDTaskRead ToolID = "compozy__task_read"
	// ToolIDTaskCreate creates one root task through the task service.
	ToolIDTaskCreate ToolID = "compozy__task_create"
	// ToolIDTaskChildCreate creates one child task through the task service.
	ToolIDTaskChildCreate ToolID = "compozy__task_child_create"
	// ToolIDTaskUpdate updates one task through the task service.
	ToolIDTaskUpdate ToolID = "compozy__task_update"
	// ToolIDTaskCancel cancels one task through the task service.
	ToolIDTaskCancel ToolID = "compozy__task_cancel"
	// ToolIDTaskBlock creates one runtime-declared task block.
	ToolIDTaskBlock ToolID = "compozy__task_block"
	// ToolIDTaskUnblock clears one runtime-declared task block.
	ToolIDTaskUnblock ToolID = "compozy__task_unblock"
	// ToolIDTaskBlocks lists runtime-declared task blocks.
	ToolIDTaskBlocks ToolID = "compozy__task_blocks"
	// ToolIDTaskRecover clears task-level needs_attention state.
	ToolIDTaskRecover ToolID = "compozy__task_recover"
	// ToolIDTaskRunList lists task runs through the task service.
	ToolIDTaskRunList ToolID = "compozy__task_run_list"
	// ToolIDTaskRunReviewRequest requests a review for one terminal task run.
	ToolIDTaskRunReviewRequest ToolID = "compozy__task_run_review_request"
	// ToolIDTaskRunReviewList lists task-run reviews through the task service.
	ToolIDTaskRunReviewList ToolID = "compozy__task_run_review_list"
	// ToolIDTaskRunReviewShow reads one task-run review through the task service.
	ToolIDTaskRunReviewShow ToolID = "compozy__task_run_review_show"
	// ToolIDTaskExecutionProfileGet reads one task execution profile.
	ToolIDTaskExecutionProfileGet ToolID = "compozy__task_execution_profile_get"
	// ToolIDTaskExecutionProfileSet updates one task execution profile.
	ToolIDTaskExecutionProfileSet ToolID = "compozy__task_execution_profile_set"
	// ToolIDTaskExecutionProfileDelete removes one task execution profile.
	ToolIDTaskExecutionProfileDelete ToolID = "compozy__task_execution_profile_delete"
	// ToolIDTaskNotificationSubscribe creates one bridge notification subscription for a task.
	ToolIDTaskNotificationSubscribe ToolID = "compozy__task_notification_subscribe"
	// ToolIDTaskNotificationList lists bridge notification subscriptions for a task.
	ToolIDTaskNotificationList ToolID = "compozy__task_notification_list"
	// ToolIDTaskNotificationShow reads one bridge notification subscription for a task.
	ToolIDTaskNotificationShow ToolID = "compozy__task_notification_show"
	// ToolIDTaskNotificationDelete deletes one bridge notification subscription for a task.
	ToolIDTaskNotificationDelete ToolID = "compozy__task_notification_delete"
	// ToolIDTaskPromoteFromThread promotes one network thread message into a durable task.
	ToolIDTaskPromoteFromThread ToolID = "compozy__task_promote_from_thread"
	// ToolIDTaskFanOutRuns creates designated sibling task runs.
	ToolIDTaskFanOutRuns ToolID = "compozy__task_fanout_runs"
	// ToolIDTaskRunClaimNext claims the next run for the caller session.
	ToolIDTaskRunClaimNext ToolID = "compozy__task_run_claim_next"
	// ToolIDTaskRunHeartbeat extends the caller session's active run lease.
	ToolIDTaskRunHeartbeat ToolID = "compozy__task_run_heartbeat"
	// ToolIDTaskRunComplete completes the caller session's active run lease.
	ToolIDTaskRunComplete ToolID = "compozy__task_run_complete"
	// ToolIDTaskRunFail fails the caller session's active run lease.
	ToolIDTaskRunFail ToolID = "compozy__task_run_fail"
	// ToolIDTaskRunRelease releases the caller session's active run lease.
	ToolIDTaskRunRelease ToolID = "compozy__task_run_release"
	// ToolIDTaskRunReviewSubmit submits the caller session's bound task-run review verdict.
	ToolIDTaskRunReviewSubmit ToolID = "compozy__task_run_review_submit"
	// ToolIDConfigShow shows the redacted effective config.
	ToolIDConfigShow ToolID = "compozy__config_show"
	// ToolIDConfigList lists redacted effective config entries.
	ToolIDConfigList ToolID = "compozy__config_list"
	// ToolIDConfigGet reads one redacted effective config entry.
	ToolIDConfigGet ToolID = "compozy__config_get"
	// ToolIDConfigSet mutates one validated config overlay value.
	ToolIDConfigSet ToolID = "compozy__config_set"
	// ToolIDConfigUnset removes one validated config overlay value.
	ToolIDConfigUnset ToolID = "compozy__config_unset"
	// ToolIDConfigDiff compares defaults/global config against the effective view.
	ToolIDConfigDiff ToolID = "compozy__config_diff"
	// ToolIDConfigPath reports resolved config paths.
	ToolIDConfigPath ToolID = "compozy__config_path"
	// ToolIDHooksList lists resolved hooks.
	ToolIDHooksList ToolID = "compozy__hooks_list"
	// ToolIDHooksInfo reads one resolved hook.
	ToolIDHooksInfo ToolID = "compozy__hooks_info"
	// ToolIDHooksEvents lists supported hook events.
	ToolIDHooksEvents ToolID = "compozy__hooks_events"
	// ToolIDHooksRuns lists hook run audit records.
	ToolIDHooksRuns ToolID = "compozy__hooks_runs"
	// ToolIDHooksCreate creates one config-backed hook declaration.
	ToolIDHooksCreate ToolID = "compozy__hooks_create"
	// ToolIDHooksUpdate updates one config-backed hook declaration.
	ToolIDHooksUpdate ToolID = "compozy__hooks_update"
	// ToolIDHooksDelete deletes one config-backed hook declaration.
	ToolIDHooksDelete ToolID = "compozy__hooks_delete"
	// ToolIDHooksEnable enables one config-backed hook declaration.
	ToolIDHooksEnable ToolID = "compozy__hooks_enable"
	// ToolIDHooksDisable disables one config-backed hook declaration.
	ToolIDHooksDisable ToolID = "compozy__hooks_disable"
	// ToolIDGoalGet reads the visible Goal projection for the caller session.
	ToolIDGoalGet ToolID = "compozy__goal_get"
	// ToolIDGoalReport records one prompt-bound Goal completion or blocker intent.
	ToolIDGoalReport ToolID = "compozy__goal_report"
	// ToolIDLoopList lists Loop definitions available in a workspace.
	ToolIDLoopList ToolID = "compozy__loop_list"
	// ToolIDLoopInspect reads one Loop definition and authoring contract.
	ToolIDLoopInspect ToolID = "compozy__loop_inspect"
	// ToolIDLoopValidate lints and compiles one Loop definition without saving it.
	ToolIDLoopValidate ToolID = "compozy__loop_validate"
	// ToolIDLoopCreate creates or forks one Loop definition.
	ToolIDLoopCreate ToolID = "compozy__loop_create"
	// ToolIDLoopRun starts or dry-runs one Loop.
	ToolIDLoopRun ToolID = "compozy__loop_run"
	// ToolIDLoopStatus reads one Loop run status.
	ToolIDLoopStatus ToolID = "compozy__loop_status"
	// ToolIDLoopRuns lists workspace-scoped Loop runs.
	ToolIDLoopRuns ToolID = "compozy__loop_runs"
	// ToolIDLoopTurns lists one Loop run's total-order Goal turn audit.
	ToolIDLoopTurns ToolID = "compozy__loop_turns"
	// ToolIDLoopCancel requests cooperative cancellation for one Loop run.
	ToolIDLoopCancel ToolID = "compozy__loop_cancel"
	// ToolIDLoopKill immediately terminalizes one Loop run.
	ToolIDLoopKill ToolID = "compozy__loop_kill"
	// ToolIDLoopNodes lists workspace-scoped Loop node state.
	ToolIDLoopNodes ToolID = "compozy__loop_nodes"
	// ToolIDLoopNodePause parks one authored Loop node.
	ToolIDLoopNodePause ToolID = "compozy__loop_node_pause"
	// ToolIDLoopNodeResume releases one authored Loop node or manual wait.
	ToolIDLoopNodeResume ToolID = "compozy__loop_node_resume"
	// ToolIDLoopNodeCancel requests cooperative node cancellation.
	ToolIDLoopNodeCancel ToolID = "compozy__loop_node_cancel"
	// ToolIDLoopNodeKill immediately fences one Loop node.
	ToolIDLoopNodeKill ToolID = "compozy__loop_node_kill"
	// ToolIDLoopNodeRequeue clears one Loop node quarantine.
	ToolIDLoopNodeRequeue ToolID = "compozy__loop_node_requeue"
	// ToolIDLoopPause pauses one running Loop at a generation boundary.
	ToolIDLoopPause ToolID = "compozy__loop_pause"
	// ToolIDLoopResume resumes one paused Loop run.
	ToolIDLoopResume ToolID = "compozy__loop_resume"
	// ToolIDLoopConfigure writes per-loop runtime config overrides.
	ToolIDLoopConfigure ToolID = "compozy__loop_configure"
	// ToolIDLoopApprove applies one human-gate decision.
	ToolIDLoopApprove ToolID = "compozy__loop_approve"
	// ToolIDLoopDelete deletes one user-authored Loop definition.
	ToolIDLoopDelete ToolID = "compozy__loop_delete"
	// ToolIDAutomationJobsList lists automation jobs through the automation manager.
	ToolIDAutomationJobsList ToolID = "compozy__automation_jobs_list"
	// ToolIDAutomationJobsGet reads one automation job through the automation manager.
	ToolIDAutomationJobsGet ToolID = "compozy__automation_jobs_get"
	// ToolIDAutomationJobsCreate creates one dynamic automation job through the automation manager.
	ToolIDAutomationJobsCreate ToolID = "compozy__automation_jobs_create"
	// ToolIDAutomationJobsUpdate updates one automation job through the automation manager.
	ToolIDAutomationJobsUpdate ToolID = "compozy__automation_jobs_update"
	// ToolIDAutomationJobsDelete deletes one dynamic automation job through the automation manager.
	ToolIDAutomationJobsDelete ToolID = "compozy__automation_jobs_delete"
	// ToolIDAutomationJobsEnable enables one automation job through the automation manager.
	ToolIDAutomationJobsEnable ToolID = "compozy__automation_jobs_enable"
	// ToolIDAutomationJobsDisable disables one automation job through the automation manager.
	ToolIDAutomationJobsDisable ToolID = "compozy__automation_jobs_disable"
	// ToolIDAutomationJobsTrigger manually triggers one automation job through the automation manager.
	ToolIDAutomationJobsTrigger ToolID = "compozy__automation_jobs_trigger"
	// ToolIDAutomationJobsHistory lists run history for one automation job.
	ToolIDAutomationJobsHistory ToolID = "compozy__automation_jobs_history"
	// ToolIDAutomationTriggersList lists automation triggers through the automation manager.
	ToolIDAutomationTriggersList ToolID = "compozy__automation_triggers_list"
	// ToolIDAutomationTriggersGet reads one automation trigger through the automation manager.
	ToolIDAutomationTriggersGet ToolID = "compozy__automation_triggers_get"
	// ToolIDAutomationTriggersCreate creates one dynamic automation trigger through the automation manager.
	ToolIDAutomationTriggersCreate ToolID = "compozy__automation_triggers_create"
	// ToolIDAutomationTriggersUpdate updates one automation trigger through the automation manager.
	ToolIDAutomationTriggersUpdate ToolID = "compozy__automation_triggers_update"
	// ToolIDAutomationTriggersDelete deletes one dynamic automation trigger through the automation manager.
	ToolIDAutomationTriggersDelete ToolID = "compozy__automation_triggers_delete"
	// ToolIDAutomationTriggersEnable enables one automation trigger through the automation manager.
	ToolIDAutomationTriggersEnable ToolID = "compozy__automation_triggers_enable"
	// ToolIDAutomationTriggersDisable disables one automation trigger through the automation manager.
	ToolIDAutomationTriggersDisable ToolID = "compozy__automation_triggers_disable"
	// ToolIDAutomationTriggersHistory lists run history for one automation trigger.
	ToolIDAutomationTriggersHistory ToolID = "compozy__automation_triggers_history"
	// ToolIDAutomationRunsList lists automation run records through the automation manager.
	ToolIDAutomationRunsList ToolID = "compozy__automation_runs_list"
	// ToolIDAutomationRunsGet reads one automation run record through the automation manager.
	ToolIDAutomationRunsGet ToolID = "compozy__automation_runs_get"
	// ToolIDAutomationSuggestionsList lists workspace-scoped automation suggestions.
	ToolIDAutomationSuggestionsList ToolID = "compozy__automation_suggestions_list"
	// ToolIDAutomationSuggestionsAccept accepts one suggestion and creates its Job.
	ToolIDAutomationSuggestionsAccept ToolID = "compozy__automation_suggestions_accept"
	// ToolIDAutomationSuggestionsDismiss durably dismisses one suggestion.
	ToolIDAutomationSuggestionsDismiss ToolID = "compozy__automation_suggestions_dismiss"
	// ToolIDMarketplaceSearch searches the shared marketplace discovery plane.
	ToolIDMarketplaceSearch ToolID = "compozy__marketplace_search"
	// ToolIDResourcesList lists desired-state resource records.
	ToolIDResourcesList ToolID = "compozy__resources_list"
	// ToolIDResourcesInfo reads one desired-state resource record.
	ToolIDResourcesInfo ToolID = "compozy__resources_info"
	// ToolIDResourcesSnapshot reads a filtered desired-state resource snapshot.
	ToolIDResourcesSnapshot ToolID = "compozy__resources_snapshot"
	// ToolIDMCPStatus probes one configured MCP server without exposing login/logout as tools.
	ToolIDMCPStatus ToolID = "compozy__mcp_status"
	// ToolIDMCPAuthStatus reads redacted MCP auth diagnostics for one configured server.
	ToolIDMCPAuthStatus ToolID = "compozy__mcp_auth_status"
)

const (
	// ToolsetIDBootstrap groups registry self-inspection tools.
	ToolsetIDBootstrap ToolsetID = "compozy__bootstrap"
	// ToolsetIDCatalog groups registry and skill catalog tools.
	ToolsetIDCatalog ToolsetID = "compozy__catalog"
	// ToolsetIDToolArtifacts groups retained oversized result tools.
	ToolsetIDToolArtifacts ToolsetID = "compozy__tool_artifacts"
	// ToolsetIDToolApprovals groups durable native-tool approval management tools.
	ToolsetIDToolApprovals ToolsetID = "compozy__tool_approvals"
	// ToolsetIDClarify exposes the session-scoped human clarification tool.
	ToolsetIDClarify ToolsetID = "compozy__clarify"
	// ToolsetIDCoordination groups network coordination tools.
	ToolsetIDCoordination ToolsetID = "compozy__coordination"
	// ToolsetIDTasks groups bounded task tools.
	ToolsetIDTasks ToolsetID = "compozy__tasks"
	// ToolsetIDAutonomy groups session-bound task-run autonomy tools.
	ToolsetIDAutonomy ToolsetID = "compozy__autonomy"
	// ToolsetIDSessions groups runtime session tools.
	ToolsetIDSessions ToolsetID = "compozy__sessions"
	// ToolsetIDAuthoredContext groups managed Soul/Heartbeat read and wake tools.
	ToolsetIDAuthoredContext ToolsetID = "compozy__authored_context"
	// ToolsetIDWorkspace groups workspace inspection and managed agent authoring tools.
	ToolsetIDWorkspace ToolsetID = "compozy__workspace"
	// ToolsetIDProviderModels groups provider model catalog tools.
	ToolsetIDProviderModels ToolsetID = "compozy__provider_models"
	// ToolsetIDMemory groups Memory v2 read and proposal tools.
	ToolsetIDMemory ToolsetID = "compozy__memory"
	// ToolsetIDMemoryAdmin groups Memory v2 operational tools.
	ToolsetIDMemoryAdmin ToolsetID = "compozy__memory_admin"
	// ToolsetIDObserve groups read-only observability tools.
	ToolsetIDObserve ToolsetID = "compozy__observe"
	// ToolsetIDBridges groups read-only bridge inspection tools.
	ToolsetIDBridges ToolsetID = "compozy__bridges"
	// ToolsetIDConfig groups validated config tools.
	ToolsetIDConfig ToolsetID = "compozy__config"
	// ToolsetIDHooks groups hook introspection and mutable config-backed hook tools.
	ToolsetIDHooks ToolsetID = "compozy__hooks"
	// ToolsetIDLoops groups Loop authoring, execution, and run-management tools.
	ToolsetIDLoops ToolsetID = "compozy__loops"
	// ToolsetIDAutomation groups automation lifecycle and run inspection tools.
	ToolsetIDAutomation ToolsetID = "compozy__automation"
	// ToolsetIDExtensions groups extension discovery and lifecycle tools.
	ToolsetIDExtensions ToolsetID = "compozy__extensions"
	// ToolsetIDMarketplace groups cross-kind marketplace discovery tools.
	ToolsetIDMarketplace ToolsetID = "compozy__marketplace"
	// ToolsetIDResources groups desired-state resource inspection tools.
	ToolsetIDResources ToolsetID = "compozy__resources"
	// ToolsetIDWindowManager groups persistent desktop, window, and layout tools.
	ToolsetIDWindowManager ToolsetID = "compozy__window_manager"
	// ToolsetIDMCP groups MCP probe and status diagnostics.
	ToolsetIDMCP ToolsetID = "compozy__mcp"
	// ToolsetIDMCPAuth groups redacted MCP auth diagnostics.
	ToolsetIDMCPAuth ToolsetID = "compozy__mcp_auth"
)

// BuiltinSource returns the provenance shared by daemon-compiled Compozy tools.
func BuiltinSource() SourceRef {
	return SourceRef{
		Kind:  SourceBuiltin,
		Owner: BuiltinSourceOwner,
		Scope: BuiltinSourceOwner,
	}
}
