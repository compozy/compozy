import { useNavigate } from "@tanstack/react-router";
import { toast } from "sonner";

import { useCurrentWindowLiveDataEnabled } from "../../hooks/use-window-live-data-enabled";
import {
  automationEditorWorkspaceId,
  automationWorkspaceAccessError,
  projectAutomationTarget,
  useAutomationTrigger,
  useAutomationTriggerEditor,
  useAutomationTriggerRuns,
  useDeleteAutomationTrigger,
  useUpdateAutomationTrigger,
} from "@/systems/automation";
import { toWorkspaceCommandSelectOptions, useActiveWorkspace } from "@/systems/workspace";

/** Detail view-model for a single automation trigger resolved from `triggerId`. */
export function useAutomationTriggerDetailPage(triggerId: string) {
  const navigate = useNavigate();
  const liveDataEnabled = useCurrentWindowLiveDataEnabled();
  const { activeWorkspaceId, isLoading: workspaceLoading, workspaces } = useActiveWorkspace();

  const triggerDetailQuery = useAutomationTrigger(triggerId, {
    enabled: liveDataEnabled && Boolean(triggerId),
  });
  const { trigger, error, isLoading } = projectTriggerDetail(
    triggerDetailQuery,
    activeWorkspaceId,
    workspaceLoading
  );
  const triggerRunsQuery = useAutomationTriggerRuns(
    triggerId,
    { limit: 10 },
    { enabled: liveDataEnabled && Boolean(triggerId) && Boolean(trigger) }
  );

  const updateMutation = useUpdateAutomationTrigger();
  const deleteMutation = useDeleteAutomationTrigger();

  const editor = useAutomationTriggerEditor({
    activeWorkspaceId: automationEditorWorkspaceId(trigger, activeWorkspaceId),
    workspaces: toWorkspaceCommandSelectOptions(workspaces),
  });

  // The detail page prefers workspace names and falls back to the persisted id.
  const workspaceNameById = (id: string | undefined): string | null => {
    if (!id) return null;
    return workspaces.find(workspace => workspace.id === id)?.name ?? id;
  };
  const loopTarget = trigger ? projectAutomationTarget(trigger) : null;

  const handleToggleEnabled = async (enabled: boolean) => {
    if (!trigger) return;
    try {
      await updateMutation.mutateAsync({
        data: { enabled },
        id: trigger.id,
        profile: trigger.profile_name,
      });
      toast.success(`${enabled ? "Enabled" : "Disabled"} ${trigger.name}.`);
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "Failed to update automation state");
    }
  };

  const handleDelete = async () => {
    if (!trigger) throw new Error("This trigger is no longer available.");
    await deleteMutation.mutateAsync({ id: trigger.id, profile: trigger.profile_name });
    toast.success(`Deleted ${trigger.name}.`);
    void navigate({ to: "/automations", replace: true });
  };

  return {
    editorDialogProps: editor.editorDialogProps,
    error,
    handleBack: () => void navigate({ to: "/automations" }),
    handleDelete,
    handleEdit: () => {
      if (trigger) editor.openEdit(trigger);
    },
    handleRetryRuns: () => {
      void triggerRunsQuery.refetch();
    },
    handleToggleEnabled: (enabled: boolean) => {
      void handleToggleEnabled(enabled);
    },
    isDeleting: deleteMutation.isPending,
    isLoading,
    isTogglePending: updateMutation.isPending,
    loopWorkspaceName:
      loopTarget?.kind === "loop" ? workspaceNameById(loopTarget.workspaceId) : null,
    runs: trigger ? (triggerRunsQuery.data ?? []) : [],
    runsError: trigger ? triggerRunsQuery.error : null,
    runsLoading: trigger ? triggerRunsQuery.isLoading : false,
    trigger,
    workspaceName: workspaceNameById(trigger?.workspace_id),
  };
}

function projectTriggerDetail(
  triggerDetailQuery: ReturnType<typeof useAutomationTrigger>,
  activeWorkspaceId: string | null | undefined,
  workspaceLoading: boolean
) {
  const loadedTrigger = triggerDetailQuery.data;
  const accessError = automationWorkspaceAccessError(
    "trigger",
    loadedTrigger,
    activeWorkspaceId,
    workspaceLoading
  );
  const trigger = workspaceLoading || accessError ? undefined : loadedTrigger;

  return {
    trigger,
    error: trigger ? null : (accessError ?? triggerDetailQuery.error),
    isLoading: (triggerDetailQuery.isLoading || workspaceLoading) && !trigger && !accessError,
  };
}
