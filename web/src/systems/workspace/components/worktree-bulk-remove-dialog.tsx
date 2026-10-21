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
import {
  useWorktreeRemovalBatch,
  type WorktreeRemovalTarget,
} from "@/systems/workspace/hooks/use-worktree-removal-batch";
import type { WorktreeRemovalBatch } from "@/systems/workspace/hooks/use-worktree-removal-selection";
import { isStaleDiscovered } from "@/systems/workspace/lib/worktree-display";

import { WorktreeRemovalRiskRows } from "./worktree-remove-risk-rows";
import {
  FORCE_DELETE_BRANCH_HINT,
  REMOVE_HISTORY_NOTE,
  REMOVE_BRANCH_HISTORY_NOTE,
} from "./worktree-remove-copy";

/** Name, path, branch and the action one frozen target will take. */
function describeTarget(target: WorktreeRemovalTarget) {
  if (target.kind === "worktree") {
    const { row } = target;
    return {
      name: row.name,
      path: row.path,
      branch: row.branch,
      action: row.state === "missing" ? "Dismiss record" : "Remove checkout",
      owner: row.profile_name,
    };
  }
  const { discovered } = target;
  return {
    name: discovered.name,
    path: discovered.path,
    branch: discovered.branch,
    action: isStaleDiscovered(discovered) ? "Clear stale Git entry" : "Remove discovered checkout",
    owner: null,
  };
}

function plural(count: number, one: string, many: string) {
  return `${count} ${count === 1 ? one : many}`;
}

/** Names only the consequences present in this batch; discovered rows keep their branch. */
function batchConsequences(batch: WorktreeRemovalBatch) {
  const live = (batch.discovered ?? []).filter(discovered => !isStaleDiscovered(discovered));
  const stale = (batch.discovered ?? []).length - live.length;
  const removed = batch.worktrees.filter(row => row.state === "ready").length + live.length;
  const dismissed = batch.worktrees.filter(row => row.state !== "ready").length;
  const parts = [];
  if (removed > 0) {
    parts.push(`${plural(removed, "checkout", "checkouts")} will be removed through Git.`);
  }
  if (dismissed > 0) {
    parts.push(
      `${plural(dismissed, "missing record", "missing records")} will be dismissed from the list; files at those paths are preserved.`
    );
  }
  if (stale > 0) {
    parts.push(
      `${plural(stale, "stale Git entry", "stale Git entries")} will be cleared; nothing is left on disk.`
    );
  }
  return parts.join(" ");
}

/** Presents the frozen removal/dismissal consequences and non-atomic per-item results. */
export function WorktreeBulkRemoveDialog({
  batch,
  onClose,
}: {
  batch: WorktreeRemovalBatch;
  onClose: () => void;
}) {
  const model = useWorktreeRemovalBatch(batch);
  const total = model.results.length;
  const hasRecords = model.batch.worktrees.length > 0;
  const hasDiscovered = (model.batch.discovered ?? []).length > 0;
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
          <DialogTitle>Remove {plural(total, "worktree", "worktrees")}?</DialogTitle>
          <DialogDescription>
            {batchConsequences(model.batch)} Sessions and task or Loop history stay readable. Each
            target is checked separately. A refusal does not undo other successes.
          </DialogDescription>
        </DialogHeader>
        {hasRecords ? (
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
                {hasDiscovered ? " Discovered worktrees always keep their branch." : null}
              </FieldDescription>
            </div>
          </Field>
        ) : null}
        <div className="max-h-64 overflow-y-auto" aria-live="polite">
          {model.results.map(result => {
            const target = describeTarget(result.target);
            return (
              <div
                key={result.target.key}
                className="border-b border-line py-2"
                data-testid={`worktree-bulk-result-${result.target.key}`}
                data-status={result.status}
              >
                <p className="truncate text-form-label font-medium">
                  {target.name} · {target.action}
                  {owners.size > 1 && target.owner ? ` · ${target.owner}` : null}
                </p>
                <p className="break-all text-form-hint text-subtle">{target.path}</p>
                {result.refusal?.risk ? (
                  <WorktreeRemovalRiskRows
                    risk={result.refusal.risk}
                    worktreeName={target.name}
                    branch={target.branch}
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
            );
          })}
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
            {model.forceTargets.map(result => {
              const target = describeTarget(result.target);
              return (
                <div key={result.target.key}>
                  <p className="text-form-label font-medium">{target.name}</p>
                  <p className="break-all text-form-hint text-subtle">{target.path}</p>
                  {result.refusal?.risk ? (
                    <WorktreeRemovalRiskRows
                      risk={result.refusal.risk}
                      worktreeName={target.name}
                      branch={target.branch}
                    />
                  ) : null}
                </div>
              );
            })}
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
