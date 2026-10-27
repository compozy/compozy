import { toast } from "sonner";

import {
  automationSuggestionsSource,
  compareAutomationViews,
  toAutomationView,
  type AutomationJob,
  type AutomationSuggestion,
  type AutomationTrigger,
  type AutomationView,
  type AutomationsRouteSearch,
} from "@/systems/automation";
import { useProfileReadScope } from "@/systems/profiles";
import { workspaceBadgeIdentity } from "@/systems/workspace";

import { deriveAutomationListingState } from "./automation-listing-state";
import { useAutomationRowActions } from "./use-automation-row-actions";
import { useAutomationLists } from "./use-automation-lists";
import { useAutomationsEditor } from "./use-automations-editor";
import { automationUnavailableMessage, useAutomationPageBase } from "./use-automation-page-base";

export type { AutomationPartialFailure, AutomationStartCounts } from "./automation-listing-state";

/** Merged listing model: both lists, one sort, honest totals and counts. */
export function mergeAutomationViews(
  jobs: readonly AutomationJob[],
  triggers: readonly AutomationTrigger[],
  toView: (entity: AutomationJob | AutomationTrigger) => AutomationView
): AutomationView[] {
  return [...jobs.map(toView), ...triggers.map(toView)].sort(compareAutomationViews);
}

/** Soonest `nextRunAt` among enabled schedules; null when none is scheduled. */
export function soonestNextRun(views: readonly AutomationView[]): string | null {
  let soonest: { at: number; iso: string } | null = null;
  for (const view of views) {
    if (view.kind !== "job" || !view.enabled || !view.nextRunAt) continue;
    const at = Date.parse(view.nextRunAt);
    if (Number.isNaN(at)) continue;
    if (soonest === null || at < soonest.at) soonest = { at, iso: view.nextRunAt };
  }
  return soonest?.iso ?? null;
}

/**
 * Footer facts over the loaded rows. "N on" and the next run are only true for
 * the whole list once no list has another page.
 */
export function summarizeLoadedAutomations(
  loaded: readonly AutomationView[],
  hasMorePages: boolean
): { enabledCount: number | null; loadedCount: number; nextRunAt: string | null } {
  if (hasMorePages) return { enabledCount: null, loadedCount: loaded.length, nextRunAt: null };
  return {
    enabledCount: loaded.filter(item => item.enabled).length,
    loadedCount: loaded.length,
    nextRunAt: soonestNextRun(loaded),
  };
}

/** Copies a webhook automation's public link. */
function copyAutomationLink(view: AutomationView): void {
  if (!view.webhookPath) return;
  const url = `${window.location.origin}${view.webhookPath}`;
  void navigator.clipboard
    .writeText(url)
    .then(() => toast.success("Copied link."))
    .catch(() => toast.error("Couldn't copy the link."));
}

/** `/automations` view-model: two list queries (minus `start`), merged, sorted and counted. */
export function useAutomationsPage(search: AutomationsRouteSearch = {}) {
  const page = useAutomationPageBase(search);
  const profile = useProfileReadScope();

  const lists = useAutomationLists({
    enabled: page.listsEnabled,
    filters: page.listFilters,
    start: page.start,
    targetFilter: page.targetFilter,
  });
  const { jobsQuery, triggersQuery, showJobs, showTriggers, loadedTriggers } = lists;

  const workspaceNames = new Map(page.workspaces.map(option => [option.id, option.name]));
  // All projects: every row names its project (User for automations without one).
  const allProjects = page.dataScope?.kind === "all";
  const toView = (entity: AutomationJob | AutomationTrigger) =>
    toAutomationView(entity, {
      timeZone: page.timeZone,
      workspaceName: id => workspaceNames.get(id),
      ...(profile.aggregate ? { ownerOf: profile.ownerOf } : {}),
      ...(allProjects
        ? { projectOf: item => workspaceBadgeIdentity(item.workspace_id, workspaceNames) }
        : {}),
    });
  // Footer truth ("M on · next run in X") spans both loaded lists, whatever the view.
  const loaded = mergeAutomationViews(jobsQuery.jobs, loadedTriggers, toView);
  const items = loaded.filter(view => (view.kind === "job" ? showJobs : showTriggers));
  const jobs = showJobs ? jobsQuery.jobs : [];
  const triggers = showTriggers ? loadedTriggers : [];

  const unavailableMessage = automationUnavailableMessage(
    page.automationRuntime,
    jobsQuery.error,
    lists.triggersError
  );
  const { loadError, partialFailure, counts, isLoading, firstRun } = deriveAutomationListingState({
    jobs: {
      shown: showJobs,
      fetched: true,
      error: jobsQuery.error,
      loaded: Boolean(jobsQuery.data),
      // No answer yet, however it is pending: still fetching, or idle until the scope settles. A
      // placeholder keeps the previous search's rows on screen, but with none it is no answer.
      loading: jobsQuery.isPending || jobsQuery.isPlaceholderData,
      total: jobsQuery.total,
    },
    triggers: {
      shown: showTriggers,
      fetched: lists.fetchTriggers,
      error: triggersQuery.error,
      loaded: Boolean(triggersQuery.data),
      loading: triggersQuery.isPending || triggersQuery.isPlaceholderData,
      total: triggersQuery.total,
    },
    unavailableMessage,
    itemCount: items.length,
    hasActiveFilters: page.hasActiveFilters,
  });
  /** Window count = both totals for the current filters; unknown until both answer. */
  const total = counts.all;

  // Suggestions are per project: never under User or the all-profiles aggregate.
  const suggestionsSource = automationSuggestionsSource(
    page.dataScope,
    firstRun && !profile.aggregate
  );

  const findEntity = (view: AutomationView): AutomationJob | AutomationTrigger | undefined =>
    view.kind === "job"
      ? jobs.find(job => job.id === view.id)
      : triggers.find(trigger => trigger.id === view.id);

  const editor = useAutomationsEditor({
    search,
    activeWorkspaceId: page.activeWorkspaceId,
    workspaces: page.workspaces,
    workspaceResolved: page.workspaceResolved,
    startView: page.start,
  });

  const actions = useAutomationRowActions({
    unavailable: unavailableMessage !== null,
    findEntity,
    openEdit: editor.openEdit,
  });

  const summary = summarizeLoadedAutomations(loaded, lists.hasMorePages);

  return {
    ...page,
    ...actions,
    ...summary,
    canLoadMore: lists.canLoadMore,
    copyLink: copyAutomationLink,
    counts,
    create: editor.create,
    editorDialogProps: editor.editorDialogProps,
    firstRun,
    suggestionsSource,
    /** All projects: the project each suggestion belongs to. */
    suggestionProject: allProjects
      ? (suggestion: AutomationSuggestion) =>
          workspaceBadgeIdentity(suggestion.workspace_id, workspaceNames)
      : undefined,
    isFetchingMore: lists.isFetchingMore,
    isLoading,
    isPaused: lists.isPaused,
    items,
    loadError,
    loadMore: lists.loadMore,
    partialFailure,
    profileScope: profile,
    retry: lists.retry,
    total,
    unavailableMessage,
  };
}
