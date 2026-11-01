import { Trash2, TriangleAlert } from "@compozy/ui/icons";

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
import { isStaleDiscovered } from "@/systems/workspace/lib/worktree-display";

import { WorktreeRemovalTargetList } from "./worktree-removal-target-list";
import {
  FORCE_DELETE_BRANCH_HINT,
  REMOVE_BRANCH_HISTORY_NOTE,
  REMOVE_HISTORY_NOTE,
} from "./worktree-remove-copy";

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
      `${plural(dismissed, "missing record", "missing records")} will be dismissed; files at those paths are kept.`
    );
  }
  if (stale > 0) {
    parts.push(
      `${plural(stale, "stale Git entry", "stale Git entries")} will be cleared from Git’s worktree list.`
    );
  }
  return parts.join(" ");
}

type BatchModel = ReturnType<typeof useWorktreeRemovalBatch>;

/** Running, partial and final tallies in one sentence; zero counts never pad it. */
function batchProgress(model: BatchModel, successes: number, failures: number) {
  const total = model.results.length;
  if (model.running) return `Removing… ${successes} of ${total} done so far`;
  return failures > 0
    ? `${successes} of ${total} done · ${failures} failed`
    : `${successes} of ${total} done`;
}

/** The title names the outcome in words; the description below carries the tally. */
function batchTitle(model: BatchModel, successes: number) {
  const total = model.results.length;
  const noun = total === 1 ? "worktree" : "worktrees";
  if (!model.attempted) return `Remove ${plural(total, "worktree", "worktrees")}?`;
  if (model.running) return `Removing ${noun}…`;
  if (successes === total) return total === 1 ? "Worktree cleaned up" : "Worktrees cleaned up";
  return total === 1 ? "Worktree was not removed" : "Some worktrees were not removed";
}

/** The opt-in is part of the confirmed consequences, so it only shows before the run. */
function BranchOptIn({ model }: { model: BatchModel }) {
  if (model.batch.worktrees.length === 0 || model.attempted) return null;
  const hasDiscovered = (model.batch.discovered ?? []).length > 0;
  return (
    <Field orientation="horizontal">
      <Checkbox
        id="worktree-bulk-delete-branch"
        aria-describedby="worktree-bulk-delete-branch-hint"
        checked={model.deleteBranch}
        data-testid="worktree-bulk-delete-branch"
        onCheckedChange={checked => model.setDeleteBranch(checked === true)}
      />
      <div className="grid gap-0.5">
        <FieldLabel htmlFor="worktree-bulk-delete-branch">Also delete local branches</FieldLabel>
        <FieldDescription id="worktree-bulk-delete-branch-hint">
          Only merged branches are deleted; the default branch is always kept.
          {hasDiscovered ? " Discovered worktrees keep theirs." : null}
        </FieldDescription>
      </div>
    </Field>
  );
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
  const successes = model.results.filter(result => result.status === "success").length;
  const failures = model.results.filter(result => result.status === "failed").length;
  const done = successes === total;
  const forceCount = model.forceEligible.length;
  if (model.forceTargets.length > 0) return <WorktreeBulkForceConfirmation model={model} />;
  return (
    <Dialog
      open
      onOpenChange={open => {
        if (!open && !model.running) onClose();
      }}
    >
      <DialogContent
        showCloseButton={false}
        unframed
        className="sm:max-w-md"
        data-testid="worktree-bulk-remove-dialog"
      >
        <DialogHeader variant="ruled">
          <DialogTitle>{batchTitle(model, successes)}</DialogTitle>
          {/* Before the run the description states the consequences; once it
              starts, the same line becomes the live tally. */}
          <DialogDescription
            role={model.attempted ? "status" : undefined}
            data-testid="worktree-bulk-progress"
            className="tabular-nums"
          >
            {model.attempted
              ? `${batchProgress(model, successes, failures)}. Session and run history stay readable.`
              : `${batchConsequences(model.batch)} Session and run history stay readable.`}
          </DialogDescription>
        </DialogHeader>
        <div className="flex min-w-0 flex-col gap-4 px-5 pt-4 pb-5">
          <WorktreeRemovalTargetList results={model.results} />
          <BranchOptIn model={model} />
        </div>
        <DialogFooter variant="ruled" className="sm:items-center">
          <div className="flex shrink-0 items-center justify-end gap-2">
            <Button variant="ghost" disabled={model.running} onClick={onClose}>
              {model.attempted ? "Close" : "Cancel"}
            </Button>
            {!done && model.attempted ? (
              <Button
                variant={forceCount > 0 ? "ghost" : "outline"}
                disabled={model.running}
                data-testid="worktree-bulk-remove-submit"
                onClick={() => void model.run()}
              >
                {model.running ? "Removing…" : "Retry failed"}
              </Button>
            ) : null}
            {!model.attempted ? (
              <Button
                variant="destructive"
                data-testid="worktree-bulk-remove-submit"
                onClick={() => void model.run()}
              >
                <Trash2 />
                Remove {plural(total, "worktree", "worktrees")}
              </Button>
            ) : null}
            {forceCount > 0 ? (
              <Button variant="destructive" disabled={model.running} onClick={model.requestForce}>
                <TriangleAlert />
                Force remove…
              </Button>
            ) : null}
          </div>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

function WorktreeBulkForceConfirmation({ model }: { model: BatchModel }) {
  const count = model.forceTargets.length;
  return (
    <ConfirmDialog
      open
      onOpenChange={open => {
        if (!open && !model.running) model.cancelForce();
      }}
      tone="destructive"
      title={`Force remove ${plural(count, "worktree", "worktrees")}?`}
      description={`${count === 1 ? "It holds" : "They hold"} work that exists nowhere else. Force removal deletes it permanently. ${model.deleteBranch ? REMOVE_BRANCH_HISTORY_NOTE : REMOVE_HISTORY_NOTE}`}
      body={
        <>
          <WorktreeRemovalTargetList results={model.forceTargets} detailedRisk />
          {model.deleteBranch ? (
            <p className="text-form-hint text-subtle">{FORCE_DELETE_BRANCH_HINT}</p>
          ) : null}
        </>
      }
      confirmLabel="Force remove"
      confirmIcon={TriangleAlert}
      cancelLabel="Cancel"
      isPending={model.running}
      onConfirm={() => void model.confirmForce()}
      contentProps={{ "data-testid": "worktree-bulk-force-dialog" }}
    />
  );
}
