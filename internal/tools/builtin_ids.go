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
	// ToolIDCommandList lists the unified command catalog for one session.
	ToolIDCommandList ToolID = "compozy__command_list"
	// ToolIDSessionList lists runtime sessions.
	ToolIDSessionList ToolID = "compozy__session_list"
	// ToolIDSessionArchive archives one stopped runtime session.
	ToolIDSessionArchive ToolID = "compozy__session_archive"
	// ToolIDSessionUnarchive restores one archived runtime session.
	ToolIDSessionUnarchive ToolID = "compozy__session_unarchive"
	// ToolIDSessionRename changes one user session's durable display name.
	ToolIDSessionRename ToolID = "compozy__session_rename"
	// ToolIDSessionCreate accepts one unbound logical user session.
	ToolIDSessionCreate ToolID = "compozy__session_create"
	// ToolIDSessionPrompt submits one prompt with an optional runtime snapshot.
	ToolIDSessionPrompt ToolID = "compozy__session_prompt"
	// ToolIDSessionRewind archives a conversation suffix and restarts the session context.
	ToolIDSessionRewind ToolID = "compozy__session_rewind"
	// ToolIDSessionContinue continues a user session with another agent, runtime, or route.
	ToolIDSessionContinue ToolID = "compozy__session_continue"
	// ToolIDSessionFork forks a user session with the same agent, whole or through one message.
	ToolIDSessionFork ToolID = "compozy__session_fork"
	// ToolIDSessionRuntimeSet selects the default runtime for future prompts.
	ToolIDSessionRuntimeSet ToolID = "compozy__session_runtime_set"
	// ToolIDSessionRuntimeClear clears the default runtime for future prompts.
	ToolIDSessionRuntimeClear ToolID = "compozy__session_runtime_clear"
	// ToolIDSessionInputsList lists durable pending session input in dispatch order.
	ToolIDSessionInputsList  ToolID = "compozy__session_inputs_list"
	ToolIDSessionInputsClear ToolID = "compozy__session_inputs_clear"
	// ToolIDSessionInputReplace atomically replaces one queued session input.
	ToolIDSessionInputReplace ToolID = "compozy__session_input_replace"
	// ToolIDSessionInputCancel cancels one queued session input.
	ToolIDSessionInputCancel ToolID = "compozy__session_input_cancel"
	// ToolIDSessionInputPromote atomically promotes queued input to steering.
	ToolIDSessionInputPromote ToolID = "compozy__session_input_promote"
	// ToolIDSessionStatus reads one runtime session snapshot.
	ToolIDSessionStatus ToolID = "compozy__session_status"
	// ToolIDSessionHistory reads grouped turn history for one session.
	ToolIDSessionHistory ToolID = "compozy__session_history"
	// ToolIDSessionEvents reads persisted events for one session.
	ToolIDSessionEvents ToolID = "compozy__session_events"
	// ToolIDSessionSearch searches retained projected messages.
	ToolIDSessionSearch ToolID = "compozy__session_search"
	// ToolIDSessionOutline reads the retained operator-message trail.
	ToolIDSessionOutline ToolID = "compozy__session_outline"
	// ToolIDSessionDescribe reads a composite read-only session description.
	ToolIDSessionDescribe ToolID = "compozy__session_describe"
	// ToolIDSessionWait waits for one bounded session badge transition.
	ToolIDSessionWait ToolID = "compozy__session_wait"
	// ToolIDSessionSpawn creates one governed child session for the caller.
	ToolIDSessionSpawn ToolID = "compozy__session_spawn"
	// ToolIDSessionStop stops one sibling or child session.
	ToolIDSessionStop ToolID = "compozy__session_stop"
	// ToolIDSessionApprove resolves one pending permission request.
	ToolIDSessionApprove ToolID = "compozy__session_approve"
	// ToolIDSessionClarifyAnswer resolves one pending clarification request.
	ToolIDSessionClarifyAnswer ToolID = "compozy__session_clarify_answer"
	// ToolIDSessionPromptCancel cancels one in-flight session prompt.
	ToolIDSessionPromptCancel ToolID = "compozy__session_prompt_cancel"
	// ToolIDNotify sends one bounded operator notification from the bound session.
	ToolIDNotify ToolID = "compozy__notify"
	// ToolIDSessionHealth reads metadata-only session health and wake eligibility.
	ToolIDSessionHealth ToolID = "compozy__session_health"
	// ToolIDAgentHeartbeatStatus reads resolved Heartbeat policy, wake state, health, and wake audit.
	ToolIDAgentHeartbeatStatus ToolID = "compozy__agent_heartbeat_status"
	// ToolIDAgentHeartbeatWake requests one managed advisory Heartbeat wake decision.
	ToolIDAgentHeartbeatWake ToolID = "compozy__agent_heartbeat_wake"
	// ToolIDWorkspaceList lists registered workspaces.
	ToolIDWorkspaceList ToolID = "compozy__workspace_list"
	// ToolIDProfileList lists the daemon profile catalog for the bound session.
	ToolIDProfileList ToolID = "compozy__profile_list"
	// ToolIDProfileCurrent reads the immutable profile bound to the caller session.
	ToolIDProfileCurrent ToolID = "compozy__profile_current"
	// ToolIDAgentList lists agent definitions visible in one workspace scope.
	ToolIDAgentList ToolID = "compozy__agent_list"
	// ToolIDWorkspaceInfo reads one registered workspace record.
	ToolIDWorkspaceInfo ToolID = "compozy__workspace_info"
	// ToolIDWorkspaceDescribe reads one resolved workspace detail projection.
	ToolIDWorkspaceDescribe ToolID = "compozy__workspace_describe"
	// ToolIDWorktreeList lists workspace-scoped worktrees.
	ToolIDWorktreeList ToolID = "compozy__worktree_list"
	// ToolIDWorktreeInspect reads one worktree with cached status and bindings.
	ToolIDWorktreeInspect ToolID = "compozy__worktree_inspect"
	// ToolIDWorktreeCreate accepts one durable worktree creation.
	ToolIDWorktreeCreate ToolID = "compozy__worktree_create"
	// ToolIDWorktreeRemove removes one worktree after safety evaluation.
	ToolIDWorktreeRemove ToolID = "compozy__worktree_remove"
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
	// ToolIDVaultList lists global redacted Vault secret metadata.
	ToolIDVaultList ToolID = "compozy__vault_list"
	// ToolIDListLogs reads redacted runtime logs.
	ToolIDListLogs ToolID = "compozy__logs"
	// ToolIDToolArtifactRead pages one retained oversized tool result.
	ToolIDToolArtifactRead ToolID = "compozy__tool_artifact_read"
	// ToolIDObserveMetrics reads daemon observability health and metrics.
	ToolIDObserveMetrics ToolID = "compozy__observe_metrics"
	// ToolIDObserveSearch searches redacted observability events.
	ToolIDObserveSearch ToolID = "compozy__observe_search"
	// ToolIDGateway inspects gateway posture and performs permission-gated management actions.
	ToolIDGateway ToolID = "compozy__gateway"
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
	// ToolIDTaskRunResult pages one task-run result through the task service.
	ToolIDTaskRunResult ToolID = "compozy__task_run_result"
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
	// ToolIDTaskWorktreePolicySet updates only one task execution profile's worktree policy.
	ToolIDTaskWorktreePolicySet ToolID = "compozy__task_worktree_policy_set"
	// ToolIDTaskExecutionProfileDelete removes one task execution profile.
	ToolIDTaskExecutionProfileDelete ToolID = "compozy__task_execution_profile_delete"
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
	// ToolIDGoalControl applies one authenticated structured Goal operation to a target session.
	ToolIDGoalControl ToolID = "compozy__goal_control"
	// ToolIDGoalReport records one prompt-bound Goal completion or blocker intent.
	ToolIDGoalReport ToolID = "compozy__goal_report"
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
	// ToolIDMarketplaceSources lists global experimental marketplace sources.
	ToolIDMarketplaceSources ToolID = "compozy__marketplace_sources"
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
	// ToolsetIDWorktrees groups workspace-scoped worktree tools.
	ToolsetIDWorktrees ToolsetID = "compozy__worktree"
	// ToolsetIDProviderModels groups provider model catalog tools.
	ToolsetIDProviderModels ToolsetID = "compozy__provider_models"
	// ToolsetIDObserve groups read-only observability tools.
	ToolsetIDObserve ToolsetID = "compozy__observe"
	// ToolsetIDGateway groups gateway inspection and permission-gated management.
	ToolsetIDGateway ToolsetID = "compozy__gateway"
	// ToolsetIDConfig groups validated config tools.
	ToolsetIDConfig ToolsetID = "compozy__config"
	// ToolsetIDHooks groups hook introspection and mutable config-backed hook tools.
	ToolsetIDHooks ToolsetID = "compozy__hooks"
	// ToolsetIDAutomation groups automation lifecycle and run inspection tools.
	ToolsetIDAutomation ToolsetID = "compozy__automation"
	// ToolsetIDExtensions groups extension discovery and lifecycle tools.
	ToolsetIDExtensions ToolsetID = "compozy__extensions"
	// ToolsetIDMarketplace groups Marketplace extension catalog and source inspection tools.
	ToolsetIDMarketplace ToolsetID = "compozy__marketplace"
	// ToolsetIDResources groups desired-state resource inspection tools.
	ToolsetIDResources ToolsetID = "compozy__resources"
	// ToolsetIDWindowManager groups persistent desktop, window, and layout tools.
	ToolsetIDWindowManager ToolsetID = "compozy__window_manager"
	// ToolsetIDTerminal groups supervised terminal observation and execution tools.
	ToolsetIDTerminal ToolsetID = "compozy__terminal"
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
