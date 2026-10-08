import { useState } from "react";
import { useNavigate } from "@tanstack/react-router";
import { useSelector, useStore } from "@xstate/store-react";
import { toast } from "sonner";

import {
  automationEditorSeed,
  compareAutomationViews,
  toAutomationView,
  useAutomationEditor,
  useAutomationJobs,
  useAutomationTriggers,
  useDeleteAutomationJob,
  useDeleteAutomationTrigger,
  useTriggerAutomationJob,
  useUpdateAutomationJob,
  useUpdateAutomationTrigger,
  type AutomationJob,
  type AutomationTrigger,
  type AutomationView,
  type AutomationsRouteSearch,
} from "@/systems/automation";
import { useProfileReadScope } from "@/systems/profiles";

import { automationPendingLogic } from "./automation-pending-store";
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

/** Which list failed when exactly one of the two loads failed (Business Rule 17). */
export type AutomationPartialFailure = "schedule" | "event" | null;

export interface AutomationStartCounts {
  all: number | null;
  schedule: number | null;
  event: number | null;
}

function viewKey(view: Pick<AutomationView, "kind" | "id">): string {
  return `${view.kind}:${view.id}`;
}

function errorMessage(error: unknown, fallback: string): string {
  return error instanceof Error && error.message ? error.message : fallback;
}

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
  const pendingStore = useStore(automationPendingLogic);
  const pendingIds = useSelector(pendingStore, snapshot => snapshot.context.pendingIds);
  const [deleteTarget, setDeleteTarget] = useState<AutomationView | null>(null);

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

  const triggersQueryError = fetchTriggers ? triggersQuery.error : null;
  const unavailableMessage = automationUnavailableMessage(
    page.automationRuntime,
    jobsQuery.error,
    triggersQueryError
  );
  // Only a rendered kind can fail the listing; the other kind's failure reads "—" in its count.
  const jobsError = showJobs ? jobsQuery.error : null;
  const triggersError = showTriggers ? triggersQuery.error : null;
  const shownKinds = Number(showJobs) + Number(showTriggers);
  const failedKinds = Number(jobsError !== null) + Number(triggersError !== null);
  const loadError =
    unavailableMessage === null && shownKinds > 0 && failedKinds === shownKinds
      ? (jobsError ?? triggersError)
      : null;
  const partialFailure: AutomationPartialFailure =
    unavailableMessage !== null || loadError !== null
      ? null
      : jobsError
        ? "schedule"
        : triggersError
          ? "event"
          : null;

  const counts: AutomationStartCounts = {
    schedule: jobsQuery.error || !jobsQuery.data ? null : jobsQuery.total,
    event: !fetchTriggers
      ? 0
      : triggersQuery.error || !triggersQuery.data
        ? null
        : triggersQuery.total,
    all: null,
  };
  counts.all =
    counts.schedule === null || counts.event === null ? null : counts.schedule + counts.event;
  /** Window count = both totals for the current filters; unknown until both answer. */
  const total = counts.all;

  const isLoading =
    items.length === 0 &&
    ((showJobs && jobsQuery.isLoading) || (showTriggers && triggersQuery.isLoading));

  const firstRun =
    !isLoading &&
    items.length === 0 &&
    !page.hasActiveFilters &&
    partialFailure === null &&
    loadError === null;
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

  const updateJob = useUpdateAutomationJob();
  const updateTrigger = useUpdateAutomationTrigger();
  const runJob = useTriggerAutomationJob();
  const deleteJob = useDeleteAutomationJob();
  const deleteTrigger = useDeleteAutomationTrigger();

  const requestAction = (id: string, run: () => Promise<void>) =>
    pendingStore.trigger.actionRequested({ id, permitted: unavailableMessage === null, run });

  const toggleEnabled = (view: AutomationView, enabled: boolean) => {
    const verb = enabled ? "on" : "off";
    requestAction(`toggle:${viewKey(view)}`, async () => {
      try {
        const mutation = view.kind === "job" ? updateJob : updateTrigger;
        await mutation.mutateAsync({ data: { enabled }, id: view.id, profile: view.profileName });
        toast.success(`Turned ${verb} ${view.name}.`);
      } catch {
        toast.error(`Couldn't turn ${verb} ${view.name}. Try again.`);
      }
    });
  };

  const runNow = (view: AutomationView) => {
    if (!view.canRunNow) return;
    requestAction(`run:${viewKey(view)}`, async () => {
      try {
        const run = await runJob.mutateAsync({ id: view.id, profile: view.profileName });
        toast.success(`Queued run ${run.id}.`);
      } catch (error) {
        toast.error(errorMessage(error, `Couldn't start ${view.name}. Try again.`));
      }
    });
  };

  const edit = (view: AutomationView) => {
    const entity = findEntity(view);
    if (!entity || !view.canEdit) return;
    editor.openEdit(entity);
  };

  const confirmDelete = async () => {
    const view = deleteTarget;
    if (!view) return;
    const mutation = view.kind === "job" ? deleteJob : deleteTrigger;
    await mutation.mutateAsync({ id: view.id, profile: view.profileName });
    toast.success(`Deleted ${view.name}.`);
    setDeleteTarget(null);
  };

  const create = (start: "schedule" | "event" | "webhook" | null = page.start) =>
    editor.openCreate({ start: start ?? "schedule" });

  const loadMore = () => {
    if (showJobs && jobsQuery.hasNextPage) void jobsQuery.fetchNextPage();
    if (showTriggers && triggersQuery.hasNextPage) void triggersQuery.fetchNextPage();
  };

  return {
    ...page,
    canLoadMore: (showJobs && jobsQuery.hasNextPage) || (showTriggers && triggersQuery.hasNextPage),
    confirmDelete,
    copyLink: copyAutomationLink,
    counts,
    create,
    deletePending: deleteJob.isPending || deleteTrigger.isPending,
    deleteTarget,
    edit,
    editorDialogProps: withSeedClose(editor.editorDialogProps, closeSeed),
    firstRun,
    suggestionsWorkspaceId,
    enabledCount: loaded.filter(item => item.enabled).length,
    isFetchingMore: jobsQuery.isFetchingNextPage || triggersQuery.isFetchingNextPage,
    isLoading,
    isPaused: jobsQuery.isPaused || triggersQuery.isPaused,
    isRunPending: (view: AutomationView) => pendingIds.has(`run:${viewKey(view)}`),
    isTogglePending: (view: AutomationView) => pendingIds.has(`toggle:${viewKey(view)}`),
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
    runNow,
    setDeleteTarget,
    toggleEnabled,
    total,
    unavailableMessage,
  };
}
