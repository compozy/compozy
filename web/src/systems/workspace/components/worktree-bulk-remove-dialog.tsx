import { useProfileReadScope } from "@/systems/profiles";
import {
  Button,
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@compozy/ui";
import { useWorktreeRemovalBatch } from "@/systems/workspace/hooks/use-worktree-removal-batch";
import type { WorktreeRemovalBatch } from "@/systems/workspace/hooks/use-worktree-removal-selection";

export function WorktreeBulkRemoveDialog({
  batch,
  onClose,
}: {
  batch: WorktreeRemovalBatch;
  onClose: () => void;
}) {
  const model = useWorktreeRemovalBatch(batch, useProfileReadScope().destinationOwner);
  const removed = model.batch.worktrees.filter(row => row.state === "ready").length;
  const dismissed = model.batch.worktrees.length - removed;
  const successes = model.results.filter(result => result.status === "success").length;
  const failures = model.results.filter(result => result.status === "failed").length;
  const done = successes === model.results.length;
  return (
    <Dialog
      open
      onOpenChange={open => {
        if (!open && !model.running) onClose();
      }}
    >
      <DialogContent showCloseButton={false} data-testid="worktree-bulk-remove-dialog">
        <DialogHeader>
          <DialogTitle>
            Remove {model.batch.worktrees.length}{" "}
            {model.batch.worktrees.length === 1 ? "worktree" : "worktrees"}?
          </DialogTitle>
          <DialogDescription>
            {removed} checkouts will be removed through Git. Eligible managed branches may be
            reclaimed. {dismissed} missing records will be dismissed from the list only; files at
            those paths, branches, Git history, sessions and task or Loop history are preserved.
            Each target is checked separately. A refusal does not undo other successes.
          </DialogDescription>
        </DialogHeader>
        <div className="max-h-64 overflow-y-auto" aria-live="polite">
          {model.results.map(result => (
            <div key={result.row.id} className="border-b border-line py-2">
              <p className="truncate text-form-label font-medium">
                {result.row.name} ·{" "}
                {result.row.state === "missing" ? "Dismiss record" : "Remove checkout"}
              </p>
              <p className="break-all text-form-hint text-subtle">{result.row.path}</p>
              {result.message ? (
                <p
                  role={result.status === "failed" ? "alert" : undefined}
                  className="text-form-hint"
                >
                  {result.message}
                </p>
              ) : null}
            </div>
          ))}
        </div>
        {model.attempted ? (
          <p role="status">
            {successes} succeeded · {failures} failed ·{" "}
            {model.results.length - successes - failures} pending
          </p>
        ) : null}
        {!model.scopeMatches ? (
          <p role="alert">
            The active profile changed. Return to {model.batch.profile.name} to continue, or close
            and select again.
          </p>
        ) : null}
        <DialogFooter>
          <Button variant="ghost" disabled={model.running} onClick={onClose}>
            {model.attempted ? "Close" : "Cancel"}
          </Button>
          {!done ? (
            <Button
              variant="destructive"
              disabled={model.running || !model.scopeMatches}
              onClick={() => void model.run()}
            >
              {model.running
                ? "Removing…"
                : model.attempted
                  ? `Retry failed (${failures})`
                  : "Remove selected"}
            </Button>
          ) : null}
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
