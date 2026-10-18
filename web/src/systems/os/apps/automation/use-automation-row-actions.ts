import { useState } from "react";
import { useSelector, useStore } from "@xstate/store-react";
import { toast } from "sonner";

import {
  useDeleteAutomationJob,
  useDeleteAutomationTrigger,
  useTriggerAutomationJob,
  useUpdateAutomationJob,
  useUpdateAutomationTrigger,
  type AutomationJob,
  type AutomationTrigger,
  type AutomationView,
} from "@/systems/automation";

import { automationPendingLogic } from "./automation-pending-store";

function viewKey(view: Pick<AutomationView, "kind" | "id">): string {
  return `${view.kind}:${view.id}`;
}

function errorMessage(error: unknown, fallback: string): string {
  return error instanceof Error && error.message ? error.message : fallback;
}

interface AutomationRowActionsInput {
  /** Runtime unavailable: switch and Run now requests are refused. */
  unavailable: boolean;
  findEntity: (view: AutomationView) => AutomationJob | AutomationTrigger | undefined;
  openEdit: (entity: AutomationJob | AutomationTrigger) => void;
}

/** Row actions that wait for the daemon (On/Off, Run now) plus Edit and Delete. */
export function useAutomationRowActions({
  unavailable,
  findEntity,
  openEdit,
}: AutomationRowActionsInput) {
  const pendingStore = useStore(automationPendingLogic);
  const pendingIds = useSelector(pendingStore, snapshot => snapshot.context.pendingIds);
  const [deleteTarget, setDeleteTarget] = useState<AutomationView | null>(null);
  const updateJob = useUpdateAutomationJob();
  const updateTrigger = useUpdateAutomationTrigger();
  const runJob = useTriggerAutomationJob();
  const deleteJob = useDeleteAutomationJob();
  const deleteTrigger = useDeleteAutomationTrigger();

  const requestAction = (id: string, run: () => Promise<void>) =>
    pendingStore.trigger.actionRequested({ id, permitted: !unavailable, run });

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
    openEdit(entity);
  };

  const confirmDelete = async () => {
    const view = deleteTarget;
    if (!view) return;
    const mutation = view.kind === "job" ? deleteJob : deleteTrigger;
    await mutation.mutateAsync({ id: view.id, profile: view.profileName });
    toast.success(`Deleted ${view.name}.`);
    setDeleteTarget(null);
  };

  return {
    confirmDelete,
    deletePending: deleteJob.isPending || deleteTrigger.isPending,
    deleteTarget,
    edit,
    isRunPending: (view: AutomationView) => pendingIds.has(`run:${viewKey(view)}`),
    isTogglePending: (view: AutomationView) => pendingIds.has(`toggle:${viewKey(view)}`),
    runNow,
    setDeleteTarget,
    toggleEnabled,
  };
}
