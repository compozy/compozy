// Package loop owns loop-domain validation and compile surfaces.
package loop

import (
	"encoding/json"

	"github.com/compozy/compozy/internal/loop/dsl"
)

const (
	// LoopMaxGateRevisions is the absolute structural gate revision ceiling.
	LoopMaxGateRevisions = dsl.GateMaxRevisionsCeiling
	// LoopMaxNoProgressWindow is the compile-time generation no-progress ceiling.
	LoopMaxNoProgressWindow = 30
	// LoopFailureBreakerLimit is the compile-time consecutive-failure breaker ceiling.
	LoopFailureBreakerLimit = 2
	// LoopMaxAncestryDepth is the maximum run-loop parent chain depth.
	LoopMaxAncestryDepth = 8
)

// Linter validates loop definitions without performing IO.
type Linter interface {
	Lint(def dsl.Definition) []LintError
}

// LintSeverity classifies a lint result.
type LintSeverity string

const (
	// SeverityError blocks publish/compile.
	SeverityError LintSeverity = "error"
	// SeverityWarning is diagnostic-only.
	SeverityWarning LintSeverity = "warning"
)

// LintError is the per-node shape surfaced to authoring clients.
type LintError struct {
	NodeID   dsl.NodeID   `json:"node_id,omitempty"`
	Path     string       `json:"path,omitempty"`
	Code     string       `json:"code"`
	Message  string       `json:"message"`
	Severity LintSeverity `json:"severity"`
}

const (
	// CodeCycle reports a graph cycle.
	CodeCycle = "cycle"
	// CodeUnreachableNode reports a node unreachable from source roots.
	CodeUnreachableNode = "unreachable_node"
	// CodeNonTerminatingStructure reports a body with no terminal path.
	CodeNonTerminatingStructure = "non_terminating_structure"
	// CodeFanOutUnbounded reports a fan-out without finite materialization bounds.
	CodeFanOutUnbounded = "fan_out_unbounded"
	// CodeStrategyCoverageUndeclared reports partial coverage without explicit author consent.
	CodeStrategyCoverageUndeclared = "strategy_coverage_undeclared"
	// CodeStrategyThresholdInvalid reports a malformed or misplaced strategy threshold.
	CodeStrategyThresholdInvalid = "strategy_threshold_invalid"
	// CodeStrategyWaitAllEquivalent hints that a 100-percent quorum is wait_all.
	CodeStrategyWaitAllEquivalent = "strategy_wait_all_equivalent"
	// CodeIterationNameConflict reports colliding or reserved fan-out iteration names.
	CodeIterationNameConflict = "iteration_name_conflict"
	// CodeGateMaxRevisionsCeilingExceeded reports gate max_revisions beyond its ceiling.
	CodeGateMaxRevisionsCeilingExceeded = "gate_max_revisions_ceiling_exceeded"
	// CodeNodeIDInvalid reports a non-snake_case node id.
	CodeNodeIDInvalid = "node_id_invalid"
	// CodeVerdictPolicyRequiresJudge reports revise_until_clean without a judge/human source.
	CodeVerdictPolicyRequiresJudge = "verdict_policy_requires_judge"
	// CodeMetricSingle reports multiple metric criteria in one definition.
	CodeMetricSingle = "metric_single"
	// CodeMetricMachineCriterionRequired reports a metric on a non-machine criterion.
	CodeMetricMachineCriterionRequired = "metric_machine_criterion_required"
	// CodeMetricDirectionRequired reports a metric without a direction.
	CodeMetricDirectionRequired = "metric_direction_required"
	// CodeMetricDirectionInvalid reports a direction outside the closed metric enum.
	CodeMetricDirectionInvalid = "metric_direction_invalid"
	// CodeMetricMinDeltaInvalid reports a negative or non-finite metric delta.
	CodeMetricMinDeltaInvalid = "metric_min_delta_invalid"
	// CodeInvalidHarvest reports an unsupported or malformed harvest policy.
	CodeInvalidHarvest = "invalid_harvest"
	// CodeUnknownActionKind reports action kinds that are neither reserved nor resolvable ToolIDs.
	CodeUnknownActionKind = "unknown_action_kind"
	// CodeUnknownControlKind reports a control kind outside the closed enum.
	CodeUnknownControlKind = "unknown_control_kind"
	// CodeUnknownSourceKind reports a source kind outside the closed enum.
	CodeUnknownSourceKind = "unknown_source_kind"
	// CodeWatchKindRequired reports a watch-source missing its extension source kind.
	CodeWatchKindRequired = "watch_kind_required"
	// CodeWatchEventsSubscriptionRequired reports a watch-events node with no subscriptions.
	CodeWatchEventsSubscriptionRequired = "watch_events_subscription_required"
	// CodeWatchEventsKindUnknown reports a watch-events subscription kind outside the hook catalog.
	CodeWatchEventsKindUnknown = "watch_events_kind_unknown"
	// CodeWatchEventsKindUnsupported reports a hook catalog kind outside the watch-events registry.
	CodeWatchEventsKindUnsupported = "watch_events_kind_unsupported"
	// CodeWatchEventsFilterInvalid reports a CEL filter that cannot compile in the event env.
	CodeWatchEventsFilterInvalid = "watch_events_filter_invalid"
	// CodeWatchEventsFilterTooBroad is reserved for registry RequiredVars gates.
	CodeWatchEventsFilterTooBroad = "watch_events_filter_too_broad"
	// CodeWatchEventsShapeInvalid reports events/watch/produces shape contradictions.
	CodeWatchEventsShapeInvalid = "watch_events_shape_invalid"
	// CodeFileImportParseRequired reports a missing or unsupported file-import parse mode.
	CodeFileImportParseRequired = "file_import_parse_required"
	// CodeDuplicateNodeID reports repeated node IDs.
	CodeDuplicateNodeID = "duplicate_node_id"
	// CodeUnknownTerminalState reports contract terminal states outside the closed enum.
	CodeUnknownTerminalState = "unknown_terminal_state"
	// CodeUnknownParameter reports a removed or unsupported authoring field.
	CodeUnknownParameter = "unknown_parameter"
	// CodeEvalErrorPolicyInvalid reports an on_eval_error value or placement outside its closed grammar.
	CodeEvalErrorPolicyInvalid = "eval_error_policy_invalid"
	// CodeRouteDefaultMissing reports a route node without its required default destination.
	CodeRouteDefaultMissing = "route_default_missing"
	// CodeRouteTargetInvalid reports an unknown, backward, duplicate, or undeclared route destination.
	CodeRouteTargetInvalid = "route_target_invalid"
	// CodeRouteMappingInvalid reports a gate outcome mapping outside the closed route grammar.
	CodeRouteMappingInvalid = "route_mapping_invalid"
	// CodeRouteActionRemoved reports the deleted branch action and points authors to object-form routing.
	CodeRouteActionRemoved = "route_action_removed"
	// CodeGoalJudgeRequired reports a Goal without at least one valid supported judge.
	CodeGoalJudgeRequired = "goal_judge_required"
	// CodeGoalObjectiveRequired reports a Goal without a non-empty objective.
	CodeGoalObjectiveRequired = "goal_objective_required"
	// CodeGoalMaxTurnsRequired reports a Goal without a positive authored turn limit.
	CodeGoalMaxTurnsRequired = "goal_max_turns_required"
	// CodeGoalOutputStatusMissingBlocked reports a Goal output schema that cannot represent blocked.
	CodeGoalOutputStatusMissingBlocked = "goal_output_status_missing_blocked"
	// CodeGoalOutputStatusMissingComplete reports a Goal output schema that cannot represent completion.
	CodeGoalOutputStatusMissingComplete = "goal_output_status_missing_complete"
	// CodeGoalOnExhaustedInvalid reports an unsupported Goal exhaustion policy.
	CodeGoalOnExhaustedInvalid = "goal_on_exhausted_invalid"
	// CodeGoalHumanJudgeUnsupported reports a human Goal judge, which v1 cannot lease safely.
	CodeGoalHumanJudgeUnsupported = "goal_human_judge_unsupported"
	// CodeContinuousForbidsParallel reports a continuous Goal inside parallel fan-out.
	CodeContinuousForbidsParallel = "continuous_forbids_parallel"
	// CodeRetryFreshSessionRequiresContinuous reports an invalid fresh-session retry policy.
	CodeRetryFreshSessionRequiresContinuous = "retry_fresh_session_requires_continuous"
	// CodeSessionSpecAmbiguous reports a session envelope that is not exactly isolated or continuous.
	CodeSessionSpecAmbiguous = "session_spec_ambiguous"
	// CodeContinuousHandleReused reports duplicate continuous Goal display identities.
	CodeContinuousHandleReused = "continuous_handle_reused"
	// CodeRetryMaxUnsupported reports the retired retry.max key.
	CodeRetryMaxUnsupported = "retry_max_unsupported"
	// CodeEnvironmentCWDRemoved reports the retired action cwd field.
	CodeEnvironmentCWDRemoved = "environment_cwd_removed"
	// CodeEnvironmentInvalid reports a malformed EnvironmentSpec.
	CodeEnvironmentInvalid = "environment_invalid"
	// CodeEnvironmentUnsupported reports Environment on a node that cannot start agents.
	CodeEnvironmentUnsupported = "environment_unsupported"
	// CodeNetworkParticipationInvalid reports malformed authored participation intent.
	CodeNetworkParticipationInvalid = "network_participation_invalid"
	// CodeLoopRequiresLive reports a Network-using graph without authored Live participation.
	CodeLoopRequiresLive = "loop_requires_live"
	// CodeErrorRouteBackward reports an error route that is not a direct forward edge.
	CodeErrorRouteBackward = "error_route_backward"
	// CodeErrorRouteConflict reports route and allow_fail on the same error policy.
	CodeErrorRouteConflict = "error_route_conflict"
	// CodeErrorRouteDead warns about an error route on an infallible node.
	CodeErrorRouteDead = "error_route_dead"
	// CodeRetryOnGoalNode reports generic lifecycle retry fields on a Goal node.
	CodeRetryOnGoalNode = "retry_on_goal_node"
	// CodeTimeoutExceedsDeadline reports a per-attempt timeout beyond the node deadline.
	CodeTimeoutExceedsDeadline = "timeout_exceeds_deadline"
	// CodeDurationInvalid reports an invalid or non-positive lifecycle duration.
	CodeDurationInvalid = "duration_invalid"
	// CodeResultContractInvalid reports a result contract that cannot resolve against output.
	CodeResultContractInvalid = "result_contract_invalid"
	// CodeEffectShapeInvalid reports an effect that is not exactly one emit or tool call.
	CodeEffectShapeInvalid = "effect_shape_invalid"
	// CodeEffectToolUnknown warns that an effect tool is absent from the schema snapshot.
	CodeEffectToolUnknown = "effect_tool_unknown"
	// CodeWaitShapeInvalid reports an invalid wait discriminator or expiry shape.
	CodeWaitShapeInvalid = "wait_shape_invalid"
	// CodeWaitExpiryWithoutPath warns that expiry can only surface needs-attention.
	CodeWaitExpiryWithoutPath = "wait_expiry_without_path"
	// CodeAskExpectRequired reports an ask without an answer schema.
	CodeAskExpectRequired = "ask_expect_required"
	// CodeResponderPolicyInvalid reports an unsupported responders.agents value.
	CodeResponderPolicyInvalid = "responder_policy_invalid"
	// CodeReviewShapeInvalid reports an invalid action review grammar.
	CodeReviewShapeInvalid = "review_shape_invalid"
	// CodeReviewRespondSchemaRequired reports respond without a declared action output.
	CodeReviewRespondSchemaRequired = "review_respond_schema_required"
	// CodeWatchIdentityRequired reports a watch source without stable event identity support.
	CodeWatchIdentityRequired = "watch_identity_required"
	// CodeParentCloseInvalid reports parent-close policy outside run-loop or its closed enum.
	CodeParentCloseInvalid = "parent_close_invalid"
	// CodeAutopauseRuleInvalid identifies a config-side autopause compilation failure.
	CodeAutopauseRuleInvalid = "autopause_rule_invalid"
)

// ToolSchemaSnapshot is the pure tool-schema view consumed by lint and compile.
type ToolSchemaSnapshot struct {
	ToolID             string          `json:"tool_id"`
	InputSchema        json.RawMessage `json:"input_schema,omitempty"`
	OutputSchema       json.RawMessage `json:"output_schema,omitempty"`
	InputSchemaDigest  string          `json:"input_schema_digest,omitempty"`
	OutputSchemaDigest string          `json:"output_schema_digest,omitempty"`
}

// ToolSchemaSource resolves open action ToolIDs without tying lint to runtime IO.
type ToolSchemaSource interface {
	Snapshot(toolID string) (ToolSchemaSnapshot, bool)
}

// WatchIdentitySource reports whether a watch source kind provides stable event identity.
type WatchIdentitySource interface {
	SupportsStableWatchIdentity(kind string) bool
}
