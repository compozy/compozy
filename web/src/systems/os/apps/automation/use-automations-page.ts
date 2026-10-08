import { useNavigate } from "@tanstack/react-router";
import { toast } from "sonner";

import {
  automationEditorSeed,
  compareAutomationViews,
  toAutomationView,
  useAutomationEditor,
  useAutomationJobs,
  useAutomationTriggers,
  type AutomationJob,
  type AutomationTrigger,
  type AutomationView,
  type AutomationsRouteSearch,
} from "@/systems/automation";
import { useProfileReadScope } from "@/systems/profiles";

import { deriveAutomationListingState } from "./automation-listing-state";
import { useAutomationRowActions } from "./use-automation-row-actions";
import {
  automationUnavailableMessage,
  useAutomationCreateSeed,
  useAutomationCreateSeedStore,
  useAutomationPageBase,
} from "./use-automation-page-base";

/** A Cancel or dismiss of the editor is the operator's close: it also clears a `?create=` link. */
function withSeedClose<T extends { editor: { onCancel: () => void } | null }>(
  props: T,
  closeSeed: () => void
): T {
  const { editor } = props;
  if (!editor) return props;
  return {
    ...props,
    editor: {
      ...editor,
      onCancel: () => {
        closeSeed();
        editor.onCancel();
      },
    },
  };
}

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
  const navigate = useNavigate();
  const profile = useProfileReadScope();

  // Both lists load in every Start view so the view counts and the window total stay
  // honest on a cold load; a Start view only decides which kind renders. Task targets
  // exist only on jobs, so `target=task` never asks for triggers.
  const fetchTriggers = page.targetFilter !== "task";
  const showJobs = page.start !== "event";
  const showTriggers = page.start !== "schedule" && fetchTriggers;
  const jobsQuery = useAutomationJobs(page.listFilters);
  const triggersQuery = useAutomationTriggers(page.listFilters, { enabled: fetchTriggers });

  const workspaceNames = new Map(page.workspaces.map(option => [option.id, option.name]));
  const toView = (entity: AutomationJob | AutomationTrigger) =>
    toAutomationView(entity, {
      timeZone: page.timeZone,
      workspaceName: id => workspaceNames.get(id),
      ...(profile.aggregate ? { ownerOf: profile.ownerOf } : {}),
    });
  const loadedTriggers = fetchTriggers ? triggersQuery.triggers : [];
  // Footer truth ("M on · next run in X") spans both loaded lists, whatever the view.
  const loaded = mergeAutomationViews(jobsQuery.jobs, loadedTriggers, toView);
  const items = loaded.filter(view => (view.kind === "job" ? showJobs : showTriggers));
  const jobs = showJobs ? jobsQuery.jobs : [];
  const triggers = showTriggers ? loadedTriggers : [];

  const unavailableMessage = automationUnavailableMessage(
    page.automationRuntime,
    jobsQuery.error,
    fetchTriggers ? triggersQuery.error : null
  );
  const { loadError, partialFailure, counts, isLoading, firstRun } = deriveAutomationListingState({
    jobs: {
      shown: showJobs,
      fetched: true,
      error: jobsQuery.error,
      loaded: Boolean(jobsQuery.data),
      loading: jobsQuery.isLoading,
      total: jobsQuery.total,
    },
    triggers: {
      shown: showTriggers,
      fetched: fetchTriggers,
      error: triggersQuery.error,
      loaded: Boolean(triggersQuery.data),
      loading: triggersQuery.isLoading,
      total: triggersQuery.total,
    },
    unavailableMessage,
    itemCount: items.length,
    hasActiveFilters: page.hasActiveFilters,
  });
  /** Window count = both totals for the current filters; unknown until both answer. */
  const total = counts.all;

  // Suggestions are workspace-scoped: never in Global scope or the all-profiles aggregate.
  const suggestionsWorkspaceId =
    firstRun && search.scope !== "global" && !profile.aggregate && page.activeWorkspaceId
      ? page.activeWorkspaceId
      : null;

  const findEntity = (view: AutomationView): AutomationJob | AutomationTrigger | undefined =>
    view.kind === "job"
      ? jobs.find(job => job.id === view.id)
      : triggers.find(trigger => trigger.id === view.id);

  const seedStore = useAutomationCreateSeedStore();
  const editor = useAutomationEditor({
    activeWorkspaceId: page.activeWorkspaceId,
    workspaces: page.workspaces,
    onSaved: saved => {
      seedStore.trigger.saved();
      void (saved.entity === "job"
        ? navigate({ to: "/automations/jobs/$jobId", params: { jobId: saved.automation.id } })
        : navigate({
            to: "/automations/triggers/$triggerId",
            params: { triggerId: saved.automation.id },
          }));
    },
  });
  const closeSeed = useAutomationCreateSeed(
    seedStore,
    automationEditorSeed(search),
    {
      activeWorkspaceId: page.activeWorkspaceId,
      editorOpen: editor.editor !== null,
      resolved: page.workspaceResolved,
    },
    seed => editor.openCreate({ loop: seed.loop, start: seed.start })
  );

  const actions = useAutomationRowActions({
    unavailable: unavailableMessage !== null,
    findEntity,
    openEdit: editor.openEdit,
  });

  /** "New automation": the given start, else the current Start view, else a schedule. */
  const create = (start?: "schedule" | "event" | "webhook" | null) =>
    editor.openCreate({ start: (start === undefined ? page.start : start) ?? "schedule" });

  const loadMore = () => {
    if (showJobs && jobsQuery.hasNextPage) void jobsQuery.fetchNextPage();
    if (showTriggers && triggersQuery.hasNextPage) void triggersQuery.fetchNextPage();
  };

  return {
    ...page,
    ...actions,
    canLoadMore: (showJobs && jobsQuery.hasNextPage) || (showTriggers && triggersQuery.hasNextPage),
    copyLink: copyAutomationLink,
    counts,
    create,
    editorDialogProps: withSeedClose(editor.editorDialogProps, closeSeed),
    firstRun,
    suggestionsWorkspaceId,
    enabledCount: loaded.filter(item => item.enabled).length,
    isFetchingMore: jobsQuery.isFetchingNextPage || triggersQuery.isFetchingNextPage,
    isLoading,
    isPaused: jobsQuery.isPaused || triggersQuery.isPaused,
    items,
    loadError,
    loadMore,
    nextRunAt: soonestNextRun(loaded),
    partialFailure,
    profileScope: profile,
    retry: () => {
      void jobsQuery.refetch();
      if (fetchTriggers) void triggersQuery.refetch();
    },
    total,
    unavailableMessage,
  };
}
