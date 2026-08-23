// Types
export type {
  CreateLoopRequest,
  LoopAnnotationsUpdateRequest,
  LoopCatalogEntry,
  LoopCatalogStableFilter,
  LoopConfigUpdateRequest,
  LoopDefinition,
  LoopDefinitionMeta,
  LoopDryRunNode,
  LoopInputSchema,
  LoopRun,
  LoopRunDetail,
  LoopRunEventFrame,
  LoopRunEventKind,
  LoopRunGeneration,
  LoopRunGenerationOutput,
  LoopRunRecord,
  LoopRunsFilter,
  LoopValidationIssue,
  PatchLoopRequest,
  RunLoopResult,
  LoopAmendment,
  LoopRequest,
  LoopRequestAggregates,
  LoopRequestFilter,
  LoopRequestListResult,
} from "./types";

export type { LoopRequestKind } from "./lib/loop-request-vocabulary";
export { LOOP_REQUEST_KIND_TITLE } from "./lib/loop-request-vocabulary";
export {
  loopRequestLocation,
  loopRequestLocationPath,
  type LoopRequestLocationTarget,
} from "./lib/loop-request-location";

// Adapters
export {
  LoopInputValidationError,
  LoopLifecycleConflictError,
  LoopReadError,
  LoopRequestError,
  LoopsApiError,
  LoopTimetravelError,
  amendLoopNode,
  approveLoopRun,
  buildLoopStreamUrl,
  cancelLoopRun,
  createLoop,
  deleteLoop,
  diffLoopRun,
  forkLoopRun,
  getLoop,
  getLoopAnnotations,
  getLoopConfig,
  getLoopRequest,
  getLoopRun,
  getLoopRunBriefing,
  getLoopRunRoster,
  getLoopRunTimeline,
  listLoopRequests,
  listLoopRuns,
  listLoops,
  patchLoop,
  pauseLoopRun,
  putLoopAnnotations,
  putLoopConfig,
  rerunLoopRun,
  respondLoopRequest,
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
  loopRequestAttentionOptions,
  loopRequestsOptions,
  loopRunDetailOptions,
  loopRunDiffOptions,
  loopRunsOptions,
  loopsCatalogOptions,
} from "./lib/query-options";

// Catalog helpers
export type { LoopCatalogFilter, LoopStatusFilter } from "./lib/loop-catalog";
export { loopFactsSegments } from "./lib/loop-catalog-presentation";

// Listing filter bridge (status options + URL parsers)
export type { LoopStatusFilterOption } from "./lib/loop-list-filters";
export { loopStatusFilterOptions } from "./lib/loop-list-filters";
export {
  loopRunDiffQuery,
  validateLoopRunDiffSearch,
  validateLoopRunsSearch,
  validateLoopsSearch,
  type LoopRunDiffRouteSearch,
  type LoopRunsRouteSearch,
  type LoopsRouteSearch,
} from "./lib/loops-route-search";

// Start-binding helpers
export type { LoopBindingKind, LoopBindingRow } from "./lib/loop-bindings";
export { bindingKindLabel, summarizeBindingKinds } from "./lib/loop-bindings";
export type { LoopStartKinds } from "./lib/loop-start-kinds";
export { describeStartKinds, RUN_FORM_START_KIND } from "./lib/loop-start-kinds";

// Read-only graph projection
export type { LoopGraph, LoopGraphEdge, LoopGraphNode, LoopNodeClass } from "./lib/loop-graph";
export { readLoopGraph } from "./lib/loop-graph";

export type { EditorEdge, EditorGraph, EditorNode, RawLoopEdge, RawLoopNode } from "./lib/codec";
export { definitionToGraph, editorEdgeId, graphToDefinition } from "./lib/codec";
export type { LoopInvariantKey, LoopInvariantStatus, LoopLintState } from "./lib/loop-editor-lint";
export type { FieldPath, FieldSpec } from "./lib/loop-node-schema";
export { buildNodeFields } from "./lib/loop-node-schema";
export type { LoopReferenceKind, LoopReferenceSuggestion } from "./lib/loop-references";
export type { DslLine } from "./lib/loop-dsl";
export { buildDslView } from "./lib/loop-dsl";
export type { PaletteGroup, PaletteItem } from "./lib/loop-palette";
export { LOOP_STORY_ICONS } from "./lib/loop-story-icons";
export type { LoopNodeKind } from "./lib/loop-node-kind-icons";
export type { LoopStartKind } from "./lib/loop-start-kind-icons";
export { LOOP_START_KIND_ICONS, loopStartKindIcon } from "./lib/loop-start-kind-icons";
export { useLoopEditor } from "./hooks/use-loop-editor";
export type { RouteEdgeDisplay } from "./lib/loop-editor-route-edges";

export type { UseLoopEditorChromeStateResult } from "./hooks/use-loop-editor-chrome-state";
export { useLoopEditorShortcuts } from "./hooks/use-loop-editor-shortcuts";
export type { LoopEditorShortcutHandlers } from "./hooks/use-loop-editor-shortcuts";
export { LoopEditor } from "./components/editor/loop-editor";
export { LoopEditorFold } from "./components/editor/loop-editor-fold";
export { LoopEditorCanvas } from "./components/editor/loop-editor-canvas";
export type {
  LoopEditorConnectionDrop,
  LoopEditorNodeActions,
  LoopEditorPaletteMode,
} from "./lib/loop-editor-types";
export { LoopEditorEdge } from "./components/editor/loop-editor-edge";
export type { LoopEditorEdgeData } from "./components/editor/loop-editor-edge";
export { LoopEditorNodeActionsProvider } from "./components/editor/loop-editor-node";
export { LoopEditorNodeMenu } from "./components/editor/loop-editor-node-menu";
export type { LoopEditorNodeMenuProps } from "./components/editor/loop-editor-node-menu";
export { LoopEditorPalette } from "./components/editor/loop-editor-palette";
export { LoopEditorPaletteMenu } from "./components/editor/loop-editor-palette-menu";
export { LoopEditorQuickAdd } from "./components/editor/loop-editor-quick-add";
export { LoopEditorConnectionPicker } from "./components/editor/loop-editor-connection-picker";
export type { LoopEditorConnectionPickerProps } from "./components/editor/loop-editor-connection-picker";
export { loopNodeCardRows } from "./lib/loop-node-card-rows";
export type { LoopNodeCardRow } from "./lib/loop-node-card-rows";

// Limits & budget
export type { LoopLimitRow } from "./lib/loop-limits";

// Run-form model
export type { LoopRunInputs } from "./lib/loop-run-form";

// Configure-sheet model
export type { LoopConfigDraft, LoopReattemptStrategy } from "./lib/loop-config-draft";

// Run-page model
export {
  type LoopStepRow,
  type LoopStepsProgressModel,
  buildStepsProgress,
} from "./lib/loop-run-progress";
export { type LoopProgressSegment } from "./lib/loop-run-fanout-band";
export {
  type LoopNodeSelection,
  type LoopRosterReach,
  type LoopRunRegisters as LoopRunRegistersModel,
  LOOP_ROSTER_CONTINUATION_COMMAND,
  loopRosterReachNote,
  projectLoopRunRegisters,
  selectedRosterNode,
} from "./lib/loop-run-registers-view";
export { type LoopStoryBeat, buildStoryBeats } from "./lib/loop-run-story-beats";
export { type LoopStreamSeam, loopStreamSeam } from "./lib/loop-run-live-seam";
export { type LoopStateChip, loopRosterStateChip } from "./lib/loop-run-state-copy";
export { type LoopRunUsageRow, type LoopUsageKey, type LoopUsageTone } from "./lib/loop-run-usage";
export { type LoopRunInputRow } from "./lib/loop-run-about";
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
export { isLiveLoopRun, isTerminalLoopStatus } from "./lib/loop-formatters";

// Read hooks
export {
  useLoop,
  useLoopAnnotations,
  useLoopConfig,
  useLoopRun,
  useLoopRuns,
  useLoops,
} from "./hooks/use-loops";
export { useLoopRequestDetail, useLoopRequests } from "./hooks/use-loop-requests";
export {
  useLoopRequestAttention,
  type LoopRequestAttention,
  type LoopRequestAttentionItem,
} from "./hooks/use-loop-request-attention";

export { useAmendLoopNode, useRespondLoopRequest } from "./hooks/use-loop-request-actions";
export {
  useForkLoopRun,
  useLoopRunDiff,
  useRerunLoopRun,
} from "./hooks/use-loop-timetravel-actions";

// Mutation hooks
export {
  useApproveLoopRun,
  useCancelLoopRun,
  useCreateLoop,
  useDeleteLoop,
  usePatchLoop,
  usePauseLoopRun,
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
export { useLoopNodeExists } from "./hooks/use-loop-node-exists";
export { type LoopNodeLifecycle, projectNodeLifecycles } from "./lib/loop-node-lifecycle";
export {
  loopControlAnswer,
  type LoopControlAnswer,
  type LoopNodeTimetravelCapability,
  type LoopNodeVerb,
  loopNodeVerbs,
  loopNodeWaitResumeItemIndex,
  loopRunVerbs,
} from "./lib/loop-node-controls";
export { buildRerunSet, type LoopRerunSet } from "./lib/loop-rerun-set";
export { LOOP_NODE_INVENTORY_LABELS, LOOP_NODE_INVENTORY_STATES } from "./lib/loop-node-inventory";

// Run-form view-model hook
export { useLoopRunForm } from "./hooks/use-loop-run-form";

// Configure-sheet view-model hook
export { useLoopConfigure } from "./hooks/use-loop-configure";
export type { UseLoopConfigureResult } from "./hooks/use-loop-configure";

// SSE stream hook
export { useLoopStream } from "./hooks/use-loop-stream";
export type { LoopStreamEventSource } from "./hooks/use-loop-stream";

// Components
export { LoopStatusPill } from "./components/loop-status-pill";
export type { LoopStatusPillProps } from "./components/loop-status-pill";
export { LoopSection } from "./components/loop-section";
export { LoopCatalog } from "./components/catalog/loop-catalog";
export { LoopCatalogCard } from "./components/catalog/loop-catalog-card";
export { LoopCatalogFilters } from "./components/catalog/loop-catalog-filters";
export { LoopCatalogLede } from "./components/catalog/loop-catalog-lede";
export { MonoTag } from "./components/mono-tag";
export { LoopDetailView } from "./components/detail/loop-detail";
export { LoopStartBindingsPanel } from "./components/detail/loop-start-bindings-panel";
export { LoopRunsView } from "./components/runs/loop-runs-view";
export { LoopRunsFilters } from "./components/runs/loop-runs-filters";
export { LoopNodeInventoryView } from "./components/runs/loop-node-inventory-view";
export type { LoopNodeInventoryViewProps } from "./components/runs/loop-node-inventory-view";
export type { LoopOutcomeValue } from "./lib/loop-runs-view";
export { LoopTargetFields } from "./components/target/loop-target-fields";

// Run form
export { LoopRunForm } from "./components/run-form/loop-run-form";
export { LoopRunActiveNotice } from "./components/run-form/loop-run-active-notice";
export { LoopRunInputField } from "./components/run-form/loop-run-input-field";
export { LoopRunOverrides } from "./components/run-form/loop-run-overrides";
export { LoopRunPlan } from "./components/run-form/loop-run-plan";

// Configure sheet
export { LoopConfigureDialog } from "./components/configure/loop-configure-dialog";

// Run page
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
export {
  type LoopRequestAnswerInput,
  type LoopRequestFocusTarget,
} from "./components/run-page/requests/loop-request-questionnaire";
export {
  LoopNodeAmendDialog,
  type LoopNodeAmendDialogProps,
} from "./components/run-page/loop-node-amend-dialog";
export {
  LoopNodeRerunDialog,
  type LoopNodeRerunDialogProps,
} from "./components/run-page/loop-node-rerun-dialog";
export { LoopForkDialog, type LoopForkDialogProps } from "./components/run-page/loop-fork-dialog";
export type { LoopGateDecision } from "./lib/loop-events";
export { LoopRunOverflowMenu } from "./components/run-page/loop-run-overflow-menu";
export { LoopRunPageBody } from "./components/run-page/loop-run-page-body";
export type { LoopRunPageBodyProps } from "./components/run-page/loop-run-page-body";
export { LoopRunUsageRail } from "./components/run-page/loop-run-usage-rail";
export { LoopRunBriefing } from "./components/run-page/loop-run-briefing";
export { LoopRunStepsProgress } from "./components/run-page/loop-run-steps-progress";
export { LoopRunStory } from "./components/run-page/loop-run-story";
export { LoopRunRegisters } from "./components/run-page/loop-run-registers";
export { LoopNodeStateChip } from "./components/run-page/loop-node-state-chip";
export {
  useLoopRunBriefing,
  useLoopRunRoster,
  useLoopRunTimeline,
} from "./hooks/use-loop-run-reads";
export {
  type LoopRunEventsReadState,
  useLoopRunEventsRead,
} from "./hooks/use-loop-run-events-read";
export {
  type LoopNodeSessionAvailability,
  loopPrunedSessionIds,
  useLoopNodeSessionAvailability,
} from "./hooks/use-loop-node-session-availability";

export type { LoopDiffView } from "./lib/loop-run-diff-model";
export { comparableGenerations, projectLoopDiff } from "./lib/loop-run-diff-model";
export {
  LoopRunDiffPickers,
  LoopRunDiffView,
  type LoopRunDiffInputsProps,
  type LoopRunDiffPickersProps,
  type LoopRunDiffRowProps,
  type LoopRunDiffViewProps,
} from "./components/run-diff";

// Loop-target editing (automation Target step)
export type { LoopTargetDraft } from "./lib/loop-target";
export { setLoopTargetInput, setLoopTargetLoop, setLoopTargetMapping } from "./lib/loop-target";
export type { LoopAutomationStartKind, LoopTargetCatalog } from "./lib/loop-target-availability";
export { loopTargetAvailabilityMessage } from "./lib/loop-target-availability";
export { useLoopTargetCatalog } from "./hooks/use-loop-target-catalog";
