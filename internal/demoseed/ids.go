package demoseed

const (
	launchWorkspaceName     = "Northstar Pay Launch HQ"
	launchWorkspaceRelative = "workspaces/northstar-pay/launch-hq"
	platformWorkspaceName   = "Northstar Pay Platform"
	platformWorkspaceRelat  = "workspaces/northstar-pay/platform"
)

const (
	agentProductLead      = "product-lead"
	agentPlatformEngineer = "platform-engineer"
	agentComplianceReview = "compliance-review"
	agentSupportLead      = "support-lead"
	agentCheckoutEngineer = "checkout-engineer"
	agentFraudAnalyst     = "fraud-analyst"
	agentReleaseManager   = "release-manager"
	agentDocsSteward      = "docs-steward"
)

const (
	providerClaude = "claude"
	providerCodex  = "codex"
	modelClaude    = "claude-sonnet-5"
	modelCodex     = "gpt-5.6-codex"
)

const (
	approveReads    = "approve-reads"
	approveAll      = "approve-all"
	toolNetworkSend = "compozy__network_send"
	toolTaskList    = "compozy__task_list"
	toolTaskRead    = "compozy__task_read"
	toolTaskUpdate  = "compozy__task_update"
	toolMemoryStore = "compozy__memory_store"
	toolLoopStatus  = "compozy__loop_status"
	toolKindRead    = "read"
	toolKindEdit    = "edit"
	toolKindExecute = "execute"
	toolKindSearch  = "search"
	toolKindOther   = "other"
)

// Tool names the session transcript renderers dispatch on.
const (
	toolBash      = "Bash"
	toolRead      = "Read"
	toolWrite     = "Write"
	toolEdit      = "Edit"
	toolGrep      = "Grep"
	toolTodoWrite = "TodoWrite"
)

const (
	taskStatusCompleted     = "completed"
	taskStatusBlocked       = "blocked"
	taskStatusReady         = "ready"
	taskStatusInProgress    = "in_progress"
	taskStatusPending       = "pending"
	taskStatusDraft         = "draft"
	taskStatusFailed        = "failed"
	taskStatusCanceled      = "canceled"
	taskStatusNeedsAttn     = "needs_attention"
	taskPriorityUrgent      = "urgent"
	taskPriorityHigh        = "high"
	taskPriorityMedium      = "medium"
	taskPriorityLow         = "low"
	approvalPolicyNone      = "none"
	approvalPolicyManual    = "manual"
	approvalNotRequired     = "not_required"
	approvalPending         = "pending"
	ownerAgentSession       = "agent_session"
	ownerHuman              = "human"
	ownerPool               = "pool"
	ownerAutomation         = "automation"
	operatorRef             = "operator:pedro"
	originWebRef            = "launch-console"
	poolPlatform            = "platform"
	poolCheckout            = "checkout"
	categoryEngineering     = "Engineering"
	categoryOperations      = "Operations"
	categoryRisk            = "Risk"
	taskSupportID           = "task_northstar_support_handoff"
	taskLaunchDecisionID    = "task_northstar_launch_decision"
	taskPlatformReleaseID   = "task_northstar_release_train"
	taskPlatformQuarantine  = "task_northstar_settlement_retry"
	taskPlatformDocsRefresh = "task_northstar_docs_refresh"
)

const (
	networkRootMessageID = "msg_northstar_01"
	jsonTextKey          = "text"
	launchChannel        = "launch-war-room"
	launchThreadID       = "thread_checkout_launch"
	launchAutomationID   = "job_northstar_canary_digest"
	launchAutomationRun  = "autorun_northstar_canary_digest"
	digestAutomationID   = "job_northstar_settlement_watch"
	digestAutomationRun  = "autorun_northstar_settlement_watch"
	worktreeID           = "wt_northstar_settlement_retry"
)

const (
	cellSucceeded   = "succeeded"
	cellRunning     = "running"
	cellPending     = "pending"
	cellWaiting     = "waiting"
	cellPaused      = "paused"
	cellPartial     = "partial"
	cellQuarantined = "quarantined"
	cellFailed      = "failed"
	cellCanceled    = "canceled"
)

const (
	loopLaunchReadiness             = "launch-readiness"
	loopMarketRollout               = "market-rollout-review"
	loopChargebackTriage            = "chargeback-triage"
	loopIncidentPostmort            = "incident-postmortem"
	loopReleaseTrain                = "release-train"
	loopDisputeSweep                = "dispute-evidence-sweep"
	loopSettlementAudit             = "partner-settlement-audit"
	loopDocsFreshness               = "docs-freshness"
	loopApprovalRunID               = "loopr_northstar_settlement_audit"
	loopWatchingRunID               = "loopr_northstar_chargeback_watch"
	loopRunningRunID                = "loopr_northstar_market_rollout"
	loopFailedRunID                 = "loopr_northstar_dispute_sweep_failed"
	loopGoalRunID                   = "loopr_northstar_incident_postmortem"
	loopReleaseParentRun            = "loopr_northstar_release_train"
	loopReleaseChildRunID           = "loopr_northstar_release_child"
	loopReleaseSettlementChildRunID = "loopr_northstar_release_settlement_child"
)

var scenarioSessionIDs = []string{
	"sess_northstar_partner_replay",
	"sess_northstar_compliance_copy",
	"sess_northstar_support_handoff",
	"sess_northstar_launch_decision",
	"sess_northstar_fraud_sweep",
	"sess_northstar_rollback_drill",
	"sess_northstar_settlement_retry",
	"sess_northstar_settlement_worker",
	"sess_northstar_release_train",
	"sess_northstar_docs_refresh",
	"sess_northstar_chargeback_watch",
	"sess_northstar_incident_review",
}
