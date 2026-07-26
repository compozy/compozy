// Package events owns canonical runtime event names and metadata shared by
// producers, logs, notifications, and contract tests.
package events

import (
	"fmt"
	"strings"
)

// Outcome classifies an event for log filtering and notification policy.
type Outcome string

const (
	OutcomeInfo    Outcome = "info"
	OutcomeSuccess Outcome = "success"
	OutcomeFailure Outcome = "failure"
	OutcomeWarning Outcome = "warning"
)

// Metadata is the canonical registry entry for one event name.
type Metadata struct {
	Name                 string
	Family               string
	Component            string
	Outcome              Outcome
	EmitsToLogs          bool
	NotificationEligible bool
	GlobalScope          bool
}

const (
	ACPUserMessage         = "user_message"
	ACPSyntheticReentry    = "synthetic_reentry"
	ACPAgentMessage        = "agent_message"
	ACPThought             = "thought"
	ACPToolCall            = "tool_call"
	ACPToolResult          = "tool_result"
	ACPPlan                = "plan"
	ACPPermission          = "permission"
	ACPUsage               = "usage"
	ACPSystem              = "system"
	ACPRuntimeProgress     = "runtime_progress"
	ACPRuntimeWarning      = "runtime_warning"
	ACPDone                = "done"
	ACPError               = "error"
	SessionStopped         = "session_stopped"
	SessionUnhealthy       = "session.unhealthy"
	SessionHung            = "session.hung"
	SessionRecovered       = "session.recovered"
	SessionCompactionFired = "session.compaction_fired"

	TaskCreated                          = "task.created"
	TaskUpdated                          = "task.updated"
	TaskPublished                        = "task.published"
	TaskApproved                         = "task.approved"
	TaskRejected                         = "task.rejected"
	TaskCanceled                         = "task.canceled"
	TaskChildCreated                     = "task.child_created"
	TaskDependencyAdded                  = "task.dependency_added"
	TaskDependencyRemoved                = "task.dependency_removed"
	TaskPaused                           = "task.paused"
	TaskResumed                          = "task.resumed"
	TaskBlockCreated                     = "task.block.created"
	TaskBlockCleared                     = "task.block.cleared"
	TaskBlockExpired                     = "task.block.expired"
	TaskNeedsAttention                   = "task.needs_attention"
	TaskRecovered                        = "task.recovered"
	TaskRunEnqueued                      = "task.run_enqueued"
	TaskRunClaimed                       = "task.run_claimed"
	TaskRunStarting                      = "task.run_starting"
	TaskRunSessionBound                  = "task.run_session_bound"
	TaskRunStarted                       = "task.run_started"
	TaskRunCompleted                     = "task.run_completed"
	TaskRunFailed                        = "task.run_failed"
	TaskRunCanceled                      = "task.run_canceled"
	TaskRunForceStopped                  = "task.run_force_stopped"
	TaskRunRecovered                     = "task.run_recovered"
	TaskRunRejected                      = "task.run_rejected"
	TaskRunLeaseExtended                 = "task.run_lease_extended"
	TaskRunLeaseExpired                  = "task.run_lease_expired"
	TaskRunReleased                      = "task.run_released"
	TaskRunOperatorForcedFail            = "task.run_operator_forced_fail"
	TaskRunOperatorRetry                 = "task.run_operator_retry"
	TaskRunRecoveredFromAttention        = "task.run_recovered_from_attention"
	TaskRunStarved                       = "task.run_starved"
	TaskRunNeedsAttention                = "task.run_needs_attention"
	TaskExecutionProfileUpdated          = "task.execution_profile_updated"
	TaskExecutionProfileDeleted          = "task.execution_profile_deleted"
	TaskRunReviewRequested               = "task.run_review_requested"
	TaskRunReviewBound                   = "task.run_review_bound"
	TaskRunReviewRecorded                = "task.run_review_recorded"
	TaskRunReviewApproved                = "task.run_review_approved"
	TaskRunReviewRejected                = "task.run_review_rejected"
	TaskRunReviewBlocked                 = "task.run_review_blocked"
	TaskRunReviewError                   = "task.run_review_error"
	TaskRunReviewTimeout                 = "task.run_review_timeout"
	TaskRunReviewInvalidOutput           = "task.run_review_invalid_output"
	TaskRunReviewRetryEnqueued           = "task.run_review_retry_enqueued"
	TaskAutoEnqueueTriggered             = "task.auto_enqueue.triggered"
	TaskCompletionHallucinationBlocked   = "task.completion.hallucination_blocked"
	TaskCompletionHallucinationSuspected = "task.completion.hallucination_suspected"
	TaskWakeDelivered                    = "task.wake.delivered"
	TaskWakeSuppressed                   = "task.wake.suppressed"

	SettingsChanged  = "settings.changed"
	RoleFallbackUsed = "role.fallback.used"
	RoleResolveError = "role.resolve.error"

	SkillShadowed   = "skill.shadowed"
	SkillLoadFailed = "skills.load_failed"

	HookDispatchStart    = "hook.dispatch.start"
	HookDispatchComplete = "hook.dispatch.complete"

	CoordinatorSpawned  = "coordinator.spawned"
	CoordinatorDecision = "coordinator.decision"
	CoordinatorStopped  = "coordinator.stopped"
	CoordinatorFailed   = "coordinator.failed"

	HarnessContextResolved         = "harness.context_resolved"
	HarnessSectionSelected         = "harness.section_selected"
	HarnessAugmenterApplied        = "harness.augmenter_applied"
	HarnessAugmenterFailed         = "harness.augmenter_failed"
	HarnessDetachedRunCompleted    = "harness.detached_run_completed"
	HarnessSyntheticReentryEmitted = "harness.synthetic_reentry_emitted"
	HarnessSyntheticReentryDropped = "harness.synthetic_reentry_dropped"

	MemoryWriteCommitted     = "memory.write.committed"
	MemoryWriteRejected      = "memory.write.rejected"
	MemoryWriteShadowed      = "memory.write.shadowed"
	MemoryWriteReindex       = "memory.write.reindex"
	MemoryWriteReverted      = "memory.write.reverted"
	MemoryProviderCollision  = "memory.provider.collision"
	MemoryRecallExecuted     = "memory.recall.executed"
	MemoryRecallSkipped      = "memory.recall.skipped"
	MemoryRecallDropped      = "memory.recall.signal_dropped"
	MemoryRecallFailed       = "memory.recall.signal_update_failed"
	MemoryDecisionsSummary   = "memory.decisions.audit_summarized"
	MemoryDecisionsPruned    = "memory.decisions.pruned"
	MemoryDreamStarted       = "memory.dream.run.started"
	MemoryDreamPromoted      = "memory.dream.run.promoted"
	MemoryDreamFailed        = "memory.dream.run.failed"
	MemoryExtractorStarted   = "memory.extractor.started"
	MemoryExtractorComplete  = "memory.extractor.completed"
	MemoryExtractorFailed    = "memory.extractor.failed"
	MemoryExtractorCoalesced = "memory.extractor.coalesced"
	MemoryExtractorDropped   = "memory.extractor.dropped"
	MemoryDailyRotated       = "memory.daily.rotated"
	MemoryDailyArchived      = "memory.daily.archived"
	MemoryDailyRestored      = "memory.daily.restored"
	MemoryDailyPurged        = "memory.daily.purged"
	MemoryDailyArchivePurged = "memory.daily.archive_purged"
	MemoryProviderEnabled    = "memory.provider.enabled"
	MemoryProviderDisabled   = "memory.provider.disabled"
	MemoryWorkspaceRelocated = "memory.workspace.relocated"
	MemoryWorkspaceRecovered = "memory.workspace.recovered"
	MemoryAgentPurged        = "memory.agent.purged"
	MemoryMigrationApplied   = "memory.migration.applied"

	SchedulerPaused         = "scheduler.paused"
	SchedulerResumed        = "scheduler.resumed"
	SchedulerDrainStarted   = "scheduler.drain_started"
	SchedulerDrainCompleted = "scheduler.drain_completed"

	AutomationSuggestionAccepted  = "automation.suggestion.accepted"
	AutomationSuggestionDismissed = "automation.suggestion.dismissed"

	TranscriptMarkerCreated       = "transcript_marker.created"
	TranscriptMarkerRedacted      = "transcript_marker.redacted"
	SessionTranscriptCacheRebuilt = "session_transcript_cache_rebuilt"
	SessionStreamSnapshotServed   = "session_stream_snapshot_served"
	SessionStreamSubscribed       = "session_stream_subscribed"
	SessionStreamOverflowFallback = "session_stream_overflow_fallback"

	ToolCallStarted          = "tool.call_started"
	ToolCallCompleted        = "tool.call_completed"
	ToolCallFailed           = "tool.call_failed"
	ToolCallDenied           = "tool.call_denied"
	ToolResultTruncated      = "tool.result_truncated"
	ToolApprovalGrantPut     = "tool.approval_grant_put"
	ToolApprovalGrantRevoked = "tool.approval_grant_revoked"

	ProviderAuthRequired          = "provider.auth_required"
	ProviderAuthRecovered         = "provider.auth_recovered"
	ProviderRateLimited           = "provider.rate_limited"
	ProviderPermissionDenied      = "provider.permission_denied"
	ProviderUnavailable           = "provider.unavailable"
	ProviderModelCatalogRefreshed = "provider.model_catalog_refreshed"

	DeadEntityMarked  = "reliability.dead_entity_marked"
	DeadEntityCleared = "reliability.dead_entity_cleared"

	BridgeNotificationSuppressed = "bridge_notification_suppressed"
	NetworkPeerJoined            = "network.peer.joined"
	NetworkPeerLeft              = "network.peer.left"

	NotificationPresetCreated        = "notification.preset_created"
	NotificationPresetUpdated        = "notification.preset_updated"
	NotificationPresetDeleted        = "notification.preset_deleted"
	NotificationPresetDispatchFailed = "notification.preset_dispatch_failed"
)

var baseRegistryEntries = []Metadata{
	info(ACPUserMessage, "session", ComponentSession),
	info(ACPSyntheticReentry, "session", ComponentSession),
	info(ACPAgentMessage, "session", ComponentSession),
	info(ACPThought, "session", ComponentSession),
	info(ACPToolCall, "session", ComponentSession),
	info(ACPToolResult, "session", ComponentSession),
	info(ACPPlan, "session", ComponentSession),
	info(ACPPermission, "session", ComponentSession),
	info(ACPUsage, "session", ComponentSession),
	info(ACPSystem, "session", ComponentSession),
	info(ACPRuntimeProgress, "session", ComponentSession),
	warning(ACPRuntimeWarning, "session", ComponentSession),
	success(ACPDone, "session", ComponentSession),
	failure(ACPError, "session", ComponentSession),
	info(SessionStopped, "session", ComponentSession),
	notify(warning(SessionUnhealthy, "session", ComponentSession)),
	notify(warning(SessionHung, "session", ComponentSession)),
	notify(success(SessionRecovered, "session", ComponentSession)),
	info(SessionCompactionFired, "session", ComponentSession),

	info(TaskCreated, "task", ComponentTask),
	info(TaskUpdated, "task", ComponentTask),
	info(TaskPublished, "task", ComponentTask),
	success(TaskApproved, "task", ComponentTask),
	warning(TaskRejected, "task", ComponentTask),
	warning(TaskCanceled, "task", ComponentTask),
	info(TaskChildCreated, "task", ComponentTask),
	info(TaskDependencyAdded, "task", ComponentTask),
	info(TaskDependencyRemoved, "task", ComponentTask),
	warning(TaskPaused, "task", ComponentTask),
	info(TaskResumed, "task", ComponentTask),
	info(TaskBlockCreated, "task.block", ComponentTask),
	info(TaskBlockCleared, "task.block", ComponentTask),
	info(TaskBlockExpired, "task.block", ComponentTask),
	notify(warning(TaskNeedsAttention, "task", ComponentTask)),
	info(TaskRecovered, "task", ComponentTask),
	info(TaskRunEnqueued, "task", ComponentTask),
	info(TaskRunClaimed, "task", ComponentTask),
	info(TaskRunStarting, "task", ComponentTask),
	info(TaskRunSessionBound, "task", ComponentTask),
	success(TaskRunStarted, "task", ComponentTask),
	notify(success(TaskRunCompleted, "task", ComponentTask)),
	notify(failure(TaskRunFailed, "task", ComponentTask)),
	notify(warning(TaskRunCanceled, "task", ComponentTask)),
	warning(TaskRunForceStopped, "task", ComponentTask),
	warning(TaskRunRecovered, "task", ComponentTask),
	warning(TaskRunRejected, "task", ComponentTask),
	info(TaskRunLeaseExtended, "task", ComponentTask),
	warning(TaskRunLeaseExpired, "task", ComponentTask),
	info(TaskRunReleased, "task", ComponentTask),
	notify(failure(TaskRunOperatorForcedFail, "task", ComponentTask)),
	notify(info(TaskRunOperatorRetry, "task", ComponentTask)),
	notify(info(TaskRunRecoveredFromAttention, "task", ComponentTask)),
	warning(TaskRunStarved, "task", ComponentTask),
	notify(warning(TaskRunNeedsAttention, "task", ComponentTask)),
	info(TaskExecutionProfileUpdated, "task", ComponentTask),
	info(TaskExecutionProfileDeleted, "task", ComponentTask),
	info(TaskRunReviewRequested, "task", ComponentTask),
	info(TaskRunReviewBound, "task", ComponentTask),
	info(TaskRunReviewRecorded, "task", ComponentTask),
	notify(success(TaskRunReviewApproved, "task", ComponentTask)),
	notify(failure(TaskRunReviewRejected, "task", ComponentTask)),
	notify(warning(TaskRunReviewBlocked, "task", ComponentTask)),
	notify(failure(TaskRunReviewError, "task", ComponentTask)),
	notify(warning(TaskRunReviewTimeout, "task", ComponentTask)),
	notify(failure(TaskRunReviewInvalidOutput, "task", ComponentTask)),
	info(TaskRunReviewRetryEnqueued, "task", ComponentTask),
	info(TaskAutoEnqueueTriggered, "task.auto_enqueue", ComponentTask),
	notify(warning(TaskCompletionHallucinationBlocked, "task.completion", ComponentTask)),
	warning(TaskCompletionHallucinationSuspected, "task.completion", ComponentTask),
	info(TaskWakeDelivered, "task.wake", ComponentTask),
	warning(TaskWakeSuppressed, "task.wake", ComponentTask),
	global(info(SettingsChanged, "settings", ComponentConfig)),
	global(info(RoleFallbackUsed, "role", ComponentRole)),
	global(failure(RoleResolveError, "role", ComponentRole)),
	global(warning(SkillShadowed, "skill", ComponentSkill)),
	global(failure(SkillLoadFailed, "skills", ComponentSkill)),
	global(info(HookDispatchStart, "hook.dispatch", ComponentHook)),
	global(info(HookDispatchComplete, "hook.dispatch", ComponentHook)),
	global(info(CoordinatorSpawned, "coordinator", ComponentHook)),
	global(info(CoordinatorDecision, "coordinator", ComponentHook)),
	global(info(CoordinatorStopped, "coordinator", ComponentHook)),
	global(failure(CoordinatorFailed, "coordinator", ComponentHook)),

	info(HarnessContextResolved, "harness", ComponentHarness),
	info(HarnessSectionSelected, "harness", ComponentHarness),
	info(HarnessAugmenterApplied, "harness", ComponentHarness),
	warning(HarnessAugmenterFailed, "harness", ComponentHarness),
	success(HarnessDetachedRunCompleted, "harness", ComponentHarness),
	info(HarnessSyntheticReentryEmitted, "harness", ComponentHarness),
	warning(HarnessSyntheticReentryDropped, "harness", ComponentHarness),

	success(MemoryWriteCommitted, "memory.write", ComponentMemory),
	warning(MemoryWriteRejected, "memory.write", ComponentMemory),
	warning(MemoryWriteShadowed, "memory.write", ComponentMemory),
	info(MemoryWriteReindex, "memory.write", ComponentMemory),
	warning(MemoryWriteReverted, "memory.write", ComponentMemory),
	success(MemoryRecallExecuted, "memory.recall", ComponentMemory),
	info(MemoryRecallSkipped, "memory.recall", ComponentMemory),
	warning(MemoryRecallDropped, "memory.recall", ComponentMemory),
	failure(MemoryRecallFailed, "memory.recall", ComponentMemory),
	success(MemoryDecisionsSummary, "memory.decisions", ComponentMemory),
	info(MemoryDecisionsPruned, "memory.decisions", ComponentMemory),
	info(MemoryDreamStarted, "memory.dream", ComponentMemory),
	success(MemoryDreamPromoted, "memory.dream", ComponentMemory),
	failure(MemoryDreamFailed, "memory.dream", ComponentMemory),
	info(MemoryExtractorStarted, "memory.extractor", ComponentMemory),
	success(MemoryExtractorComplete, "memory.extractor", ComponentMemory),
	failure(MemoryExtractorFailed, "memory.extractor", ComponentMemory),
	info(MemoryExtractorCoalesced, "memory.extractor", ComponentMemory),
	warning(MemoryExtractorDropped, "memory.extractor", ComponentMemory),
	success(MemoryDailyRotated, "memory.daily", ComponentMemory),
	success(MemoryDailyArchived, "memory.daily", ComponentMemory),
	success(MemoryDailyRestored, "memory.daily", ComponentMemory),
	warning(MemoryDailyPurged, "memory.daily", ComponentMemory),
	warning(MemoryDailyArchivePurged, "memory.daily", ComponentMemory),
	success(MemoryProviderEnabled, "memory.provider", ComponentMemory),
	warning(MemoryProviderDisabled, "memory.provider", ComponentMemory),
	global(warning(MemoryProviderCollision, "memory.provider", ComponentMemory)),
	info(MemoryWorkspaceRelocated, "memory.workspace", ComponentMemory),
	success(MemoryWorkspaceRecovered, "memory.workspace", ComponentMemory),
	warning(MemoryAgentPurged, "memory.agent", ComponentMemory),
	success(MemoryMigrationApplied, "memory.migration", ComponentMemory),

	notify(global(warning(SchedulerPaused, "scheduler", ComponentScheduler))),
	global(info(SchedulerResumed, "scheduler", ComponentScheduler)),
	global(info(SchedulerDrainStarted, "scheduler", ComponentScheduler)),
	global(success(SchedulerDrainCompleted, "scheduler", ComponentScheduler)),
	global(success(AutomationSuggestionAccepted, "automation.suggestion", ComponentAutomation)),
	global(info(AutomationSuggestionDismissed, "automation.suggestion", ComponentAutomation)),

	info(TranscriptMarkerCreated, "transcript_marker", ComponentTranscript),
	warning(TranscriptMarkerRedacted, "transcript_marker", ComponentTranscript),
	info(SessionTranscriptCacheRebuilt, "transcript_cache", ComponentTranscript),
	info(SessionStreamSnapshotServed, "transcript_stream", ComponentTranscript),
	info(SessionStreamSubscribed, "transcript_stream", ComponentTranscript),
	warning(SessionStreamOverflowFallback, "transcript_stream", ComponentTranscript),

	global(info(ToolCallStarted, "tool", ComponentTools)),
	global(success(ToolCallCompleted, "tool", ComponentTools)),
	notify(global(failure(ToolCallFailed, "tool", ComponentTools))),
	notify(global(warning(ToolCallDenied, "tool", ComponentTools))),
	notify(global(warning(ToolResultTruncated, "tool", ComponentTools))),
	global(success(ToolApprovalGrantPut, "tool.approval_grant", ComponentTools)),
	global(warning(ToolApprovalGrantRevoked, "tool.approval_grant", ComponentTools)),

	notify(global(warning(ProviderAuthRequired, "provider", ComponentProvider))),
	global(success(ProviderAuthRecovered, "provider", ComponentProvider)),
	notify(global(warning(ProviderRateLimited, "provider", ComponentProvider))),
	notify(global(failure(ProviderPermissionDenied, "provider", ComponentProvider))),
	notify(global(failure(ProviderUnavailable, "provider", ComponentProvider))),
	global(success(ProviderModelCatalogRefreshed, "provider", ComponentProvider)),
	global(info(MarketplaceCatalogRefresh, "marketplace.catalog", ComponentMarketplace)),
	global(info(MarketplaceInstall, "marketplace.install", ComponentMarketplace)),

	notify(global(success(ExtensionInstalled, "extension", ComponentExtension))),
	notify(global(success(ExtensionUpdated, "extension", ComponentExtension))),
	notify(global(warning(ExtensionRemoved, "extension", ComponentExtension))),
	global(success(ExtensionEnabled, "extension", ComponentExtension)),
	global(warning(ExtensionDisabled, "extension", ComponentExtension)),
	global(info(ExtensionDigestVerify, "extension.digest", ComponentExtension)),
	global(warning(DeadEntityMarked, "reliability.dead_entity", ComponentReliability)),
	global(success(DeadEntityCleared, "reliability.dead_entity", ComponentReliability)),

	global(success(NotificationPresetCreated, "notification.preset", ComponentNotification)),
	global(info(NotificationPresetUpdated, "notification.preset", ComponentNotification)),
	global(warning(NotificationPresetDeleted, "notification.preset", ComponentNotification)),
	global(failure(NotificationPresetDispatchFailed, "notification.preset", ComponentNotification)),
	notify(global(warning(BridgeNotificationSuppressed, "bridge_notification", ComponentNotification))),
	notify(global(success(NetworkPeerJoined, "network.peer", ComponentNetwork))),
	notify(global(warning(NetworkPeerLeft, "network.peer", ComponentNetwork))),
}

var registryByName = mustBuildRegistry(registryEntries)

// ValidatePublicName rejects deleted or unsupported public event families.
func ValidatePublicName(name string) error {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return nil
	}
	if strings.HasPrefix(trimmed, "task_run.") {
		return fmt.Errorf("events: %q is not a public event family; use task.run_* events", trimmed)
	}
	return nil
}

// ValidOutcome reports whether value is one of the canonical event outcomes.
func ValidOutcome(value string) bool {
	switch Outcome(strings.TrimSpace(value)) {
	case "", OutcomeInfo, OutcomeSuccess, OutcomeFailure, OutcomeWarning:
		return true
	default:
		return false
	}
}

// ValidComponent reports whether component is present in the registry.
func ValidComponent(component string) bool {
	component = strings.TrimSpace(component)
	if component == "" {
		return true
	}
	for _, meta := range registryEntries {
		if meta.Component == component {
			return true
		}
	}
	return false
}

func mustBuildRegistry(entries []Metadata) map[string]Metadata {
	registry := make(map[string]Metadata, len(entries))
	for _, entry := range entries {
		name := strings.TrimSpace(entry.Name)
		if name == "" {
			panic("events: registry entry missing name")
		}
		if _, exists := registry[name]; exists {
			panic("events: duplicate registry entry " + name)
		}
		if strings.TrimSpace(entry.Family) == "" {
			panic("events: registry entry missing family for " + name)
		}
		if strings.TrimSpace(entry.Component) == "" {
			panic("events: registry entry missing component for " + name)
		}
		if !ValidOutcome(string(entry.Outcome)) || entry.Outcome == "" {
			panic("events: registry entry has invalid outcome for " + name)
		}
		entry.Name = name
		entry.Family = strings.TrimSpace(entry.Family)
		entry.Component = strings.TrimSpace(entry.Component)
		registry[name] = entry
	}
	return registry
}
