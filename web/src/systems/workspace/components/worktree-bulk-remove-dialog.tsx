import {
  Button,
  Checkbox,
  ConfirmDialog,
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  Field,
  FieldDescription,
  FieldLabel,
} from "@compozy/ui";
import { useWorktreeRemovalBatch } from "@/systems/workspace/hooks/use-worktree-removal-batch";
import type { WorktreeRemovalBatch } from "@/systems/workspace/hooks/use-worktree-removal-selection";

import { WorktreeRemovalRiskRows } from "./worktree-remove-risk-rows";
import {
  FORCE_DELETE_BRANCH_HINT,
  REMOVE_HISTORY_NOTE,
  REMOVE_BRANCH_HISTORY_NOTE,
} from "./worktree-remove-copy";

/** Presents the frozen removal/dismissal consequences and non-atomic per-item results. */
export function WorktreeBulkRemoveDialog({
  batch,
  onClose,
}: {
  batch: WorktreeRemovalBatch;
  onClose: () => void;
}) {
  const model = useWorktreeRemovalBatch(batch);
  const removed = model.batch.worktrees.filter(row => row.state === "ready").length;
  const dismissed = model.batch.worktrees.length - removed;
  const successes = model.results.filter(result => result.status === "success").length;
  const failures = model.results.filter(result => result.status === "failed").length;
  const done = successes === model.results.length;
  const owners = new Set(model.batch.worktrees.map(row => row.profile_id));
  if (model.forceTargets.length > 0) return <WorktreeBulkForceConfirmation model={model} />;
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
            {removed} {removed === 1 ? "checkout" : "checkouts"} will be removed through Git.{" "}
            {dismissed} missing {dismissed === 1 ? "record" : "records"} will be dismissed from the
            list; files at those paths are preserved. Sessions and task or Loop history stay
            readable. Each target is checked separately. A refusal does not undo other successes.
          </DialogDescription>
        </DialogHeader>
        <Field orientation="horizontal">
          <Checkbox
            id="worktree-bulk-delete-branch"
            aria-describedby="worktree-bulk-delete-branch-hint"
            checked={model.deleteBranch}
            disabled={model.attempted}
            data-testid="worktree-bulk-delete-branch"
            onCheckedChange={checked => model.setDeleteBranch(checked === true)}
          />
          <div className="grid gap-1">
            <FieldLabel htmlFor="worktree-bulk-delete-branch">
              Also delete local branches
            </FieldLabel>
            <FieldDescription id="worktree-bulk-delete-branch-hint">
              Without force, only merged branches are deleted. The main checkout’s branch and the
              default branch are always kept. Results are reported per worktree.
            </FieldDescription>
          </div>
        </Field>
        <div className="max-h-64 overflow-y-auto" aria-live="polite">
          {model.results.map(result => (
            <div
              key={result.row.id}
              className="border-b border-line py-2"
              data-testid={`worktree-bulk-result-${result.row.id}`}
              data-status={result.status}
            >
              <p className="truncate text-form-label font-medium">
                {result.row.name} ·{" "}
                {result.row.state === "missing" ? "Dismiss record" : "Remove checkout"}
                {owners.size > 1 ? ` · ${result.row.profile_name}` : null}
              </p>
              <p className="break-all text-form-hint text-subtle">{result.row.path}</p>
              {result.refusal?.risk ? (
                <WorktreeRemovalRiskRows
                  risk={result.refusal.risk}
                  worktreeName={result.row.name}
                  branch={result.row.branch}
                />
              ) : null}
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
        <DialogFooter>
          <Button variant="ghost" disabled={model.running} onClick={onClose}>
            {model.attempted ? "Close" : "Cancel"}
          </Button>
          {model.forceEligible.length > 0 ? (
            <Button
              variant="ghost"
              className="text-danger hover:bg-danger-tint"
              disabled={model.running}
              onClick={model.requestForce}
            >
              Force remove… ({model.forceEligible.length})
            </Button>
          ) : null}
          {!done ? (
            <Button
              variant="destructive"
              disabled={model.running}
              data-testid="worktree-bulk-remove-submit"
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

function WorktreeBulkForceConfirmation({
  model,
}: {
  model: ReturnType<typeof useWorktreeRemovalBatch>;
}) {
  return (
    <ConfirmDialog
      open
      onOpenChange={open => {
        if (!open && !model.running) model.cancelForce();
      }}
      tone="danger"
      title={`Force remove ${model.forceTargets.length} ${model.forceTargets.length === 1 ? "worktree" : "worktrees"}?`}
      description="These worktrees contain local work. Force removal discards uncommitted changes. Review each target before continuing."
      note={model.deleteBranch ? REMOVE_BRANCH_HISTORY_NOTE : REMOVE_HISTORY_NOTE}
      body={
        <div className="flex flex-col gap-4">
          {model.deleteBranch ? <p className="text-form-hint">{FORCE_DELETE_BRANCH_HINT}</p> : null}
          <div className="flex max-h-80 flex-col gap-4 overflow-y-auto">
            {model.forceTargets.map(result => (
              <div key={result.row.id}>
                <p className="text-form-label font-medium">{result.row.name}</p>
                <p className="break-all text-form-hint text-subtle">{result.row.path}</p>
                {result.refusal?.risk ? (
                  <WorktreeRemovalRiskRows
                    risk={result.refusal.risk}
                    worktreeName={result.row.name}
                    branch={result.row.branch}
                  />
                ) : null}
              </div>
            ))}
          </div>
        </div>
      }
      confirmLabel="Force remove"
      cancelLabel="Cancel"
      isPending={model.running}
      onConfirm={() => void model.confirmForce()}
      contentProps={{ "data-testid": "worktree-bulk-force-dialog" }}
    />
  );
}
