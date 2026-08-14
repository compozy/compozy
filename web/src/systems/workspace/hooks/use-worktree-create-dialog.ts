import { useState } from "react";

import {
  buildWorktreeCreatePreview,
  deriveWorktreeParentDir,
  type WorktreeCreatePreview,
} from "../lib/worktree-naming";
import {
  decodeWorktreeRefusal,
  refusalBranchHolder,
  WORKTREE_ERROR_CODES,
  type WorktreeRefusal,
} from "../lib/worktree-refusal";
import type { WorktreePayload, WorktreesResponse } from "../types";
import { useCancelWorktreeCreate, useCreateWorktree } from "./use-worktrees";

export interface WorktreeCreateDraft {
  name: string;
  branch: string;
  baseRef: string;
  existingBranch: string;
}

const EMPTY_DRAFT: WorktreeCreateDraft = { name: "", branch: "", baseRef: "", existingBranch: "" };

/** Which draft fields clear which refusal when edited. */
const REFUSAL_FIELD_KEYS: Record<
  Exclude<WorktreeCreateFieldError, null>,
  ReadonlyArray<keyof WorktreeCreateDraft>
> = {
  name: ["name"],
  branch: ["branch", "existingBranch"],
  baseRef: ["baseRef"],
};

/** Which field a refusal belongs to, so the reason renders on the failing input. */
export type WorktreeCreateFieldError = "name" | "branch" | "baseRef" | null;

export interface BranchCandidate {
  branch: string;
  /** Name of the worktree already holding it; `null` when free. */
  heldBy: string | null;
}

export interface WorktreeCreateDialogModel {
  draft: WorktreeCreateDraft;
  setDraft: (next: Partial<WorktreeCreateDraft>) => void;
  advancedOpen: boolean;
  setAdvancedOpen: (open: boolean) => void;
  /** Generated suggestion, shown as a placeholder only — never prefilled. */
  generatedName: string;
  preview: WorktreeCreatePreview | null;
  branchCandidates: BranchCandidate[];
  refusal: WorktreeRefusal | null;
  fieldError: WorktreeCreateFieldError;
  /** Worktree to jump to when a branch is already checked out elsewhere. */
  heldByWorktree: WorktreePayload | null;
  isSubmitting: boolean;
  canSubmit: boolean;
  submit: () => void;
  reset: () => void;
  /**
   * The durable pending row returned by the 202. Present from acceptance until
   * the row goes ready, which is the window in which the creation can still be
   * cancelled daemon-side.
   */
  pendingWorktree: WorktreePayload | null;
  /**
   * Cancels the accepted creation and unwinds its artifacts. Only callable once
   * a worktree id exists — before that there is nothing durable to cancel, and
   * aborting the in-flight POST would not stop a create the daemon already took.
   */
  cancelCreate: () => void;
  isCancelling: boolean;
  cancelError: string | null;
}

function fieldForRefusal(refusal: WorktreeRefusal | null): WorktreeCreateFieldError {
  switch (refusal?.code) {
    case WORKTREE_ERROR_CODES.nameTaken:
    case WORKTREE_ERROR_CODES.pathExists:
      return "name";
    case WORKTREE_ERROR_CODES.branchHeld:
    case WORKTREE_ERROR_CODES.branchAtRoot:
      return "branch";
    case WORKTREE_ERROR_CODES.baseRefNotFound:
      return "baseRef";
    default:
      return null;
  }
}

export function useWorktreeCreateDialog(
  workspaceId: string,
  listing: WorktreesResponse | undefined,
  options: { generatedName: string; onCreated?: (worktree: WorktreePayload) => void }
): WorktreeCreateDialogModel {
  const [draft, setDraftState] = useState<WorktreeCreateDraft>(EMPTY_DRAFT);
  const [advancedOpen, setAdvancedOpen] = useState(false);
  const [pendingWorktree, setPendingWorktree] = useState<WorktreePayload | null>(null);
  const createMutation = useCreateWorktree(workspaceId);
  const cancelMutation = useCancelWorktreeCreate(workspaceId);

  const worktrees = listing?.worktrees ?? [];
  const parentDir = deriveWorktreeParentDir(worktrees);

  const heldBranches = new Map<string, string>();
  for (const worktree of worktrees) heldBranches.set(worktree.branch, worktree.name);
  const branchCandidates: BranchCandidate[] = [];
  for (const [branch, name] of heldBranches) branchCandidates.push({ branch, heldBy: name });

  const refusal = decodeWorktreeRefusal(createMutation.error);
  const holderName = refusalBranchHolder(refusal);
  const heldByWorktree =
    worktrees.find(
      worktree =>
        worktree.path === holderName ||
        worktree.name === holderName ||
        worktree.branch === holderName
    ) ?? null;

  // The effective name drives the preview: an empty field previews the
  // generated suggestion, matching what submitting would actually create.
  const effectiveName = draft.name.trim() === "" ? options.generatedName : draft.name;
  const preview = buildWorktreeCreatePreview(effectiveName, draft.branch, parentDir);

  const fieldError = fieldForRefusal(refusal);

  const setDraft = (next: Partial<WorktreeCreateDraft>) => {
    // Editing the field a refusal points at clears it, so the primary unblocks
    // as soon as the user addresses the reason instead of staying dead.
    if (fieldError && REFUSAL_FIELD_KEYS[fieldError].some(key => key in next)) {
      createMutation.reset();
    }
    setDraftState(current => ({ ...current, ...next }));
  };

  const reset = () => {
    setDraftState(EMPTY_DRAFT);
    setAdvancedOpen(false);
    setPendingWorktree(null);
    createMutation.reset();
    cancelMutation.reset();
  };

  // A pending row that reached `ready` (or left the list) is past the point of
  // cancellation, so the affordance retires itself rather than lingering.
  const livePending =
    pendingWorktree === null
      ? null
      : (worktrees.find(worktree => worktree.id === pendingWorktree.id) ?? pendingWorktree);
  const cancellablePending = livePending?.state === "pending" ? livePending : null;

  return {
    draft,
    setDraft,
    advancedOpen,
    setAdvancedOpen,
    generatedName: options.generatedName,
    preview,
    branchCandidates,
    refusal,
    fieldError,
    heldByWorktree,
    isSubmitting: createMutation.isPending,
    // A standing refusal blocks the primary until the offending field changes.
    canSubmit: !createMutation.isPending && refusal === null,
    submit: () => {
      createMutation.mutate(
        {
          name: effectiveName,
          ...(draft.branch.trim() ? { branch: draft.branch.trim() } : {}),
          ...(draft.baseRef.trim() ? { base_ref: draft.baseRef.trim() } : {}),
          ...(draft.existingBranch.trim() ? { existing_branch: draft.existingBranch.trim() } : {}),
        },
        {
          onSuccess: worktree => {
            // 202: the row is durable and materializing. Hold it so Cancel can
            // reach the daemon-side creation by id.
            setPendingWorktree(worktree);
            options.onCreated?.(worktree);
          },
        }
      );
    },
    reset,
    pendingWorktree: cancellablePending,
    cancelCreate: () => {
      if (!cancellablePending) return;
      cancelMutation.mutate(cancellablePending.id, {
        onSuccess: () => setPendingWorktree(null),
      });
    },
    isCancelling: cancelMutation.isPending,
    cancelError: cancelMutation.error instanceof Error ? cancelMutation.error.message : null,
  };
}
