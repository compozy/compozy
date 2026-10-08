import { useEffect, useState } from "react";
import { useNavigate } from "@tanstack/react-router";
import { useStore } from "@xstate/store-react";
import { toast } from "sonner";

import { automationEditIntentLogic } from "./automation-edit-intent-store";
import { automationUnavailableMessage } from "./use-automation-page-base";
import { useCurrentWindowLiveDataEnabled } from "../../hooks/use-window-live-data-enabled";
import {
  AutomationApiError,
  automationEditorWorkspaceId,
  automationLastRanAt,
  automationListingSearch,
  type AutomationDetailRouteSearch,
  type AutomationDetailStatus,
  type AutomationEditorSection,
  type AutomationEntityKind,
  type AutomationRun,
  automationWorkspaceAccessError,
  projectAutomationTarget,
  toAutomationView,
  useAutomationJob,
  useAutomationEditor,
  useAutomationJobRuns,
  useAutomationTrigger,
  useAutomationTriggerRuns,
  useDeleteAutomationJob,
  useDeleteAutomationTrigger,
  useTriggerAutomationJob,
  useUpdateAutomationJob,
  useUpdateAutomationTrigger,
} from "@/systems/automation";
import { LoopsApiError, useLoop } from "@/systems/loops";
import { useSettingsAutomation } from "@/systems/settings";
import { toWorkspaceCommandSelectOptions, useActiveWorkspace } from "@/systems/workspace";

const RUN_LIMIT = { limit: 10 } as const;
const MISSING_MESSAGE = "This automation is no longer available.";

function isNotFound(error: unknown): boolean {
  return error instanceof AutomationApiError && error.status === 404;
}

function errorMessage(error: unknown, fallback: string): string {
  return error instanceof Error && error.message.trim() ? error.message : fallback;
}

/**
 * One detail view-model for both daemon entities. The route keeps the entity
 * (`/automations/jobs/:id` | `/automations/triggers/:id`) so every read and
 * mutation goes to one resource; the page itself renders one `AutomationView`.
 */
export function useAutomationDetailPage(
  kind: AutomationEntityKind,
  id: string,
  search: AutomationDetailRouteSearch = {}
) {
  const navigate = useNavigate();
  const liveDataEnabled = useCurrentWindowLiveDataEnabled();
  const { activeWorkspaceId, isLoading: workspaceLoading, workspaces } = useActiveWorkspace();
  const isJob = kind === "job";
  const [queuedRun, setQueuedRun] = useState<AutomationRun | null>(null);

  const jobQuery = useAutomationJob(id, { enabled: liveDataEnabled && isJob && Boolean(id) });
  const triggerQuery = useAutomationTrigger(id, {
    enabled: liveDataEnabled && !isJob && Boolean(id),
  });
  const detailQuery = isJob ? jobQuery : triggerQuery;
  const loaded = detailQuery.data;
  const accessError = automationWorkspaceAccessError(loaded, activeWorkspaceId, workspaceLoading);
  const entity = workspaceLoading || accessError ? undefined : loaded;

  const runsEnabled = liveDataEnabled && Boolean(entity);
  const jobRunsQuery = useAutomationJobRuns(id, RUN_LIMIT, { enabled: runsEnabled && isJob });
  const triggerRunsQuery = useAutomationTriggerRuns(id, RUN_LIMIT, {
    enabled: runsEnabled && !isJob,
  });
  const runsQuery = isJob ? jobRunsQuery : triggerRunsQuery;

  const settingsQuery = useSettingsAutomation();
  const timeZone = settingsQuery.data?.config?.timezone?.trim() || undefined;
  const runtimeUnavailable = automationUnavailableMessage(
    settingsQuery.data?.runtime ?? null,
    detailQuery.error
  );

  const workspaceNameById = (workspaceId: string | undefined): string | null => {
    if (!workspaceId) return null;
    return workspaces.find(workspace => workspace.id === workspaceId)?.name ?? workspaceId;
  };
  const sentenceContext = {
    ...(timeZone ? { timeZone } : {}),
    workspaceName: workspaceNameById,
  };
  const view = entity ? toAutomationView(entity, sentenceContext) : undefined;

  const target = entity ? projectAutomationTarget(entity) : null;
  const loopTarget = target?.kind === "loop" ? target : null;
  const loopQuery = useLoop(
    loopTarget?.workspaceId ?? "",
    loopTarget?.loopName ?? "",
    liveDataEnabled && loopTarget !== null
  );
  const loopMissing = loopQuery.error instanceof LoopsApiError && loopQuery.error.status === 404;

  const updateJob = useUpdateAutomationJob();
  const updateTrigger = useUpdateAutomationTrigger();
  const deleteJob = useDeleteAutomationJob();
  const deleteTrigger = useDeleteAutomationTrigger();
  const runNow = useTriggerAutomationJob();
  const updateMutation = isJob ? updateJob : updateTrigger;
  const deleteMutation = isJob ? deleteJob : deleteTrigger;

  const editorWorkspaceId = automationEditorWorkspaceId(entity, activeWorkspaceId);
  const editorWorkspaces = toWorkspaceCommandSelectOptions(workspaces);
  const editor = useAutomationEditor({
    activeWorkspaceId: editorWorkspaceId,
    workspaces: editorWorkspaces,
  });

  const openEdit = (section?: AutomationEditorSection) => {
    if (!entity) return;
    editor.openEdit(entity, { section });
  };

  /** Drops the consumed `edit` deep link without adding a history entry. */
  const clearEditParam = () => {
    const next = { ...search, edit: undefined };
    void (isJob
      ? navigate({
          to: "/automations/jobs/$jobId",
          params: { jobId: id },
          search: next,
          replace: true,
        })
      : navigate({
          to: "/automations/triggers/$triggerId",
          params: { triggerId: id },
          search: next,
          replace: true,
        }));
  };

  // `?edit=options` ("Set up retries") opens the editor once, then leaves the URL.
  const editIntent = useStore(automationEditIntentLogic);
  const editKey = search.edit ? `${id}:${search.edit}` : null;
  const editReady = Boolean(entity) && view?.canEdit === true;
  useEffect(() => {
    editIntent.trigger.intentObserved({
      key: editKey,
      ready: editReady,
      consume: () => {
        openEdit(search.edit === "options" ? "options" : undefined);
        clearEditParam();
      },
    });
  });

  const persistedRuns = entity ? (runsQuery.data ?? []) : [];
  const runs =
    isJob && queuedRun && !persistedRuns.some(run => run.id === queuedRun.id)
      ? [queuedRun, ...persistedRuns].slice(0, RUN_LIMIT.limit)
      : persistedRuns;

  const handleToggleEnabled = async (enabled: boolean) => {
    if (!entity || updateMutation.isPending) return;
    try {
      await updateMutation.mutateAsync({
        data: { enabled },
        id: entity.id,
        profile: entity.profile_name,
      });
      toast.success(`${enabled ? "Turned on" : "Turned off"} ${entity.name}.`);
    } catch {
      toast.error(`Couldn't turn ${enabled ? "on" : "off"} ${entity.name}. Try again.`);
    }
  };

  const handleRunNow = async () => {
    if (!entity || !isJob || runtimeUnavailable || runNow.isPending) return;
    try {
      const run = await runNow.mutateAsync({ id: entity.id, profile: entity.profile_name });
      setQueuedRun(run);
      toast.success(`Queued run ${run.id}.`);
    } catch (error) {
      toast.error(errorMessage(error, "Couldn't start this automation."));
    }
  };

  const handleDelete = async () => {
    if (!entity) throw new Error(MISSING_MESSAGE);
    try {
      await deleteMutation.mutateAsync({ id: entity.id, profile: entity.profile_name });
    } catch (error) {
      // Deleted elsewhere already: the user's intent is satisfied.
      if (!isNotFound(error)) throw error;
    }
    toast.success(`Deleted ${entity.name}.`);
    void navigate({ to: "/automations", search: automationListingSearch(search), replace: true });
  };

  const status: AutomationDetailStatus = accessError
    ? "elsewhere"
    : entity
      ? "ready"
      : detailQuery.isLoading || workspaceLoading
        ? "loading"
        : detailQuery.error && !isNotFound(detailQuery.error)
          ? "error"
          : "missing";

  return {
    editorDialogProps: editor.editorDialogProps,
    isJob,
    panel: {
      status,
      statusMessage:
        status === "elsewhere"
          ? (accessError?.message ?? "")
          : status === "error"
            ? errorMessage(detailQuery.error, "Failed to load this automation.")
            : MISSING_MESSAGE,
      entity,
      view,
      sentenceContext,
      loopWorkspaceName: loopTarget ? workspaceNameById(loopTarget.workspaceId) : null,
      loopMissing,
      lastRanAt: automationLastRanAt(runs) ?? view?.lastRun?.startedAt ?? null,
      runs,
      runsError: entity ? runsQuery.error : null,
      runsLoading: entity ? runsQuery.isLoading : false,
      state: {
        isDeleting: deleteMutation.isPending,
        isTogglePending: updateMutation.isPending,
        isRunNowPending: runNow.isPending,
        isRunNowDisabled: runtimeUnavailable !== null,
      },
      onBack: () => void navigate({ to: "/automations", search: automationListingSearch(search) }),
      onDelete: handleDelete,
      onEdit: () => openEdit(),
      onRetryRuns: () => void runsQuery.refetch(),
      onRunNow: () => void handleRunNow(),
      // Opens in place: the `?edit=options` deep link is for arrivals, not a history entry.
      onSetUpRetries: () => openEdit("options"),
      onToggleEnabled: (enabled: boolean) => void handleToggleEnabled(enabled),
    },
  };
}
