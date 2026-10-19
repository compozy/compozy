import { createStoreLogic } from "@xstate/store";
import { useSelector, useStore } from "@xstate/store-react";
import { useQueryClient } from "@tanstack/react-query";
import { dismissWorktree, removeWorktree } from "@/systems/workspace/adapters/worktree-api";
import { inspectWorktree } from "@/systems/workspace/adapters/worktree-exit-api";
import { workspaceKeys } from "@/systems/workspace/lib/query-keys";
import { describeBranchOutcome } from "@/systems/workspace/lib/worktree-branch-outcome";
import { decodeWorktreeRefusal } from "@/systems/workspace/lib/worktree-refusal";
import { removeWorktreeFromList } from "@/systems/workspace/lib/worktree-list-reconciliation";
import { activeWorkspaceStore } from "@/systems/workspace/stores/active-workspace-store";
import type { WorktreePayload, WorktreesResponse } from "@/systems/workspace/types";
import type { WorktreeRemovalBatch } from "@/systems/workspace/hooks/use-worktree-removal-selection";

interface RemovalResult {
  row: WorktreePayload;
  status: "pending" | "running" | "success" | "failed";
  message: string;
}
const batchLogic = createStoreLogic({
  context: (batch: WorktreeRemovalBatch) => ({
    batch,
    deleteBranch: false,
    running: false,
    attempted: false,
    results: batch.worktrees.map(row => ({ row, status: "pending", message: "" }) as RemovalResult),
  }),
  on: {
    // The opt-in is part of the confirmed consequences; it freezes once a run starts.
    deleteBranch: (context, event: { value: boolean }) =>
      context.attempted ? undefined : { ...context, deleteBranch: event.value },
    start: context =>
      context.running ? undefined : { ...context, running: true, attempted: true },
    outcome: (
      context,
      event: { id: string; status: RemovalResult["status"]; message: string }
    ) => ({
      ...context,
      results: context.results.map(result =>
        result.row.id === event.id
          ? { ...result, status: event.status, message: event.message }
          : result
      ),
    }),
    finish: context => ({ ...context, running: false }),
  },
});

/**
 * Confirms terminal receipts or rejects drift in a frozen target before mutation.
 * Owner/path/state changes require fresh selection rather than changing the confirmed action.
 */
async function reconcileTarget(
  batch: WorktreeRemovalBatch,
  row: WorktreePayload
): Promise<boolean> {
  const current = (await inspectWorktree(batch.workspaceId, row.id)).worktree;
  if (
    current.id !== row.id ||
    current.workspace_id !== batch.workspaceId ||
    current.profile_id !== row.profile_id ||
    current.profile_archived
  ) {
    throw new Error("Ownership changed. Refresh the list before selecting this worktree again.");
  }
  if (current.state === "removed" || current.state === "dismissed") return true;
  if (current.state !== row.state || current.path !== row.path) {
    throw new Error("Worktree state changed. Refresh and confirm a new selection.");
  }
  if (current.agent_activity !== "idle") throw new Error("Worktree has an active session.");
  return false;
}

/** Runs one confirmed target with its owner's authority and describes the receipt. */
async function cleanupTarget(row: WorktreePayload, workspaceId: string, deleteBranch: boolean) {
  const options = { profile: row.profile_name, force: false, deleteBranch };
  if (row.state === "missing") {
    const branch = await dismissWorktree(workspaceId, row.id, options);
    const record = "Record dismissed. Files and history preserved.";
    return branch ? `${record} ${describeBranchOutcome(branch)}` : record;
  }
  const branch = await removeWorktree(workspaceId, row.id, options);
  return branch ? `Worktree removed. ${describeBranchOutcome(branch)}` : "Worktree removed.";
}

/**
 * Executes one confirmed batch through the singular lifecycle APIs in repository-queue order.
 * Reconciles lost responses, preserves successful receipts, and retries only unresolved
 * targets without force. Scope and cache cleanup follow the confirmed workspace and ID.
 */
export function useWorktreeRemovalBatch(input: WorktreeRemovalBatch) {
  const store = useStore(batchLogic, input);
  const context = useSelector(store, snapshot => snapshot.context);
  const queryClient = useQueryClient();
  const run = async () => {
    if (!store.can.start()) return;
    store.trigger.start();
    const { batch, results, deleteBranch } = store.getSnapshot().context;
    // Targets share the daemon's bounded repository queue. Chain each request
    // after the previous receipt so selecting many rows cannot exhaust it.
    return results
      .reduce<Promise<void>>(
        (previous, result) =>
          previous.then(async () => {
            if (result.status === "success") return;
            const row = result.row;
            store.trigger.outcome({ id: row.id, status: "running", message: "Checking…" });
            let message = "Already removed or dismissed; confirmed by CompozyOS.";
            try {
              if (!(await reconcileTarget(batch, row))) {
                message = await cleanupTarget(row, batch.workspaceId, deleteBranch);
              }
            } catch (error) {
              let confirmed = false;
              try {
                confirmed = await reconcileTarget(batch, row);
              } catch {
                /* Keep the original refusal visible. */
              }
              if (!confirmed) {
                store.trigger.outcome({
                  id: row.id,
                  status: "failed",
                  message:
                    decodeWorktreeRefusal(error)?.message ??
                    (error instanceof Error
                      ? error.message
                      : "Result unconfirmed. Retry checks CompozyOS before acting."),
                });
                return;
              }
            }
            store.trigger.outcome({ id: row.id, status: "success", message });
            activeWorkspaceStore.trigger.worktreeRemoved({
              workspaceId: batch.workspaceId,
              worktreeId: row.id,
            });
            queryClient.setQueryData<WorktreesResponse>(
              workspaceKeys.worktrees(batch.workspaceId),
              current => removeWorktreeFromList(current, row.id)
            );
            queryClient.removeQueries({
              queryKey: workspaceKeys.worktreeDetail(batch.workspaceId, row.id),
            });
          }),
        Promise.resolve()
      )
      .finally(() => {
        store.trigger.finish();
        void queryClient.invalidateQueries({
          queryKey: workspaceKeys.worktrees(batch.workspaceId),
        });
      });
  };
  return {
    ...context,
    setDeleteBranch: (value: boolean) => store.trigger.deleteBranch({ value }),
    run,
  };
}
