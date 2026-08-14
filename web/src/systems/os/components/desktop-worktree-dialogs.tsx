import type { useWorktreeDialogTargets } from "../hooks/use-worktree-dialog-targets";
import type { DesktopShellModel } from "../hooks/use-desktop-shell-model";
import {
  WorktreeAdoptDialogBoundary,
  WorktreeContextDialogBoundary,
  WorktreeCreateDialogBoundary,
  WorktreeMissingDialogBoundary,
  WorktreeRemoveDialogBoundary,
} from "./worktree-dialog-boundaries";

interface DesktopWorktreeDialogsProps {
  model: DesktopShellModel;
  worktreeDialogs: ReturnType<typeof useWorktreeDialogTargets>;
  /** The acting scope a create/adopt selection binds (focused window or shell). */
  scopeId: string;
}

/** Mounts each worktree dialog boundary only while its target exists. */
export function DesktopWorktreeDialogs({
  model,
  worktreeDialogs,
  scopeId,
}: DesktopWorktreeDialogsProps) {
  return (
    <>
      {model.worktreeCreateWorkspaceId ? (
        <WorktreeCreateDialogBoundary
          workspaceId={model.worktreeCreateWorkspaceId}
          workspaceName={
            model.workspaces.find(workspace => workspace.id === model.worktreeCreateWorkspaceId)
              ?.name ?? "workspace"
          }
          listing={model.worktreesByWorkspace[model.worktreeCreateWorkspaceId]}
          scopeId={scopeId}
          onOpenChange={open =>
            model.setWorktreeCreateWorkspaceId(open ? model.worktreeCreateWorkspaceId : null)
          }
        />
      ) : null}
      {worktreeDialogs.adoptTarget ? (
        <WorktreeAdoptDialogBoundary
          workspaceId={worktreeDialogs.adoptTarget.workspaceId}
          discovered={worktreeDialogs.adoptTarget.discovered}
          scopeId={scopeId}
          onClose={worktreeDialogs.closeAdopt}
        />
      ) : null}
      {worktreeDialogs.removeTarget ? (
        <WorktreeRemoveDialogBoundary
          workspaceId={worktreeDialogs.removeTarget.workspaceId}
          worktree={worktreeDialogs.removeTarget.worktree}
          onClose={worktreeDialogs.closeRemove}
        />
      ) : null}
      {worktreeDialogs.missingTarget ? (
        <WorktreeMissingDialogBoundary
          workspaceId={worktreeDialogs.missingTarget.workspaceId}
          worktree={worktreeDialogs.missingTarget.worktree}
          onClose={worktreeDialogs.closeMissing}
        />
      ) : null}
      {worktreeDialogs.contextTarget ? (
        <WorktreeContextDialogBoundary
          workspaceId={worktreeDialogs.contextTarget.workspaceId}
          worktree={worktreeDialogs.contextTarget.worktree}
          onCleanUp={worktreeDialogs.requestContextCleanup}
          onClose={worktreeDialogs.closeContext}
        />
      ) : null}
    </>
  );
}
