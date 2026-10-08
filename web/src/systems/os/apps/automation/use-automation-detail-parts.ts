import { useEffect } from "react";
import { useNavigate } from "@tanstack/react-router";
import { useStore } from "@xstate/store-react";
import { toast } from "sonner";

import { automationEditIntentLogic } from "./automation-edit-intent-store";
import {
  AutomationApiError,
  automationListingSearch,
  automationWorkspaceAccessError,
  projectAutomationTarget,
  type AutomationDetailRouteSearch,
  type AutomationDetailStatus,
  type AutomationEditorSection,
  type AutomationEntity,
  type AutomationEntityKind,
  type AutomationRun,
  useAutomationJob,
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

export const RUN_LIMIT = { limit: 10 } as const;
export const MISSING_MESSAGE = "This automation is no longer available.";

function isNotFound(error: unknown): boolean {
  return error instanceof AutomationApiError && error.status === 404;
}

export function errorMessage(error: unknown, fallback: string): string {
  return error instanceof Error && error.message.trim() ? error.message : fallback;
}

interface DetailStatusInput {
  accessError: Error | null;
  entity: AutomationEntity | undefined;
  loading: boolean;
  error: unknown;
}

/** Which page renders: ready, still loading, another project's, missing (404) or failed. */
export function automationDetailStatus(input: DetailStatusInput): AutomationDetailStatus {
  if (input.accessError) return "elsewhere";
  if (input.entity) return "ready";
  if (input.loading) return "loading";
  return input.error && !isNotFound(input.error) ? "error" : "missing";
}

export function automationDetailStatusMessage(
  status: AutomationDetailStatus,
  accessError: Error | null,
  error: unknown
): string {
  if (status === "elsewhere") return accessError?.message ?? "";
  if (status === "error") return errorMessage(error, "Failed to load this automation.");
  return MISSING_MESSAGE;
}

/** A queued Run now row leads the persisted list until the refetch includes it. */
export function withQueuedRun(
  persisted: AutomationRun[],
  queued: AutomationRun | null
): AutomationRun[] {
  if (!queued || persisted.some(run => run.id === queued.id)) return persisted;
  return [queued, ...persisted].slice(0, RUN_LIMIT.limit);
}

/** The routed entity, unless it belongs to another project than the selected one. */
export function useAutomationDetailEntity(
  kind: AutomationEntityKind,
  id: string,
  liveDataEnabled: boolean,
  activeWorkspaceId: string | null | undefined,
  workspaceLoading: boolean
) {
  const isJob = kind === "job";
  const jobQuery = useAutomationJob(id, { enabled: liveDataEnabled && isJob && Boolean(id) });
  const triggerQuery = useAutomationTrigger(id, {
    enabled: liveDataEnabled && !isJob && Boolean(id),
  });
  const detailQuery = isJob ? jobQuery : triggerQuery;
  const accessError = automationWorkspaceAccessError(
    detailQuery.data,
    activeWorkspaceId,
    workspaceLoading
  );
  const entity: AutomationEntity | undefined =
    workspaceLoading || accessError ? undefined : detailQuery.data;
  return { accessError, detailQuery, entity };
}

/** The last runs of the routed entity; read only once the entity is visible. */
export function useAutomationDetailRuns(kind: AutomationEntityKind, id: string, enabled: boolean) {
  const isJob = kind === "job";
  const jobRunsQuery = useAutomationJobRuns(id, RUN_LIMIT, { enabled: enabled && isJob });
  const triggerRunsQuery = useAutomationTriggerRuns(id, RUN_LIMIT, {
    enabled: enabled && !isJob,
  });
  return isJob ? jobRunsQuery : triggerRunsQuery;
}

/** A Loop target and whether that Loop is gone from its project. */
export function useLoopTargetPresence(entity: AutomationEntity | undefined, enabled: boolean) {
  const target = entity ? projectAutomationTarget(entity) : null;
  const loopTarget = target?.kind === "loop" ? target : null;
  const loopQuery = useLoop(
    loopTarget?.workspaceId ?? "",
    loopTarget?.loopName ?? "",
    enabled && loopTarget !== null
  );
  const loopMissing = loopQuery.error instanceof LoopsApiError && loopQuery.error.status === 404;
  return { loopTarget, loopMissing };
}

/** The mutations of the routed entity's kind (Run now is jobs-only). */
export function useAutomationDetailMutations(kind: AutomationEntityKind) {
  const updateJob = useUpdateAutomationJob();
  const updateTrigger = useUpdateAutomationTrigger();
  const deleteJob = useDeleteAutomationJob();
  const deleteTrigger = useDeleteAutomationTrigger();
  const runNow = useTriggerAutomationJob();
  const isJob = kind === "job";
  return {
    update: isJob ? updateJob : updateTrigger,
    remove: isJob ? deleteJob : deleteTrigger,
    runNow,
  };
}

interface EditIntentInput {
  kind: AutomationEntityKind;
  id: string;
  search: AutomationDetailRouteSearch;
  ready: boolean;
  openEdit: (section?: AutomationEditorSection) => void;
}

/** `?edit=options` ("Set up retries") opens the editor once, then leaves the URL. */
export function useAutomationEditIntent({ kind, id, search, ready, openEdit }: EditIntentInput) {
  const navigate = useNavigate();
  const editIntent = useStore(automationEditIntentLogic);
  const editKey = search.edit ? `${id}:${search.edit}` : null;

  // Drops the consumed deep link without adding a history entry.
  const clearEditParam = () => {
    const next = { ...search, edit: undefined };
    void (kind === "job"
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

  useEffect(() => {
    editIntent.trigger.intentObserved({
      key: editKey,
      ready,
      consume: () => {
        openEdit(search.edit === "options" ? "options" : undefined);
        clearEditParam();
      },
    });
  });
}

interface DetailActionsInput {
  entity: AutomationEntity | undefined;
  kind: AutomationEntityKind;
  search: AutomationDetailRouteSearch;
  runtimeUnavailable: boolean;
  mutations: ReturnType<typeof useAutomationDetailMutations>;
  onQueued: (run: AutomationRun) => void;
}

/** On/Off, Run now and Delete — non-optimistic, each reporting through a toast. */
export function useAutomationDetailActions({
  entity,
  kind,
  search,
  runtimeUnavailable,
  mutations,
  onQueued,
}: DetailActionsInput) {
  const navigate = useNavigate();
  const { update, remove, runNow } = mutations;

  const toggleEnabled = async (enabled: boolean) => {
    if (!entity || update.isPending) return;
    try {
      await update.mutateAsync({ data: { enabled }, id: entity.id, profile: entity.profile_name });
      toast.success(`${enabled ? "Turned on" : "Turned off"} ${entity.name}.`);
    } catch {
      toast.error(`Couldn't turn ${enabled ? "on" : "off"} ${entity.name}. Try again.`);
    }
  };

  const startNow = async () => {
    if (!entity || kind !== "job" || runtimeUnavailable || runNow.isPending) return;
    try {
      const run = await runNow.mutateAsync({ id: entity.id, profile: entity.profile_name });
      onQueued(run);
      toast.success(`Queued run ${run.id}.`);
    } catch (error) {
      toast.error(errorMessage(error, "Couldn't start this automation."));
    }
  };

  const deleteEntity = async () => {
    if (!entity) throw new Error(MISSING_MESSAGE);
    try {
      await remove.mutateAsync({ id: entity.id, profile: entity.profile_name });
    } catch (error) {
      // Deleted elsewhere already: the user's intent is satisfied.
      if (!isNotFound(error)) throw error;
    }
    toast.success(`Deleted ${entity.name}.`);
    void navigate({ to: "/automations", search: automationListingSearch(search), replace: true });
  };

  return { deleteEntity, startNow, toggleEnabled };
}
