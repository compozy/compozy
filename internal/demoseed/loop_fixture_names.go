package demoseed

// Loop run-event kinds and payload keys the fixtures replay verbatim.
const (
	evtGenerationStarted = "generation_started"
	evtNodeRunning       = "node_running"
	evtNodeSucceeded     = "node_succeeded"
	evtNodeFailed        = "node_failed"
	evtNodeCanceled      = "node_canceled"
	evtNodeQuarantined   = "node_quarantined"
	evtNodeRetry         = "node_retry_scheduled"
	evtNodeWaitStarted   = "node_wait_started"
	evtNodePaused        = "node_paused"
	evtGateVerdict       = "gate_verdict"
	evtTokenTick         = "token_tick"
	evtNeedsApproval     = "needs_approval"
	evtStatusChanged     = "status_changed"
)

const (
	keyGeneration   = "generation"
	keyNodeID       = "node_id"
	keyItemIndex    = "item_index"
	keyReason       = "reason"
	keyStatus       = "status"
	keyFrom         = "from"
	keyTo           = "to"
	keyCause        = "cause"
	keyVerdict      = "verdict"
	keyValue        = "value"
	keyTokensUsed   = "tokens_used"
	keyTerminal     = "terminal"
	keyKind         = "kind"
	keyWaitKind     = "wait_kind"
	keyScheduleKind = "schedule_kind"
	keyMarket       = "market"
)

// Loop run statuses and generation origins the fixtures declare.
const (
	runDone          = "done"
	runNoOp          = "no-op"
	runBlocked       = "blocked"
	runFailed        = "failed"
	runExhausted     = "exhausted"
	runStalled       = "stalled"
	runCanceled      = "canceled"
	runQueued        = "queued"
	runNeedsApproval = "needs-approval"
	runWatching      = "watching"
	runPaused        = "paused"
	originInitial    = "initial"
	originGateNext   = "gate_next_generation"
)

const (
	causeContract       = "contract"
	causeOperatorCancel = "operator_cancel"
	causeNoProgress     = "no_progress"
	causeBudget         = "budget"
	causeApproval       = "approval"
	causeWatchEvents    = "watch_events"
	causePauseBoundary  = "pause_boundary"
)

// Node identifiers, mirrored from the embedded Loop definitions.
const (
	nodeMarketInput      = "market_input"
	nodeAssess           = "assess"
	nodeHasBlockers      = "has_blockers"
	nodeDecisionGate     = "decision_gate"
	nodeMarketsInput     = "markets_input"
	nodeSplitMarkets     = "split_markets"
	nodeReviewFanout     = "review_fanout"
	nodeReviewMarket     = "review_market"
	nodeCollectReviews   = "collect_reviews"
	nodePromotionGate    = "promotion_gate"
	nodeSettlementBatch  = "settlement_batch"
	nodeIncidentInput    = "incident_input"
	nodeWritePostmortem  = "write_postmortem"
	nodeReadiness        = "readiness"
	nodeSettlementAudit  = "settlement_audit"
	nodeDisputeFiles     = "dispute_files"
	nodeBundleFanout     = "bundle_fanout"
	nodeBuildBundle      = "build_bundle"
	nodeCollectBundles   = "collect_bundles"
	nodePartnerInput     = "partner_input"
	nodeAudit            = "audit"
	nodeOperatorSignoff  = "operator_signoff"
	nodeAreaInput        = "area_input"
	nodeFlagStale        = "flag_stale"
	partnerEvidenceTgt   = "http:partner-evidence"
	partnerTimeoutCause  = "partner evidence endpoint timed out"
	quarantineLaneReason = "three consecutive failures on the same lane"
)
