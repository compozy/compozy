// Types
export type {
  ApproveLoopRunRequest,
  CreateLoopRequest,
  LoopAggregate30d,
  LoopAnnotation,
  LoopAnnotationsUpdateRequest,
  LoopCatalogEntry,
  LoopCatalogStableFilter,
  LoopsListResponse,
  LoopCatalogInfo,
  LoopConfig,
  LoopConfigUpdateRequest,
  LoopContract,
  LoopContractBudget,
  LoopContractVerification,
  LoopDefinition,
  LoopDefinitionGraph,
  LoopDefinitionMeta,
  LoopDetail,
  LoopDryRunNode,
  LoopDryRunPreview,
  LoopEffectiveConfig,
  LoopInputSchema,
  LoopInputSchemaField,
  LoopNodeInventoryItem,
  LoopNodeInventoryState,
  LoopRun,
  LoopRunActionResult,
  LoopRunAggregates,
  LoopRunDetail,
  LoopRunEventFrame,
  LoopRunEventKind,
  LoopRunGeneration,
  LoopRunGenerationOutput,
  LoopRunListResult,
  LoopRunRecord,
  LoopRunStatus,
  LoopRunsFilter,
  LoopStartBinding,
  LoopStreamFilter,
  LoopValidationIssue,
  GoalTurn,
  GoalTurnFilter,
  GoalTurnPage,
  LoopWatchEventSubscription,
  LoopWatchEventsState,
  PatchLoopRequest,
  RunLoopRequest,
  RunLoopResult,
  ValidateLoopRequest,
  ValidateLoopResult,
} from "./types";

// Adapters
export {
  LoopLifecycleConflictError,
  LoopsApiError,
  approveLoopRun,
  buildLoopStreamUrl,
  cancelLoopRun,
  createLoop,
  deleteLoop,
  getLoop,
  getLoopAnnotations,
  getLoopConfig,
  getLoopRun,
  listLoopRuns,
  listGoalTurns,
  listLoops,
  patchLoop,
  pauseLoopRun,
  putLoopAnnotations,
  putLoopConfig,
  resumeLoopRun,
  runLoop,
  validateLoop,
} from "./adapters/loops-api";

// Query infrastructure
export type { GoalControlAction } from "./lib/goal-control-action";
export { loopsKeys } from "./lib/query-keys";
export {
  loopAnnotationsOptions,
  loopConfigOptions,
  loopDetailOptions,
  loopRunDetailOptions,
  loopNodeInventoryOptions,
  loopRunsOptions,
  loopsCatalogOptions,
} from "./lib/query-options";

// Catalog helpers
export type {
  LoopCatalogFilter,
  LoopCatalogGroup,
  LoopKind,
  LoopKindFilter,
  LoopStatusFilter,
} from "./lib/loop-catalog";
export {
  UNBOUNDED_CAP,
  groupLoopCatalog,
  hasActiveLoopFilters,
  hasHumanGate,
  isUnboundedCap,
  iterationCapLabel,
  loopCategory,
  loopInputCount,
  loopKind,
  matchesLoopFilter,
  successRateLabel,
} from "./lib/loop-catalog";
export { loopFactsSegments } from "./lib/loop-catalog-presentation";

// Listing filter bridge (status options + URL parsers)
export type { LoopStatusFilterOption } from "./lib/loop-list-filters";
export {
  loopStatusFilterOptions,
  parseLoopCategoryFilter,
  parseLoopKindFilter,
  parseLoopStatusFilter,
} from "./lib/loop-list-filters";

// Start-binding helpers
export type { LoopBindingKind, LoopBindingRow } from "./lib/loop-bindings";
export { bindingKindLabel, summarizeBindingKinds } from "./lib/loop-bindings";
export type { LoopStartKinds } from "./lib/loop-start-kinds";
export { describeStartKinds, RUN_FORM_START_KIND } from "./lib/loop-start-kinds";

// Read-only graph projection
export type { LoopGraph, LoopGraphEdge, LoopGraphNode, LoopNodeClass } from "./lib/loop-graph";
export {
  fanOutSummary,
  findWatchNode,
  goalNodeIds,
  nodeClassLabel,
  readLoopGraph,
} from "./lib/loop-graph";

// Visual editor — bijective codec, layout, linter, node schema, references (task 22)
export type { EditorEdge, EditorGraph, EditorNode, RawLoopEdge, RawLoopNode } from "./lib/codec";
export { definitionToGraph, editorEdgeId, graphToDefinition } from "./lib/codec";
export { layoutEditorGraph, annotationsToPositions } from "./lib/loop-editor-layout";
export type { LoopInvariantKey, LoopInvariantStatus, LoopLintState } from "./lib/loop-editor-lint";
export {
  LOOP_INVARIANTS,
  applyLintToNodes,
  buildLintState,
  classifyInvariant,
  emptyLintState,
} from "./lib/loop-editor-lint";
export type { FieldPath, FieldSpec } from "./lib/loop-node-schema";
export { buildNodeFields } from "./lib/loop-node-schema";
export type { LoopReferenceKind, LoopReferenceSuggestion } from "./lib/loop-references";
export {
  activeReferenceQuery,
  buildReferenceNamespace,
  filterReferences,
} from "./lib/loop-references";
export type { DslLine } from "./lib/loop-dsl";
export { buildDslView } from "./lib/loop-dsl";
export type { PaletteGroup, PaletteItem } from "./lib/loop-palette";
export { LOOP_PALETTE, uniqueNodeId } from "./lib/loop-palette";
export {
  getAtPath,
  isNodeIdPath,
  renameNodeId,
  setAtPath,
  setNodeField,
} from "./lib/loop-editor-draft";
export { useLoopEditor } from "./hooks/use-loop-editor";
export type {
  LoopEditorStatus,
  LoopEditorView,
  UseLoopEditorResult,
} from "./hooks/use-loop-editor";
export { LoopEditor } from "./components/editor/loop-editor";

// Limits & budget
export type { LoopLimitRow } from "./lib/loop-limits";
export {
  LOOP_CEILINGS,
  buildLoopLimits,
  formatTokenBudget,
  formatWallClock,
} from "./lib/loop-limits";

// Runs view-model
export type {
  LoopBudgetBar,
  LoopBudgetTone,
  LoopKpi,
  LoopOutcomeSegment,
  LoopRunKpis,
  LoopRunPartition,
} from "./lib/loop-runs-view";
export {
  buildOutcomeSegments,
  buildRunKpis,
  formatTokenCount,
  loopBudgetBar,
  loopRunOriginLine,
  partitionRuns,
} from "./lib/loop-runs-view";

// Run-form model
export type { LoopRunInputs } from "./lib/loop-run-form";
export {
  hasInputValue,
  initialRunInputs,
  isRunFormValid,
  missingRequiredInputs,
  serializeRunInputs,
} from "./lib/loop-run-form";
export type {
  LoopBudgetPolicy,
  LoopOverrideDraft,
  LoopOverrideField,
  LoopOverrideKey,
} from "./lib/loop-overrides";
export {
  buildConfigOverrides,
  buildOverrideFields,
  clampOverrideValue,
  hasActiveOverrides,
  initialOverrideDraft,
  summarizeRunLimits,
} from "./lib/loop-overrides";

// Configure-sheet model
export type {
  EnabledChecksMap,
  LoopConfigCheckDescriptor,
  LoopConfigCheckState,
} from "./lib/loop-config-checks";
export {
  buildCheckDescriptors,
  defaultCheckStates,
  initialCheckStates,
  parseEnabledChecks,
  serializeEnabledChecks,
} from "./lib/loop-config-checks";
export type { LoopConfigDraft, LoopReattemptStrategy } from "./lib/loop-config-draft";
export {
  buildConfigureModel,
  buildLoopConfigRequest,
  initialConfigDraft,
  resetConfigDraft,
} from "./lib/loop-config-draft";

// Run-page model
export {
  buildNextNote,
  buildRunStory,
  type LoopRunStory,
  type LoopRunStoryContext,
  type LoopStoryIcon,
  type LoopStoryIssue,
  type LoopStoryNow,
  type LoopStoryRow,
  type LoopStoryTaskLink,
} from "./lib/loop-run-story";
export {
  buildRunProgress,
  latestGateVerdict,
  type LoopProgressSegmentState,
  type LoopRunProgressModel,
} from "./lib/loop-run-progress";
export {
  buildRunUsage,
  deriveCostEstimate,
  formatClockDuration,
  type LoopRunUsageRow,
  type LoopUsageKey,
  type LoopUsageTone,
  runElapsedSeconds,
  usageNote,
  usageSnapshotFacts,
} from "./lib/loop-run-usage";
export {
  buildInputRows,
  humanizeStartOrigin,
  type LoopRunInputRow,
  watchedSubject,
} from "./lib/loop-run-about";
export {
  applyLoopEventFrame,
  emptyLoopRunLiveState,
  type LoopApprovalFact,
  type LoopApprovalRequest,
  type LoopCoordinatorFailure,
  type LoopGateVerdict,
  type LoopGoalTurnLive,
  type LoopRunLiveState,
} from "./lib/loop-events";

// Run-page view projection (one derivation path shared by the page hook + fixtures)
export type { LoopRunPageView, LoopRunPageViewInput } from "./lib/loop-run-page-view";
export { projectLoopRunPageView } from "./lib/loop-run-page-view";
export { useNowTick } from "./hooks/use-now-tick";

// Formatters and helpers
export type { LoopStatusSignal } from "./lib/loop-formatters";
export {
  LOOP_STATUS_LABELS,
  LOOP_STATUS_TONE,
  isLoopRunStatus,
  isTerminalLoopStatus,
  loopStatusLabel,
  loopStatusPulse,
  loopStatusSignal,
  loopStatusTone,
} from "./lib/loop-formatters";

// Read hooks
export {
  useLoop,
  useLoopAnnotations,
  useLoopConfig,
  useLoopRun,
  useLoopRuns,
  useLoops,
} from "./hooks/use-loops";

// Mutation hooks
export {
  useApproveLoopRun,
  useCancelLoopRun,
  useCreateLoop,
  useDeleteLoop,
  usePatchLoop,
  usePauseLoopRun,
  usePutLoopAnnotations,
  usePutLoopConfig,
  useKillLoopRun,
  useResumeLoopRun,
  useRunLoop,
  useValidateLoop,
} from "./hooks/use-loop-actions";

// Node lifecycle (Spec 1). Only what crosses the system boundary lives here;
// files inside `systems/loops/` import their own modules directly.
export {
  useCancelLoopNode,
  useKillLoopNode,
  usePauseLoopNode,
  useRequeueLoopNode,
  useResumeLoopNode,
} from "./hooks/use-loop-node-actions";
export { useLoopNodeInventory } from "./hooks/use-loop-node-inventory";
export { type LoopNodeLifecycle, projectNodeLifecycles } from "./lib/loop-node-lifecycle";
export {
  loopControlAnswer,
  type LoopNodeVerb,
  loopNodeWaitResumeItemIndex,
  loopRunVerbs,
} from "./lib/loop-node-controls";
export { buildNodeNowLines } from "./lib/loop-node-now-view";
export {
  isLoopNodeInventoryState,
  LOOP_NODE_INVENTORY_LABELS,
  LOOP_NODE_INVENTORY_STATES,
  type LoopNodeInventorySort,
} from "./lib/loop-node-inventory";

// Run-form view-model hook
export { useLoopRunForm } from "./hooks/use-loop-run-form";

// Configure-sheet view-model hook
export { useLoopConfigure } from "./hooks/use-loop-configure";
export type { UseLoopConfigureResult } from "./hooks/use-loop-configure";

// SSE stream hook
export { useLoopStream } from "./hooks/use-loop-stream";
export { mergeGoalTurns, mergeGoalTurnTimeline, useGoalTurns } from "./hooks/use-goal-turns";
export type { GoalTurnTimelineItem, UseGoalTurnsOptions } from "./hooks/use-goal-turns";
export type {
  LoopStreamEventSource,
  LoopStreamEventSourceFactory,
  UseLoopStreamOptions,
} from "./hooks/use-loop-stream";

// Components
export { LoopStatusPill } from "./components/loop-status-pill";
export type { LoopStatusPillProps } from "./components/loop-status-pill";
export { LoopCatalog } from "./components/catalog/loop-catalog";
export { LoopCatalogCard } from "./components/catalog/loop-catalog-card";
export { LoopCatalogFilters } from "./components/catalog/loop-catalog-filters";
export { LoopCatalogLede } from "./components/catalog/loop-catalog-lede";
export { MonoTag } from "./components/mono-tag";
export { LoopDetailView } from "./components/detail/loop-detail";
export { LoopStartBindingsPanel } from "./components/detail/loop-start-bindings-panel";
export { LoopRunsView } from "./components/runs/loop-runs-view";
export { LoopNodeInventoryView } from "./components/runs/loop-node-inventory-view";
export type { LoopNodeInventoryViewProps } from "./components/runs/loop-node-inventory-view";
export type { LoopOutcomeValue } from "./components/runs/loop-runs-outcome-filter";
export { LoopTargetFields } from "./components/target/loop-target-fields";

// Run form
export { LoopRunForm } from "./components/run-form/loop-run-form";
export { LoopRunActiveNotice } from "./components/run-form/loop-run-active-notice";
export { LoopRunAfterStart } from "./components/run-form/loop-run-after-start";
export { LoopRunInputField } from "./components/run-form/loop-run-input-field";
export { LoopRunOverrides } from "./components/run-form/loop-run-overrides";
export { LoopRunPreview } from "./components/run-form/loop-run-preview";
export { LoopRunWaysToStart } from "./components/run-form/loop-run-ways-to-start";

// Configure sheet
export { LoopConfigureDialog } from "./components/configure/loop-configure-dialog";

// Run page
export { GoalTurnTimeline } from "./components/run-page/goal-turn-timeline";
export { LoopRunAboutRail } from "./components/run-page/loop-run-about-rail";
export { LoopRunControls } from "./components/run-page/loop-run-controls";
export { LoopNodeRowActions } from "./components/run-page/loop-node-row-actions";
export {
  LoopNodeControlDialog,
  type LoopNodeVerbCommit,
  type LoopNodeVerbRequest,
} from "./components/run-page/loop-node-control-dialog";
export { LoopQuarantineSheet } from "./components/run-page/loop-quarantine-sheet";
export {
  LoopRunControlDialog,
  type LoopRunConfirmVerb,
} from "./components/run-page/loop-run-control-dialog";
export { LoopRunInspectSheet } from "./components/run-page/loop-run-inspect-sheet";
export { LoopRunNeedsYouCard } from "./components/run-page/loop-run-needs-you-card";
export type { LoopGateDecision } from "./lib/loop-events";
export { LoopRunNextNote } from "./components/run-page/loop-run-next-note";
export { LoopRunNowCard } from "./components/run-page/loop-run-now-card";
export { LoopRunOutcomeCard } from "./components/run-page/loop-run-outcome-card";
export { LoopRunOverflowMenu } from "./components/run-page/loop-run-overflow-menu";
export { LoopRunPageBody } from "./components/run-page/loop-run-page-body";
export type { LoopRunPageBodyProps } from "./components/run-page/loop-run-page-body";
export { LoopRunProgressPanel } from "./components/run-page/loop-run-progress-panel";
export { LoopRunSection } from "./components/run-page/loop-run-section";
export { LoopRunStoryTimeline } from "./components/run-page/loop-run-story-timeline";
export { LoopRunSubhead } from "./components/run-page/loop-run-subhead";
export { LoopRunTurnsDisclosure } from "./components/run-page/loop-run-turns-disclosure";
export { LoopRunUsageRail } from "./components/run-page/loop-run-usage-rail";

// Loop-target editing (automation Target step)
export type { LoopTargetDraft } from "./lib/loop-target";
export { setLoopTargetInput, setLoopTargetLoop, setLoopTargetMapping } from "./lib/loop-target";
export type {
  LoopAutomationStartKind,
  LoopTargetAvailabilityStatus,
  LoopTargetCatalog,
} from "./lib/loop-target-availability";
export {
  loopDeclaresStartKind,
  loopTargetAvailabilityMessage,
  projectLoopTargetCatalog,
} from "./lib/loop-target-availability";
export { useLoopTargetCatalog } from "./hooks/use-loop-target-catalog";
