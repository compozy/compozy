import { useEffect, useRef, useState } from "react";
import { hashKey, useQuery, useQueryClient } from "@tanstack/react-query";

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
import { workspaceKeys } from "../lib/query-keys";
import { worktreeMaterializationFailureOptions } from "../lib/query-options";
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
  materializationError: string | null;
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
  const [pendingWorktree, setPendingWorktree] = useState<WorktreePayload | null>(
    () => listing?.worktrees.find(worktree => worktree.state === "pending") ?? null
  );
  const [acceptedCatalogSeen, setAcceptedCatalogSeen] = useState(() =>
    Boolean(listing?.worktrees.some(worktree => worktree.state === "pending"))
  );
  const completedWorktreeID = useRef<string | null>(null);
  const completionSubscription = useRef<(() => void) | null>(null);
  const queryClient = useQueryClient();
  const createMutation = useCreateWorktree(workspaceId);
  const cancelMutation = useCancelWorktreeCreate(workspaceId);
  const worktrees = listing?.worktrees ?? [];
  const catalogPending = worktrees.find(worktree => worktree.state === "pending") ?? null;
  const trackedPending = pendingWorktree ?? catalogPending;
  const creationFailure = useQuery(
    worktreeMaterializationFailureOptions(workspaceId, trackedPending?.id ?? "")
  );

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

  const stopCompletionSubscription = () => {
    completionSubscription.current?.();
    completionSubscription.current = null;
  };

  const setDraft = (next: Partial<WorktreeCreateDraft>) => {
    // Editing the field a refusal points at clears it, so the primary unblocks
    // as soon as the user addresses the reason instead of staying dead.
    if (fieldError && REFUSAL_FIELD_KEYS[fieldError].some(key => key in next)) {
      createMutation.reset();
    }
    setDraftState(current => ({ ...current, ...next }));
  };

  const reset = () => {
    stopCompletionSubscription();
    setDraftState(EMPTY_DRAFT);
    setAdvancedOpen(false);
    setPendingWorktree(null);
    setAcceptedCatalogSeen(false);
    completedWorktreeID.current = null;
    createMutation.reset();
    cancelMutation.reset();
  };

  useEffect(() => {
    return () => {
      completionSubscription.current?.();
      completionSubscription.current = null;
    };
  }, []);

  useEffect(() => {
    if (pendingWorktree || !catalogPending) return;
    const worktreesQueryHash = hashKey(workspaceKeys.worktrees(workspaceId));
    const finishFromCatalog = () => {
      const catalog = queryClient.getQueryData<WorktreesResponse>(
        workspaceKeys.worktrees(workspaceId)
      );
      const authoritative = catalog?.worktrees.find(
        candidate => candidate.id === catalogPending.id
      );
      if (authoritative?.state !== "ready" || completedWorktreeID.current === authoritative.id) {
        return;
      }
      completedWorktreeID.current = authoritative.id;
      options.onCreated?.(authoritative);
    };
    const unsubscribe = queryClient.getQueryCache().subscribe(event => {
      if (hashKey(event.query.queryKey) === worktreesQueryHash) finishFromCatalog();
    });
    finishFromCatalog();
    return unsubscribe;
  }, [catalogPending, options.onCreated, pendingWorktree, queryClient, workspaceId]);

  const listedAccepted =
    trackedPending === null
      ? null
      : (worktrees.find(worktree => worktree.id === trackedPending.id) ?? null);
  const catalogFailure =
    listedAccepted && listedAccepted.state !== "pending" && listedAccepted.state !== "ready"
      ? listedAccepted.setup_error?.trim() || "Worktree creation failed and was rolled back."
      : null;
  const acceptedRowVanished =
    trackedPending !== null && listing !== undefined && acceptedCatalogSeen && !listedAccepted;
  const materializationError =
    creationFailure.data?.trim() ||
    catalogFailure ||
    (acceptedRowVanished ? "Worktree creation failed and was rolled back." : null);

  // Keep cancellation live until the catalog reaches a terminal outcome. The
  // accepted payload covers the short interval before the first stream refresh.
  const livePending = materializationError ? null : (listedAccepted ?? trackedPending);
  const cancellablePending = livePending?.state === "pending" ? livePending : null;
  const materializing =
    trackedPending !== null && materializationError === null && listedAccepted?.state !== "ready";

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
    isSubmitting: createMutation.isPending || materializing,
    // A standing refusal blocks the primary until the offending field changes.
    canSubmit: !createMutation.isPending && !materializing && refusal === null,
    submit: () => {
      if (createMutation.isPending || materializing) return;
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
            completedWorktreeID.current = null;
            setAcceptedCatalogSeen(false);
            setPendingWorktree(worktree);
            stopCompletionSubscription();
            let catalogSeen = false;
            const finishFromCatalog = () => {
              const failure = queryClient.getQueryData<string>(
                workspaceKeys.worktreeMaterializationFailure(workspaceId, worktree.id)
              );
              if (failure?.trim()) {
                stopCompletionSubscription();
                return;
              }
              const catalog = queryClient.getQueryData<WorktreesResponse>(
                workspaceKeys.worktrees(workspaceId)
              );
              const authoritative = catalog?.worktrees.find(
                candidate => candidate.id === worktree.id
              );
              if (authoritative) {
                catalogSeen = true;
                setAcceptedCatalogSeen(true);
              }
              if (catalog && catalogSeen && !authoritative) {
                stopCompletionSubscription();
                return;
              }
              if (
                authoritative &&
                authoritative.state !== "pending" &&
                authoritative.state !== "ready"
              ) {
                stopCompletionSubscription();
                return;
              }
              if (
                authoritative?.state !== "ready" ||
                completedWorktreeID.current === authoritative.id
              ) {
                return;
              }
              completedWorktreeID.current = authoritative.id;
              stopCompletionSubscription();
              options.onCreated?.(authoritative);
            };
            const worktreesQueryHash = hashKey(workspaceKeys.worktrees(workspaceId));
            completionSubscription.current = queryClient.getQueryCache().subscribe(event => {
              if (hashKey(event.query.queryKey) === worktreesQueryHash) finishFromCatalog();
            });
            finishFromCatalog();
          },
        }
      );
    },
    reset,
    pendingWorktree: cancellablePending,
    cancelCreate: () => {
      if (!cancellablePending) return;
      cancelMutation.mutate(cancellablePending.id, {
        onSuccess: () => {
          stopCompletionSubscription();
          setPendingWorktree(null);
          setAcceptedCatalogSeen(false);
        },
      });
    },
    isCancelling: cancelMutation.isPending,
    cancelError: cancelMutation.error instanceof Error ? cancelMutation.error.message : null,
    materializationError,
  };
}
