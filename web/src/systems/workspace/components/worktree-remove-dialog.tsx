import { Trash2, TriangleAlert } from "@compozy/ui/icons";

import {
  Alert,
  AlertDescription,
  Button,
  Checkbox,
  ConfirmDialog,
  Field,
  FieldDescription,
  FieldLabel,
  MetadataList,
  MonoId,
} from "@compozy/ui";

import type { WorktreeRefusal } from "../lib/worktree-refusal";
import type { WorktreePayload } from "../types";
import {
  boundIdleNote,
  DELETE_BRANCH_HINT,
  FORCE_DELETE_BRANCH_HINT,
  REMOVE_BRANCH_HISTORY_NOTE,
  REMOVE_DOWNGRADE_NOTE,
  REMOVE_HISTORY_NOTE,
  removeDialogDescription,
  removeDialogTitle,
  resolveRemoveStage,
} from "./worktree-remove-copy";
import { WorktreePath } from "./worktree-path";
import { WorktreeRemovalRiskRows } from "./worktree-remove-risk-rows";

export interface WorktreeRemoveDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  worktree: WorktreePayload;
  refusal: WorktreeRefusal | null;
  /** Set once the user has passed through the refusal's force doorway. */
  forceRequested: boolean;
  onRequestForce: () => void;
  boundSessions?: { running: number; idle: number };
  isRemoving: boolean;
  /** Opt-in: also delete the local branch once the checkout is removed. */
  deleteBranch: boolean;
  onDeleteBranchChange: (deleteBranch: boolean) => void;
  onConfirm: (force: boolean) => void;
  /** The alternative to destroying the work: the assisted exit flow. */
  onCommitAndPush?: () => void;
  onOpenSession?: () => void;
}

/**
 * Two-step removal (ADR-006). A refusal never becomes a confirm by itself: the
 * user must pass the explicit force doorway, and the force step re-states the
 * quantities so the loss is named twice before it happens.
 */
export function WorktreeRemoveDialog({
  open,
  onOpenChange,
  worktree,
  refusal,
  forceRequested,
  onRequestForce,
  boundSessions = { running: 0, idle: 0 },
  isRemoving,
  deleteBranch,
  onDeleteBranchChange,
  onConfirm,
  onCommitAndPush,
  onOpenSession,
}: WorktreeRemoveDialogProps) {
  const stage = resolveRemoveStage(refusal, forceRequested, boundSessions);
  const risk = refusal?.risk ?? null;
  const isRefusal = stage === "dirty-refusal" || stage === "orphaned" || stage === "bound-running";
  const branchOptIn =
    !isRefusal && worktree.branch !== "" ? (
      <Field orientation="horizontal">
        <Checkbox
          id="worktree-remove-delete-branch"
          aria-describedby="worktree-remove-delete-branch-hint"
          checked={deleteBranch}
          disabled={isRemoving}
          data-testid="worktree-remove-delete-branch"
          onCheckedChange={checked => onDeleteBranchChange(checked === true)}
        />
        <div className="grid gap-1">
          <FieldLabel htmlFor="worktree-remove-delete-branch">Also delete local branch</FieldLabel>
          <FieldDescription id="worktree-remove-delete-branch-hint">
            {stage === "force" ? FORCE_DELETE_BRANCH_HINT : DELETE_BRANCH_HINT}
          </FieldDescription>
        </div>
      </Field>
    ) : null;

  const facts = (
    <MetadataList data-testid="worktree-remove-facts">
      <MetadataList.Row label="Branch">
        <MonoId value={worktree.branch} preserveCase size="sm" />
      </MetadataList.Row>
      <MetadataList.Row label="Path">
        <WorktreePath className="w-full" path={worktree.path} />
      </MetadataList.Row>
    </MetadataList>
  );

  return (
    <ConfirmDialog
      open={open}
      onOpenChange={onOpenChange}
      tone={stage === "force" ? "danger" : isRefusal ? "warning" : "danger"}
      title={removeDialogTitle(stage, worktree.name)}
      description={removeDialogDescription(stage, worktree.branch, risk, deleteBranch)}
      note={
        stage === "downgrade"
          ? REMOVE_DOWNGRADE_NOTE
          : stage === "bound-idle"
            ? boundIdleNote(boundSessions.idle)
            : stage === "force"
              ? "This deletes work that was never pushed anywhere."
              : deleteBranch
                ? REMOVE_BRANCH_HISTORY_NOTE
                : REMOVE_HISTORY_NOTE
      }
      noteTone={stage === "downgrade" ? "info" : stage === "force" ? "warning" : "neutral"}
      body={
        <div className="flex flex-col gap-4">
          {risk && (stage === "dirty-refusal" || stage === "orphaned" || stage === "force") ? (
            <WorktreeRemovalRiskRows
              risk={risk}
              worktreeName={worktree.name}
              // A detached checkout has no branch; the risk copy still needs a subject.
              branch={worktree.branch || "This detached checkout"}
            />
          ) : null}
          {facts}
          {branchOptIn}
          {stage === "bound-running" ? (
            <Alert variant="warning" data-testid="worktree-remove-session-running">
              <AlertDescription>
                A session is running a turn on this worktree. Stop it, or try again after the turn
                ends. Session history stays readable either way.
              </AlertDescription>
            </Alert>
          ) : null}
          {stage === "dirty-refusal" ? (
            // Ghost-danger doorway to the second confirm — never the direct
            // action, and it sits after Cancel in DOM and tab order.
            <div className="flex items-center gap-2">
              <Button
                type="button"
                variant="ghost"
                size="sm"
                className="text-danger hover:bg-danger-tint"
                data-testid="worktree-remove-force-doorway"
                onClick={onRequestForce}
              >
                <TriangleAlert className="size-3.5" />
                Force remove…
              </Button>
            </div>
          ) : null}
        </div>
      }
      confirmLabel={
        stage === "bound-running"
          ? "Open session"
          : stage === "dirty-refusal"
            ? "Commit & push first"
            : stage === "orphaned"
              ? "Force remove…"
              : stage === "force"
                ? "Force remove"
                : "Remove worktree"
      }
      cancelLabel="Cancel"
      isPending={isRemoving}
      confirmIcon={stage === "force" || stage === "orphaned" ? TriangleAlert : Trash2}
      onConfirm={() => {
        if (stage === "bound-running") return onOpenSession?.();
        // The refusal's primary is the way out that keeps the work.
        if (stage === "dirty-refusal") return onCommitAndPush?.();
        // Nothing can be committed without Git metadata; force is the only way on,
        // and it still passes the second confirm that re-states the loss.
        if (stage === "orphaned") return onRequestForce();
        onConfirm(stage === "force");
      }}
      contentProps={{ "data-testid": "worktree-remove-dialog", "data-stage": stage }}
      confirmButtonProps={{
        "data-testid": "worktree-remove-submit",
        disabled:
          (stage === "bound-running" && !onOpenSession) ||
          (stage === "dirty-refusal" && !onCommitAndPush),
        title:
          stage === "dirty-refusal" && !onCommitAndPush
            ? "Commit and push is available from worktree details."
            : undefined,
      }}
      cancelButtonProps={{ "data-testid": "worktree-remove-cancel" }}
    />
  );
}
