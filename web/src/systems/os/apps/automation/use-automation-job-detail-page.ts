import { useState } from "react";
import { useNavigate } from "@tanstack/react-router";
import { toast } from "sonner";

import { automationUnavailableMessage } from "./use-automation-page-base";
import { useCurrentWindowLiveDataEnabled } from "../../hooks/use-window-live-data-enabled";
import {
  automationEditorWorkspaceId,
  type AutomationRun,
  automationWorkspaceAccessError,
  useAutomationJob,
  useAutomationJobEditor,
  useAutomationJobRuns,
  useDeleteAutomationJob,
  useTriggerAutomationJob,
  useUpdateAutomationJob,
} from "@/systems/automation";
import { useSettingsAutomation } from "@/systems/settings";
import { toWorkspaceCommandSelectOptions, useActiveWorkspace } from "@/systems/workspace";

/** Detail view-model for a single automation job resolved from the route `jobId`. */
export function useAutomationJobDetailPage(jobId: string) {
  const navigate = useNavigate();
  const liveDataEnabled = useCurrentWindowLiveDataEnabled();
  const { activeWorkspaceId, isLoading: workspaceLoading, workspaces } = useActiveWorkspace();
  const [queuedRun, setQueuedRun] = useState<AutomationRun | null>(null);

  const jobDetailQuery = useAutomationJob(jobId, {
    enabled: liveDataEnabled && Boolean(jobId),
  });
  const { job, error, isLoading } = projectJobDetail(
    jobDetailQuery,
    activeWorkspaceId,
    workspaceLoading
  );
  const jobRunsQuery = useAutomationJobRuns(
    jobId,
    { limit: 10 },
    { enabled: liveDataEnabled && Boolean(jobId) && Boolean(job) }
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

  const persistedRuns = job ? (jobRunsQuery.data ?? []) : [];
  const runs =
    job && queuedRun && !persistedRuns.some(run => run.id === queuedRun.id)
      ? [queuedRun, ...persistedRuns]
      : persistedRuns;

  const editor = useAutomationJobEditor({
    activeWorkspaceId: automationEditorWorkspaceId(job, activeWorkspaceId),
    workspaces: toWorkspaceCommandSelectOptions(workspaces),
  });

  const handleToggleEnabled = async (enabled: boolean) => {
    if (!job) return;
    try {
      await updateMutation.mutateAsync({
        data: { enabled },
        id: job.id,
        profile: job.profile_name,
      });
      toast.success(`${enabled ? "Enabled" : "Disabled"} ${job.name}.`);
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "Failed to update automation state");
    }
  };

  const handleTriggerNow = async () => {
    if (!job || runtimeUnavailableMessage) return;
    try {
      const run = await triggerMutation.mutateAsync({ id: job.id, profile: job.profile_name });
      setQueuedRun(run);
      toast.success(`Queued run ${run.id}.`);
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "Failed to trigger automation job");
    }
  };

  const handleDelete = async () => {
    if (!job) throw new Error("This job is no longer available.");
    await deleteMutation.mutateAsync({ id: job.id, profile: job.profile_name });
    toast.success(`Deleted ${job.name}.`);
    void navigate({ to: "/jobs", replace: true });
  };

  return {
    editorDialogProps: editor.editorDialogProps,
    error,
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
    isLoading,
    isTogglePending: updateMutation.isPending,
    isTriggerPending: triggerMutation.isPending,
    isTriggerDisabled: runtimeUnavailableMessage !== null,
    job,
    runs,
    runsError: job ? jobRunsQuery.error : null,
    runsLoading: job ? jobRunsQuery.isLoading : false,
  };
}

function projectJobDetail(
  jobDetailQuery: ReturnType<typeof useAutomationJob>,
  activeWorkspaceId: string | null | undefined,
  workspaceLoading: boolean
) {
  const loadedJob = jobDetailQuery.data;
  const accessError = automationWorkspaceAccessError(
    "job",
    loadedJob,
    activeWorkspaceId,
    workspaceLoading
  );
  const job = workspaceLoading || accessError ? undefined : loadedJob;

  return {
    job,
    error: job ? null : (accessError ?? jobDetailQuery.error),
    isLoading: (jobDetailQuery.isLoading || workspaceLoading) && !job && !accessError,
  };
}
