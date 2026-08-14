import {
  Alert,
  AlertDescription,
  Button,
  Dialog,
  DialogContent,
  DialogTitle,
  dialogShellClass,
  Spinner,
} from "@compozy/ui";

import type { WorktreeDetailModel } from "../hooks/use-worktree-detail-context";
import { WorktreeCommitDialog } from "./worktree-commit-dialog";
import { WorktreeDetailHeader } from "./worktree-detail-header";
import { WorktreeExitControl } from "./worktree-exit-control";
import { WorktreeMergedEvidence } from "./worktree-merged-evidence";
import { WorktreePrDialog } from "./worktree-pr-dialog";
import { WorktreeStatusStrip } from "./worktree-status-strip";

interface WorktreeDetailDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  model: WorktreeDetailModel;
}

/**
 * The worktree context: what this checkout is, what git says about it, and the
 * one assisted way out of it.
 *
 * The exit control lives here and only here — the session header carries the
 * binding chip alone, because a session row already carries enough density.
 */
export function WorktreeDetailDialog({ open, onOpenChange, model }: WorktreeDetailDialogProps) {
  return (
    <Dialog onOpenChange={onOpenChange} open={open}>
      <DialogContent className={dialogShellClass("sm")} data-slot="worktree-detail-dialog" unframed>
        <DialogTitle className="sr-only">
          {model.worktree ? `${model.worktree.name} worktree details` : "Worktree details"}
        </DialogTitle>
        {model.state === "ready" && model.worktree ? (
          <div className="flex flex-col gap-4 px-5 py-4">
            <WorktreeDetailHeader
              onRemove={model.onRemove}
              sessionTitle={model.sessionTitle}
              worktree={model.worktree}
            />
            {model.status ? (
              <WorktreeStatusStrip
                forge={model.ladder?.forge}
                forgeStatus={model.forgeStatus}
                staleLabel={model.staleLabel}
                status={model.status}
              />
            ) : null}
            {model.ladder ? (
              <WorktreeExitControl
                isRunning={model.isRunning}
                ladder={model.ladder}
                onAction={model.onAction}
              />
            ) : null}
            {model.ladder ? (
              <WorktreeMergedEvidence
                cleanup={model.ladder.cleanup}
                onCleanUp={model.onCleanUp}
                staleLabel={model.staleLabel}
              />
            ) : null}
          </div>
        ) : model.state === "loading" ? (
          <div className="flex items-center justify-center px-5 py-8">
            <Spinner className="size-4" />
          </div>
        ) : model.state === "error" ? (
          <div className="flex flex-col gap-3 px-5 py-4">
            <Alert variant="danger">
              <AlertDescription>
                {model.error || "Worktree details could not be loaded."}
              </AlertDescription>
            </Alert>
            {model.retry ? (
              <Button onClick={model.retry} size="sm" variant="outline">
                Retry
              </Button>
            ) : null}
          </div>
        ) : (
          <div className="px-5 py-4 text-small-body text-subtle">
            This worktree is no longer available.
          </div>
        )}
      </DialogContent>
      {model.commitDialog ? <WorktreeCommitDialog {...model.commitDialog} /> : null}
      {model.prDialog ? <WorktreePrDialog {...model.prDialog} /> : null}
    </Dialog>
  );
}
