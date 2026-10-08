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
  useAutomationPageBase,
} from "./use-automation-page-base";

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

  const loadJobs = page.start !== "event";
  // Task targets exist only on jobs, so `target=task` never asks for triggers.
  const loadTriggers = page.start !== "schedule" && page.targetFilter !== "task";
  const jobsQuery = useAutomationJobs(page.listFilters, { enabled: loadJobs });
  const triggersQuery = useAutomationTriggers(page.listFilters, { enabled: loadTriggers });

  const workspaceNames = new Map(page.workspaces.map(option => [option.id, option.name]));
  const toView = (entity: AutomationJob | AutomationTrigger) =>
    toAutomationView(entity, {
      workspaceName: id => workspaceNames.get(id),
      ...(profile.aggregate ? { ownerOf: profile.ownerOf } : {}),
    });
  const jobs = loadJobs ? jobsQuery.jobs : [];
  const triggers = loadTriggers ? triggersQuery.triggers : [];
  const items = mergeAutomationViews(jobs, triggers, toView);

  const jobsError = loadJobs ? jobsQuery.error : null;
  const triggersError = loadTriggers ? triggersQuery.error : null;
  const unavailableMessage = automationUnavailableMessage(
    page.automationRuntime,
    jobsError,
    triggersError
  );
  const bothFailed = jobsError !== null && triggersError !== null;
  const onlyKindRequested = !loadJobs || !loadTriggers;
  const loadError =
    unavailableMessage === null &&
    (bothFailed || (onlyKindRequested && (jobsError ?? triggersError) !== null))
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

  const jobsTotal = jobsQuery.data ? jobsQuery.total : null;
  const triggersTotal = triggersQuery.data ? triggersQuery.total : null;
  const counts: AutomationStartCounts = {
    schedule: jobsQuery.error ? null : jobsTotal,
    event: page.targetFilter === "task" ? 0 : triggersQuery.error ? null : triggersTotal,
    all: null,
  };
  counts.all =
    counts.schedule === null && counts.event === null
      ? null
      : (counts.schedule ?? 0) + (counts.event ?? 0);
  const total = (loadJobs ? (jobsTotal ?? 0) : 0) + (loadTriggers ? (triggersTotal ?? 0) : 0);

  const isLoading =
    items.length === 0 &&
    ((loadJobs && jobsQuery.isLoading) || (loadTriggers && triggersQuery.isLoading));

  const findEntity = (view: AutomationView): AutomationJob | AutomationTrigger | undefined =>
    view.kind === "job"
      ? jobs.find(job => job.id === view.id)
      : triggers.find(trigger => trigger.id === view.id);

  const editor = useAutomationEditor({
    activeWorkspaceId: page.activeWorkspaceId,
    workspaces: page.workspaces,
    onSaved: saved =>
      void (saved.entity === "job"
        ? navigate({ to: "/automations/jobs/$jobId", params: { jobId: saved.automation.id } })
        : navigate({
            to: "/automations/triggers/$triggerId",
            params: { triggerId: saved.automation.id },
          })),
  });
  useAutomationCreateSeed(automationEditorSeed(search), page.activeWorkspaceId, seed =>
    editor.openCreate({ loop: seed.loop, start: seed.start })
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
    if (loadJobs && jobsQuery.hasNextPage) void jobsQuery.fetchNextPage();
    if (loadTriggers && triggersQuery.hasNextPage) void triggersQuery.fetchNextPage();
  };

  return {
    ...page,
    canLoadMore: (loadJobs && jobsQuery.hasNextPage) || (loadTriggers && triggersQuery.hasNextPage),
    confirmDelete,
    copyLink: copyAutomationLink,
    counts,
    create,
    deletePending: deleteJob.isPending || deleteTrigger.isPending,
    deleteTarget,
    edit,
    editorDialogProps: editor.editorDialogProps,
    enabledCount: items.filter(item => item.enabled).length,
    isFetchingMore: jobsQuery.isFetchingNextPage || triggersQuery.isFetchingNextPage,
    isLoading,
    isPaused: jobsQuery.isPaused || triggersQuery.isPaused,
    isRunPending: (view: AutomationView) => pendingIds.has(`run:${viewKey(view)}`),
    isTogglePending: (view: AutomationView) => pendingIds.has(`toggle:${viewKey(view)}`),
    items,
    loadError,
    loadMore,
    nextRunAt: soonestNextRun(items),
    partialFailure,
    profileScope: profile,
    retry: () => {
      if (loadJobs) void jobsQuery.refetch();
      if (loadTriggers) void triggersQuery.refetch();
    },
    runNow,
    setDeleteTarget,
    toggleEnabled,
    total,
    unavailableMessage,
  };
}
