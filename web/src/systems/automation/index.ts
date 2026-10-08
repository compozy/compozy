// Types
export type {
  AutomationFireLimit,
  AutomationJob,
  AutomationJobListFilter,
  AutomationJobsListResponse,
  AutomationJobStableFilter,
  AutomationKind,
  AutomationRetry,
  AutomationRun,
  AutomationRunHistoryFilter,
  AutomationRunListFilter,
  AutomationRunStatus,
  AutomationSuggestion,
  AutomationSuggestionAcceptanceResponse,
  AutomationSuggestionDismissalResponse,
  AutomationSuggestionsListResponse,
  AutomationSuggestionStatus,
  AutomationSchedule,
  AutomationScheduleMode,
  AutomationSchedulerState,
  AutomationScope,
  AutomationScopeFilter,
  AutomationSource,
  AutomationTrigger,
  AutomationTriggerFilter,
  AutomationTriggerListFilter,
  AutomationTriggersListResponse,
  AutomationTriggerStableFilter,
  CreateAutomationJobRequest,
  CreateAutomationTriggerRequest,
  UpdateAutomationJobRequest,
  UpdateAutomationTriggerRequest,
} from "./types";

// Adapters
export {
  AutomationApiError,
  createAutomationJob,
  createAutomationTrigger,
  deleteAutomationJob,
  deleteAutomationTrigger,
  getAutomationJob,
  getAutomationTrigger,
  listAutomationJobRuns,
  listAutomationJobs,
  listAutomationRuns,
  listAutomationTriggerRuns,
  listAutomationTriggers,
  triggerAutomationJob,
  updateAutomationJob,
  updateAutomationTrigger,
} from "./adapters/automation-api";
export {
  acceptAutomationSuggestion,
  dismissAutomationSuggestion,
  listAutomationSuggestions,
} from "./adapters/automation-suggestions-api";

// Query infrastructure
export { automationKeys } from "./lib/query-keys";
export {
  buildAutomationJobRequest,
  buildAutomationTriggerRequest,
} from "./lib/automation-requests";
export {
  automationJobDetailOptions,
  automationJobRunsOptions,
  automationJobsListOptions,
  automationRunsListOptions,
  automationSuggestionsListOptions,
  automationTriggerDetailOptions,
  automationTriggerRunsOptions,
  automationTriggersListOptions,
} from "./lib/query-options";
export type { AutomationLoopTarget, AutomationTargetMode } from "./lib/automation-drafts";
export {
  LOOP_TARGET_KIND,
  automationJobToDraft,
  automationJobUpdateFromDraft,
  automationTargetMode,
  automationTriggerToDraft,
  automationTriggerUpdateFromDraft,
  createAutomationJobDraft,
  createAutomationTriggerDraft,
  createLoopTargetJobDraft,
  createLoopTargetTriggerDraft,
  emptyLoopTarget,
  loopTargetWorkspaceId,
  normalizeAutomationRetry,
  retryDraftForStrategy,
  setJobTargetMode,
  setTriggerTargetMode,
} from "./lib/automation-drafts";
export type { AutomationDialogHandle } from "./lib/dialog-handle";
export { createAutomationDialogHandle } from "./lib/dialog-handle";
// Shared target projection for agent- and loop-backed automation surfaces.
export { projectAutomationTarget, type AutomationTargetProjection } from "./lib/automation-target";
export {
  automationSourceLabel,
  automationScopeLabel,
  automationScopeTone,
  automationSourceTone,
  automationLastRunLabel,
  automationLastRunMeta,
  automationRunStateGlyph,
  describeFireLimit,
  describeRetry,
  describeTrigger,
  formatDate,
  formatDateTime,
  formatPromptPreview,
  formatRelativeTime,
  formatRunDuration,
  formatRunTitle,
} from "./lib/automation-formatters";
export {
  applyAutomationFilterChips,
  automationFiltersToChips,
  buildAutomationFilterFields,
} from "./lib/automation-list-filters";
export {
  automationListLoopFilter,
  automationRouteHasActiveFilters,
  parseAutomationEnabled,
  parseAutomationScope,
  parseAutomationSource,
  automationEditorSeed,
  automationsStartView,
  automationDetailSearchFrom,
  automationListingSearch,
  parseAutomationTarget,
  validateAutomationDetailSearch,
  type AutomationEditorSeed,
  validateAutomationsSearch,
  type AutomationDetailRouteSearch,
  type AutomationsRouteSearch,
} from "./lib/automation-route-search";
export { redirectLegacyAutomationURL } from "./lib/automation-redirects";
export { AUTOMATION_START_ICON } from "./lib/automation-start-icon";
export type { AutomationEditorSection } from "./lib/automation-form-draft";
export {
  automationLocationLabel,
  automationTimeStat,
  compareAutomationViews,
  toAutomationView,
  type AutomationDoes,
  type AutomationEntityKind,
  type AutomationLastRun,
  type AutomationStart,
  type AutomationView,
  type AutomationViewContext,
} from "./lib/automation-view";
export {
  automationSentenceIsIncomplete,
  automationSentenceText,
  describeAutomation,
  describeSchedule,
  type AutomationDraft,
  type AutomationSentence,
  type AutomationSentenceSegment,
  type SentenceContext,
} from "./lib/automation-sentence";
export { automationLastRanAt, type AutomationEntity } from "./lib/automation-detail";
export {
  automationEditorWorkspaceId,
  automationMatchesActiveWorkspace,
  automationWorkspaceAccessError,
} from "./lib/workspace-access";
export type {
  AutomationFilterHandlers,
  AutomationFilterState,
} from "./lib/automation-list-filters";
// Hooks
export {
  useAutomationJob,
  useAutomationJobs,
  useAutomationJobRuns,
  useAutomationRuns,
  useAutomationTrigger,
  useAutomationTriggers,
  useAutomationTriggerRuns,
} from "./hooks/use-automation";
export {
  useCreateAutomationJob,
  useCreateAutomationTrigger,
  useDeleteAutomationJob,
  useDeleteAutomationTrigger,
  useTriggerAutomationJob,
  useUpdateAutomationJob,
  useUpdateAutomationTrigger,
} from "./hooks/use-automation-actions";
export {
  useAutomationEditor,
  type AutomationEditorCreateOptions,
  type AutomationSaveResult,
} from "./hooks/use-automation-editor";
export {
  useAcceptAutomationSuggestion,
  useDismissAutomationSuggestion,
} from "./hooks/use-automation-suggestion-actions";
export { useAutomationSuggestions } from "./hooks/use-automation-suggestions";

// Components
export {
  AutomationDetailPanel,
  type AutomationDetailPanelProps,
  type AutomationDetailStatus,
} from "./components/automation-detail/automation-detail-panel";
export { AutomationEditorDialog } from "./components/automation-editor-dialog";
export { AutomationListFilters } from "./components/automation-list-filters";
export { AutomationCatalogShell } from "./components/automation-catalog-shell";
export { AutomationRow, type AutomationItemControls } from "./components/automation-row";
export { AutomationCard } from "./components/automation-card";
export { AutomationStartViews } from "./components/automation-start-views";
export {
  AutomationSuggestionsCard,
  type AutomationSuggestionsCardProps,
  type AutomationSuggestionPendingAction,
} from "./components/automation-suggestions-card";
export { AutomationSuggestionsPanel } from "./components/automation-suggestions-panel";
