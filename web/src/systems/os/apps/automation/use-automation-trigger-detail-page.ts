import { useNavigate } from "@tanstack/react-router";
import { toast } from "sonner";

import { useCurrentWindowLiveDataEnabled } from "../../hooks/use-window-live-data-enabled";
import {
  automationMatchesActiveWorkspace,
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
  const loadedTrigger = triggerDetailQuery.data;
  const canAccessTrigger =
    loadedTrigger !== undefined &&
    !workspaceLoading &&
    automationMatchesActiveWorkspace(loadedTrigger, activeWorkspaceId);
  const triggerRunsQuery = useAutomationTriggerRuns(
    triggerId,
    { limit: 10 },
    { enabled: liveDataEnabled && Boolean(triggerId) && canAccessTrigger }
  );

  const updateMutation = useUpdateAutomationTrigger();
  const deleteMutation = useDeleteAutomationTrigger();

  const trigger = canAccessTrigger ? loadedTrigger : undefined;
  const accessError = automationWorkspaceAccessError(
    "trigger",
    loadedTrigger,
    activeWorkspaceId,
    workspaceLoading
  );

  const editor = useAutomationTriggerEditor({
    activeWorkspaceId,
    workspaces: toWorkspaceCommandSelectOptions(workspaces),
  });

  // The detail page speaks in workspace names; ids stay on the inspection surface.
  const workspaceNameById = (id: string | undefined): string | null => {
    if (!id) return null;
    return workspaces.find(workspace => workspace.id === id)?.name ?? id;
  };
  const loopTarget = trigger ? projectAutomationTarget(trigger) : null;

  const handleToggleEnabled = async (enabled: boolean) => {
    if (!trigger) return;
    try {
      await updateMutation.mutateAsync({ data: { enabled }, id: trigger.id });
      toast.success(`${enabled ? "Enabled" : "Disabled"} ${trigger.name}.`);
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "Failed to update automation state");
    }
  };

  const handleDelete = async () => {
    if (!trigger) throw new Error("This trigger is no longer available.");
    await deleteMutation.mutateAsync({ id: trigger.id });
    toast.success(`Deleted ${trigger.name}.`);
    void navigate({ to: "/triggers", replace: true });
  };

  return {
    editorDialogProps: editor.editorDialogProps,
    error: trigger ? null : (accessError ?? triggerDetailQuery.error),
    handleBack: () => void navigate({ to: "/triggers" }),
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
    isLoading: (triggerDetailQuery.isLoading || workspaceLoading) && !trigger && !accessError,
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
