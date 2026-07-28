import { useState } from "react";
import { useNavigate } from "@tanstack/react-router";
import { toast } from "sonner";

import {
  useAutomationJob,
  useAutomationJobEditor,
  useAutomationJobRuns,
  automationMatchesActiveWorkspace,
  automationWorkspaceAccessError,
  useDeleteAutomationJob,
  useTriggerAutomationJob,
  useUpdateAutomationJob,
} from "@/systems/automation";
import type { AutomationRun } from "@/systems/automation";
import { useSettingsAutomation } from "@/systems/settings";
import { toWorkspaceCommandSelectOptions, useActiveWorkspace } from "@/systems/workspace";

import { automationUnavailableMessage } from "./use-automation-page-base";

/** Detail view-model for a single automation job resolved from the route `jobId`. */
export function useAutomationJobDetailPage(jobId: string) {
  const navigate = useNavigate();
  const { activeWorkspaceId, isLoading: workspaceLoading, workspaces } = useActiveWorkspace();
  const [queuedRun, setQueuedRun] = useState<AutomationRun | null>(null);

  const jobDetailQuery = useAutomationJob(jobId, { enabled: Boolean(jobId) });
  const loadedJob = jobDetailQuery.data;
  const canAccessJob =
    loadedJob !== undefined &&
    !workspaceLoading &&
    automationMatchesActiveWorkspace(loadedJob, activeWorkspaceId);
  const jobRunsQuery = useAutomationJobRuns(
    jobId,
    { limit: 10 },
    { enabled: Boolean(jobId) && canAccessJob }
  );
  const settingsQuery = useSettingsAutomation();
  const runtimeUnavailableMessage = automationUnavailableMessage(
    "jobs",
    settingsQuery.data?.runtime ?? null,
    jobDetailQuery.error
  );

  const updateMutation = useUpdateAutomationJob();
  const deleteMutation = useDeleteAutomationJob();
  const triggerMutation = useTriggerAutomationJob();

  const job = canAccessJob ? loadedJob : undefined;
  const accessError = automationWorkspaceAccessError(
    "job",
    loadedJob,
    activeWorkspaceId,
    workspaceLoading
  );
  const persistedRuns = job ? (jobRunsQuery.data ?? []) : [];
  const runs =
    job && queuedRun && !persistedRuns.some(run => run.id === queuedRun.id)
      ? [queuedRun, ...persistedRuns]
      : persistedRuns;

  const editor = useAutomationJobEditor({
    activeWorkspaceId,
    workspaces: toWorkspaceCommandSelectOptions(workspaces),
  });

  const handleToggleEnabled = async (enabled: boolean) => {
    if (!job) return;
    try {
      await updateMutation.mutateAsync({ data: { enabled }, id: job.id });
      toast.success(`${enabled ? "Enabled" : "Disabled"} ${job.name}.`);
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "Failed to update automation state");
    }
  };

  const handleTriggerNow = async () => {
    if (!job || runtimeUnavailableMessage) return;
    try {
      const run = await triggerMutation.mutateAsync({ id: job.id });
      setQueuedRun(run);
      toast.success(`Queued run ${run.id}.`);
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "Failed to trigger automation job");
    }
  };

  const handleDelete = async () => {
    if (!job) throw new Error("This job is no longer available.");
    await deleteMutation.mutateAsync({ id: job.id });
    toast.success(`Deleted ${job.name}.`);
    void navigate({ to: "/jobs", replace: true });
  };

  return {
    editorDialogProps: editor.editorDialogProps,
    error: job ? null : (accessError ?? jobDetailQuery.error),
    handleBack: () => void navigate({ to: "/jobs" }),
    handleDelete,
    handleEdit: () => {
      if (job) editor.openEdit(job);
    },
    handleToggleEnabled: (enabled: boolean) => {
      void handleToggleEnabled(enabled);
    },
    handleTriggerNow: () => {
      void handleTriggerNow();
    },
    isDeleting: deleteMutation.isPending,
    isLoading: (jobDetailQuery.isLoading || workspaceLoading) && !job && !accessError,
    isTogglePending: updateMutation.isPending,
    isTriggerPending: triggerMutation.isPending,
    isTriggerDisabled: runtimeUnavailableMessage !== null,
    job,
    runs,
    runsError: job ? jobRunsQuery.error : null,
    runsLoading: job ? jobRunsQuery.isLoading : false,
  };
}
