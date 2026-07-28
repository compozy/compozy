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
  LoopsApiError,
  approveLoopRun,
  buildLoopStreamUrl,
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
  stopLoopRun,
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
  countByKind,
  groupLoopCatalog,
  hasActiveLoopFilters,
  hasHumanGate,
  isUnboundedCap,
  iterationCapLabel,
  loopCategories,
  loopCategory,
  loopInputCount,
  loopKind,
  loopStatuses,
  matchesLoopFilter,
  successRateLabel,
} from "./lib/loop-catalog";

// Listing filter bridge (reui Filters + URL parsers)
export type { LoopFilterFieldKey, LoopFilterHandlers } from "./lib/loop-list-filters";
export {
  applyLoopFilterChips,
  buildLoopFilterFields,
  loopFiltersToChips,
  parseLoopCategoryFilter,
  parseLoopKindFilter,
  parseLoopStatusFilter,
} from "./lib/loop-list-filters";

// Start-binding helpers
export type { LoopBindingKind, LoopBindingRow } from "./lib/loop-bindings";
export { bindingKindLabel, summarizeBindingKinds } from "./lib/loop-bindings";

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
export type {
  LoopRunStory,
  LoopRunStoryContext,
  LoopStoryIcon,
  LoopStoryIssue,
  LoopStoryNow,
  LoopStoryRow,
  LoopStoryTaskLink,
} from "./lib/loop-run-story";
export { buildNextNote, buildRunStory } from "./lib/loop-run-story";
export type { LoopProgressSegmentState, LoopRunProgressModel } from "./lib/loop-run-progress";
export { buildRunProgress, latestGateVerdict } from "./lib/loop-run-progress";
export type { LoopRunUsageRow, LoopUsageKey, LoopUsageTone } from "./lib/loop-run-usage";
export {
  buildRunUsage,
  deriveCostEstimate,
  formatClockDuration,
  runElapsedSeconds,
  usageNote,
  usageSnapshotFacts,
} from "./lib/loop-run-usage";
export type { LoopRunInputRow } from "./lib/loop-run-about";
export { buildInputRows, humanizeStartOrigin, watchedSubject } from "./lib/loop-run-about";
export type {
  LoopApprovalFact,
  LoopApprovalRequest,
  LoopCoordinatorFailure,
  LoopGateVerdict,
  LoopGoalTurnLive,
  LoopRunLiveState,
} from "./lib/loop-events";
export { applyLoopEventFrame, emptyLoopRunLiveState } from "./lib/loop-events";

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
  useCreateLoop,
  useDeleteLoop,
  usePatchLoop,
  usePauseLoopRun,
  usePutLoopAnnotations,
  usePutLoopConfig,
  useResumeLoopRun,
  useRunLoop,
  useStopLoopRun,
  useValidateLoop,
} from "./hooks/use-loop-actions";

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
export { MonoTag } from "./components/mono-tag";
export { LoopDetailView } from "./components/detail/loop-detail";
export { LoopStartBindingsPanel } from "./components/detail/loop-start-bindings-panel";
export { LoopRunsView } from "./components/runs/loop-runs-view";
export type { LoopOutcomeValue } from "./components/runs/loop-runs-outcome-filter";
export { LoopTargetFields } from "./components/target/loop-target-fields";

// Run form
export { LoopRunForm } from "./components/run-form/loop-run-form";
export { LoopRunInputField } from "./components/run-form/loop-run-input-field";
export { LoopRunOverrides } from "./components/run-form/loop-run-overrides";
export { LoopRunPreview } from "./components/run-form/loop-run-preview";

// Configure sheet
export { LoopConfigureDialog } from "./components/configure/loop-configure-dialog";

// Run page
export { GoalTurnTimeline } from "./components/run-page/goal-turn-timeline";
export { LoopRunAboutRail } from "./components/run-page/loop-run-about-rail";
export { LoopRunControls } from "./components/run-page/loop-run-controls";
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
