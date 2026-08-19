import type { OperationQuery, OperationRequestBody, OperationResponse } from "@/lib/api-contract";

// Catalog + definition ------------------------------------------------------

export type LoopsListResponse = OperationResponse<"listLoops", 200>;
export type LoopCatalogFilter = OperationQuery<"listLoops">;
export type LoopCatalogStableFilter = Omit<LoopCatalogFilter, "cursor">;
export type LoopCatalogEntry = LoopsListResponse["loops"][number];
export type LoopCatalogInfo = LoopCatalogEntry["catalog"];
export type LoopSource = LoopCatalogEntry["source"];
export type LoopContract = LoopCatalogEntry["contract"];
export type LoopContractBudget = LoopContract["budget"];
export type LoopContractVerification = NonNullable<LoopContract["verification"]>[number];
export type LoopInputSchema = NonNullable<LoopCatalogEntry["inputs"]>;
export type LoopInputSchemaField = LoopInputSchema[string];
export type LoopStartBinding = NonNullable<LoopCatalogEntry["start"]>[number];
export type LoopAggregate30d = LoopCatalogEntry["aggregate_30d"];

export type LoopDetail = OperationResponse<"getLoop", 200>["loop"];
export type LoopDefinition = LoopDetail["definition"];
export type LoopDefinitionMeta = LoopDefinition["meta"];
export type LoopDefinitionGraph = LoopDefinition["graph"];

// Runs ----------------------------------------------------------------------

export type LoopRunListResult = OperationResponse<"listLoopRuns", 200>;
export type LoopRun = LoopRunListResult["runs"][number];
export type LoopRunAggregates = LoopRunListResult["aggregates"];
export type LoopRunDetail = OperationResponse<"getLoopRun", 200>;
export type LoopRunRecord = LoopRunDetail["run"];
export type LoopRunGeneration = NonNullable<LoopRunDetail["generations"]>[number];
export type LoopRunGenerationOutput = LoopRunGeneration["outputs"][number];

// Parked watch-events node read-model (present only while a loop is dormant on events).
export type LoopWatchEventsState = NonNullable<LoopRunDetail["watch_events"]>;
export type LoopWatchEventSubscription = LoopWatchEventsState["subscriptions"][number];

// SSE -----------------------------------------------------------------------

export type LoopRunEventFrame = OperationResponse<"streamLoopRunEvents", 200>;
export type LoopStreamFilter = OperationQuery<"streamLoopRunEvents">;
export type LoopRunsFilter = OperationQuery<"listLoopRuns">;

export type LoopRunEventKind = LoopRunEventFrame["kind"];

// Config --------------------------------------------------------------------

export type LoopConfigResponse = OperationResponse<"getLoopConfig", 200>;
export type LoopConfig = NonNullable<LoopConfigResponse["config"]>;
export interface LoopConfigSnapshot {
  config: LoopConfig | null;
  effectiveConfig: LoopConfigResponse["effective_config"];
}
export type LoopConfigUpdateRequest = OperationRequestBody<"putLoopConfig">;

/**
 * The execution environment descriptor. The same shape is the loop-level default
 * (`config.environment`) and the per-node override (`params.environment`), which
 * is why the node inspector and the configure dialog share one control grammar.
 */
export type LoopEnvironmentSpec = NonNullable<LoopConfig["environment"]>;
export type LoopEnvironmentMode = LoopEnvironmentSpec["mode"];

// Editor annotations (node positions) ---------------------------------------

export type LoopAnnotation = OperationResponse<"getLoopAnnotations", 200>["annotations"][number];
export type LoopAnnotationsUpdateRequest = OperationRequestBody<"putLoopAnnotations">;

// Write requests ------------------------------------------------------------

export type CreateLoopRequest = OperationRequestBody<"createLoop">;
export type PatchLoopRequest = OperationRequestBody<"patchLoop">;
export type RunLoopRequest = OperationRequestBody<"runLoop">;
export type RunLoopResult = OperationResponse<"runLoop", 201>;
export type LoopDryRunPreview = NonNullable<RunLoopResult["dry_run"]>;
export type LoopDryRunNode = LoopDryRunPreview["nodes"][number];
export type LoopEffectiveConfig = LoopConfigResponse["effective_config"];
export type ValidateLoopRequest = OperationRequestBody<"validateLoop">;
export type ValidateLoopResult = OperationResponse<"validateLoop", 200>;
export type LoopValidationIssue = NonNullable<ValidateLoopResult["errors"]>[number];
export type ApproveLoopRunRequest = OperationRequestBody<"approveLoopRun">;

/**
 * Boundary controls keep their compact acknowledgement response.
 */
export type LoopRunActionResult = OperationResponse<"pauseLoopRun", 200>;

/** Run cancel/kill operations return the shared structured lifecycle projection. */
export type LoopRunMutationResult = OperationResponse<"cancelLoopRun", 200>;

// Node lifecycle ------------------------------------------------------------

export type LoopNodeMutationResult = OperationResponse<"pauseLoopNode", 200>;
/** Durable per-node control truth: pause/quarantine/attention/cancel + revision. */
export type LoopNodeControl = NonNullable<LoopRunDetail["node_controls"]>[number];
/** One durable wait cell (timer, event, approval, dependency). */
export type LoopNodeWait = NonNullable<LoopRunDetail["waits"]>[number];
/** Provenance attached to a pause/cancel/requeue: who asked, why, when. */
export type LoopControlProvenance = NonNullable<LoopNodeControl["pause_provenance"]>;

export type LoopNodeInventoryResponse = OperationResponse<"listLoopNodes", 200>;
export type LoopNodeInventoryItem = LoopNodeInventoryResponse["items"][number];
export type LoopNodeInventoryFilter = OperationQuery<"listLoopNodes">;
/** The closed workspace inventory vocabulary (`waiting|quarantined|attention|retrying`). */
export type LoopNodeInventoryState = LoopNodeInventoryItem["state"];

export type LoopNodePauseRequest = OperationRequestBody<"pauseLoopNode">;
export type LoopNodeResumeRequest = OperationRequestBody<"resumeLoopNode">;
export type LoopNodeMutationRequest = OperationRequestBody<"cancelLoopNode">;

export type LoopRequestListResult = OperationResponse<"listLoopRequests", 200>;
export type LoopRequestFilter = OperationQuery<"listLoopRequests">;

export type LoopRequestStableFilter = Omit<LoopRequestFilter, "cursor">;

export type LoopRequest = LoopRequestListResult["items"][number];
export type LoopRequestAggregates = LoopRequestListResult["aggregates"];
export type LoopRequestDetail = OperationResponse<"getLoopRequest", 200>;
export type LoopRespondRequest = OperationRequestBody<"respondLoopRequest">;
export type LoopRespondResult = OperationResponse<"respondLoopRequest", 200>;
export type LoopRespondProvenance = LoopRespondResult["provenance"];

export type LoopAmendRequest = OperationRequestBody<"amendLoopNode">;
export type LoopAmendResult = OperationResponse<"amendLoopNode", 200>;

export type LoopAmendment = NonNullable<LoopRunDetail["amendments"]>[number];

export type LoopValueSummary = NonNullable<LoopAmendment["original_summary"]>;

export type LoopDiffQuery = OperationQuery<"diffLoopRun">;
export type LoopDiff = OperationResponse<"diffLoopRun", 200>;
export type LoopDiffSide = LoopDiff["base"];
export type LoopDiffNode = LoopDiff["nodes"][number];
export type LoopDiffInput = LoopDiff["inputs"][number];

export type LoopDiffValue = NonNullable<LoopDiffNode["base"]>;

export type LoopRerunRequest = OperationRequestBody<"rerunLoopRun">;
export type LoopRerunResult = OperationResponse<"rerunLoopRun", 200>;

export type LoopForkRequest = OperationRequestBody<"forkLoopRun">;
export type LoopForkResult = OperationResponse<"forkLoopRun", 201>;

export type LoopForkLink = NonNullable<LoopRunRecord["forked_from"]>;

export type LoopRouteCause = LoopRunGeneration["route_causes"][number];

// Status vocabulary ---------------------------------------------------------

export type LoopRunStatus = LoopRun["status"];
export type GoalTurnPage = OperationResponse<"listGoalTurns", 200>;
export type GoalTurn = GoalTurnPage["turns"][number];
export type GoalTurnFilter = OperationQuery<"listGoalTurns">;
