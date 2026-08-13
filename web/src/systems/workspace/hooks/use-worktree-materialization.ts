import { useSelector, useStore } from "@xstate/store-react";
import { useEffect } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";

import { notifyUser } from "@/lib/user-feedback";

import type { WorktreePayload } from "../types";
import { workspaceKeys } from "../lib/query-keys";
import { worktreeMaterializationFailureOptions } from "../lib/query-options";
import { useCancelWorktreeCreate, useCreateWorktree, useWorktrees } from "./use-worktrees";
import {
  worktreeMaterializationLogic,
  type WorktreeMaterializationRequestState,
} from "./worktree-materialization-store";

/**
 * Observable materialization state. `pending` carries the daemon's own phase, so
 * the strip advances only when the worktree really advances — never on a timer.
 */
export type WorktreeMaterializationStatus =
  | "idle"
  | "creating"
  | "pending"
  | "ready"
  | "failed"
  | "canceled";

export interface WorktreeMaterialization {
  status: WorktreeMaterializationStatus;
  /** Daemon pending phase (`branch` · `checkout` · `copy` · `setup`), absent once ready. */
  phase?: string;
  worktree?: WorktreePayload;
  /** Create refusal, or the setup error the row reports. */
  error?: string;
  start: (name: string) => void;
  cancel: () => void;
  recover: () => void;
  retry: () => void;
  reset: () => void;
}

interface DerivedState {
  status: WorktreeMaterializationStatus;
  phase?: string;
  error?: string;
}

/**
 * Projects the request state plus the observed row into one status.
 *
 * The row is the authority once a create is accepted: the 202 only promises a
 * pending record, and the catalog stream carries every phase change from there.
 */
function deriveStatus(
  request: WorktreeMaterializationRequestState,
  worktree: WorktreePayload | undefined
): DerivedState {
  if (request.status === "idle") return { status: "idle" };
  if (request.status === "creating") return { status: "creating" };
  if (request.status === "canceled") return { status: "canceled" };
  if (request.status === "failed") return { status: "failed", error: request.error };
  if (!worktree) {
    // Accepted, but the row is not in the list yet — still creating, not missing.
    return { status: "creating" };
  }
  if (worktree.state === "pending") {
    return { status: "pending", phase: worktree.pending_phase };
  }
  if (worktree.state === "ready") return { status: "ready" };
  return { status: "failed", error: worktree.setup_error };
}

/**
 * Creates one worktree and reports its materialization truthfully.
 *
 * Nothing here simulates progress: the create call returns a pending row, and
 * every subsequent phase comes from the worktree list, which the catalog stream
 * keeps fresh. A failure keeps the request so the caller can retry, pick another
 * target, or fall back to the workspace root without losing what was typed.
 */
export function useWorktreeMaterialization(workspaceId: string): WorktreeMaterialization {
  const store = useStore(worktreeMaterializationLogic);
  const queryClient = useQueryClient();
  const request = useSelector(store, snapshot => snapshot.context);
  const createWorktree = useCreateWorktree(workspaceId);
  const cancelCreate = useCancelWorktreeCreate(workspaceId);
  const worktrees = useWorktrees(workspaceId, { enabled: request.status === "active" });

  const worktree = request.worktreeId
    ? worktrees.data?.worktrees.find(row => row.id === request.worktreeId)
    : undefined;
  const failure = useQuery(
    worktreeMaterializationFailureOptions(workspaceId, request.worktreeId ?? "")
  );
  const derived = deriveStatus(request, worktree);

  useEffect(() => {
    if (request.status !== "active" || !request.worktreeId) {
      return;
    }
    const observedError = failure.data?.trim();
    if (!observedError && (worktree || !worktrees.isFetched || worktrees.isFetching)) return;
    store.trigger.failureObserved({
      worktreeId: request.worktreeId,
      error: observedError || "Worktree creation failed and was rolled back.",
    });
  }, [
    failure.data,
    request.status,
    request.worktreeId,
    store,
    worktree,
    worktrees.isFetched,
    worktrees.isFetching,
  ]);

  const reset = () => {
    if (request.worktreeId) {
      queryClient.removeQueries({
        queryKey: workspaceKeys.worktreeMaterializationFailure(workspaceId, request.worktreeId),
        exact: true,
      });
    }
    store.trigger.reset({});
  };
  const cancelWorktree = async (worktreeId: string) => {
    try {
      await cancelCreate.mutateAsync(worktreeId);
    } catch (cause) {
      notifyUser({
        message:
          cause instanceof Error && cause.message.trim() !== ""
            ? cause.message
            : "Failed to cancel worktree creation.",
        tone: "error",
      });
    }
  };

  return {
    ...derived,
    worktree,
    start: (name: string) =>
      store.trigger.startRequested({
        name,
        execute: value => createWorktree.mutateAsync(value.trim() ? { name: value } : {}),
        cancel: cancelWorktree,
      }),
    cancel: () => store.trigger.cancelRequested({ execute: cancelWorktree }),
    recover: reset,
    retry: () =>
      store.trigger.retryRequested({
        execute: value => createWorktree.mutateAsync(value.trim() ? { name: value } : {}),
        cancel: cancelWorktree,
      }),
    reset,
  };
}
