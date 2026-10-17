import { lazy, Suspense, type ReactNode } from "react";

import { ConfirmDialog } from "@compozy/ui";

import { useWorktrees, WorktreeBulkRemoveDialog } from "@/systems/workspace";

import type { useWorktreeDialogTargets } from "../hooks/use-worktree-dialog-targets";
import type { DesktopShellModel } from "../hooks/use-desktop-shell-model";
import { OsWindowErrorBoundary } from "./os-window-error-boundary";

const WorktreeAdoptDialogBoundary = lazy(() =>
  import("./worktree-dialog-boundaries").then(module => ({
    default: module.WorktreeAdoptDialogBoundary,
  }))
);
const WorktreeContextDialogBoundary = lazy(() =>
  import("./worktree-dialog-boundaries").then(module => ({
    default: module.WorktreeContextDialogBoundary,
  }))
);
const WorktreeCreateDialogBoundary = lazy(() =>
  import("./worktree-dialog-boundaries").then(module => ({
    default: module.WorktreeCreateDialogBoundary,
  }))
);
const WorktreeMissingDialogBoundary = lazy(() =>
  import("./worktree-dialog-boundaries").then(module => ({
    default: module.WorktreeMissingDialogBoundary,
  }))
);
const WorktreeRemoveDialogBoundary = lazy(() =>
  import("./worktree-dialog-boundaries").then(module => ({
    default: module.WorktreeRemoveDialogBoundary,
  }))
);

interface DesktopWorktreeDialogsProps {
  model: DesktopShellModel;
  worktreeDialogs: ReturnType<typeof useWorktreeDialogTargets>;
  /** The acting scope a create/adopt selection binds (focused window or shell). */
  scopeId: string;
}

function DeferredWorktreeDialog({
  children,
  onClose,
}: {
  children: ReactNode;
  onClose: () => void;
}) {
  return (
    <OsWindowErrorBoundary
      title="Worktree dialog"
      fallback={
        <ConfirmDialog
          open
          title="The worktree dialog couldn't open"
          description="Reload CompozyOS to try again."
          cancelLabel="Close"
          confirmLabel="Reload"
          onConfirm={() => window.location.reload()}
          onOpenChange={open => {
            if (!open) onClose();
          }}
        />
      }
    >
      <Suspense fallback={null}>{children}</Suspense>
    </OsWindowErrorBoundary>
  );
}

/** Mounts each worktree dialog boundary only while its target exists. */
export function DesktopWorktreeDialogs({
  model,
  worktreeDialogs,
  scopeId,
}: DesktopWorktreeDialogsProps) {
  const createListing = useWorktrees(model.worktreeCreateWorkspaceId, {
    enabled: model.worktreeCreateWorkspaceId !== null,
  });

  return (
    <>
      {worktreeDialogs.removalBatch ? (
        <WorktreeBulkRemoveDialog
          batch={worktreeDialogs.removalBatch}
          onClose={worktreeDialogs.closeRemoveBatch}
        />
      ) : null}
      {model.worktreeCreateWorkspaceId ? (
        <DeferredWorktreeDialog onClose={() => model.setWorktreeCreateWorkspaceId(null)}>
          <WorktreeCreateDialogBoundary
            workspaceId={model.worktreeCreateWorkspaceId}
            workspaceName={
              model.workspaces.find(workspace => workspace.id === model.worktreeCreateWorkspaceId)
                ?.name ?? "workspace"
            }
            listing={createListing.data}
            scopeId={scopeId}
            onOpenChange={open =>
              model.setWorktreeCreateWorkspaceId(open ? model.worktreeCreateWorkspaceId : null)
            }
          />
        </DeferredWorktreeDialog>
      ) : null}
      {worktreeDialogs.adoptTarget ? (
        <DeferredWorktreeDialog onClose={worktreeDialogs.closeAdopt}>
          <WorktreeAdoptDialogBoundary
            workspaceId={worktreeDialogs.adoptTarget.workspaceId}
            discovered={worktreeDialogs.adoptTarget.discovered}
            scopeId={scopeId}
            onClose={worktreeDialogs.closeAdopt}
          />
        </DeferredWorktreeDialog>
      ) : null}
      {worktreeDialogs.removeTarget ? (
        <DeferredWorktreeDialog onClose={worktreeDialogs.closeRemove}>
          <WorktreeRemoveDialogBoundary
            workspaceId={worktreeDialogs.removeTarget.workspaceId}
            worktree={worktreeDialogs.removeTarget.worktree}
            onClose={worktreeDialogs.closeRemove}
          />
        </DeferredWorktreeDialog>
      ) : null}
      {worktreeDialogs.missingTarget ? (
        <DeferredWorktreeDialog onClose={worktreeDialogs.closeMissing}>
          <WorktreeMissingDialogBoundary
            workspaceId={worktreeDialogs.missingTarget.workspaceId}
            worktree={worktreeDialogs.missingTarget.worktree}
            onClose={worktreeDialogs.closeMissing}
          />
        </DeferredWorktreeDialog>
      ) : null}
      {worktreeDialogs.contextTarget ? (
        <DeferredWorktreeDialog onClose={worktreeDialogs.closeContext}>
          <WorktreeContextDialogBoundary
            workspaceId={worktreeDialogs.contextTarget.workspaceId}
            worktree={worktreeDialogs.contextTarget.worktree}
            onCleanUp={worktreeDialogs.requestContextCleanup}
            onClose={worktreeDialogs.closeContext}
          />
        </DeferredWorktreeDialog>
      ) : null}
    </>
  );
}
