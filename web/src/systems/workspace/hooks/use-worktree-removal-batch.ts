import { createStoreLogic } from "@xstate/store";
import { useSelector, useStore } from "@xstate/store-react";
import { useQueryClient } from "@tanstack/react-query";
import { dismissWorktree, removeWorktree } from "../adapters/worktree-api";
import { inspectWorktree } from "../adapters/worktree-exit-api";
import { workspaceKeys } from "../lib/query-keys";
import { decodeWorktreeRefusal } from "../lib/worktree-refusal";
import { removeWorktreeFromList } from "../lib/worktree-list-reconciliation";
import { activeWorkspaceStore } from "../stores/active-workspace-store";
import type { WorktreePayload, WorktreesResponse } from "../types";
import type {
  WorktreeRemovalBatch,
  WorktreeRemovalProfile,
} from "./use-worktree-removal-selection";

interface RemovalResult {
  row: WorktreePayload;
  status: "pending" | "running" | "success" | "failed";
  message: string;
}
const batchLogic = createStoreLogic({
  context: (batch: WorktreeRemovalBatch) => ({
    batch,
    running: false,
    attempted: false,
    results: batch.worktrees.map(row => ({ row, status: "pending", message: "" }) as RemovalResult),
  }),
  on: {
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

async function reconcileTarget(
  batch: WorktreeRemovalBatch,
  row: WorktreePayload
): Promise<boolean> {
  const current = (await inspectWorktree(batch.workspaceId, row.id)).worktree;
  if (
    current.id !== row.id ||
    current.workspace_id !== batch.workspaceId ||
    current.profile_id !== batch.profile.id ||
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

export function useWorktreeRemovalBatch(
  input: WorktreeRemovalBatch,
  activeProfile: WorktreeRemovalProfile | null
) {
  const store = useStore(batchLogic, input);
  const context = useSelector(store, snapshot => snapshot.context);
  const queryClient = useQueryClient();
  const scopeMatches =
    activeProfile?.id === context.batch.profile.id &&
    activeProfile.name === context.batch.profile.name &&
    !activeProfile.archived;
  const run = async () => {
    if (!scopeMatches || !store.can.start()) return;
    store.trigger.start();
    const { batch, results } = store.getSnapshot().context;
    try {
      for (const result of results) {
        if (result.status === "success") continue;
        const row = result.row;
        store.trigger.outcome({ id: row.id, status: "running", message: "Checking…" });
        let message = "Already removed or dismissed; confirmed by CompozyOS.";
        try {
          if (!(await reconcileTarget(batch, row))) {
            if (row.state === "missing") {
              await dismissWorktree(batch.workspaceId, row.id, undefined, batch.profile.name);
              message = "Record dismissed. Files and history preserved.";
            } else {
              await removeWorktree(batch.workspaceId, row.id, {
                profile: batch.profile.name,
                force: false,
              });
              message = "Worktree removed.";
            }
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
            continue;
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
      }
    } finally {
      store.trigger.finish();
      void queryClient.invalidateQueries({ queryKey: workspaceKeys.worktrees(batch.workspaceId) });
    }
  };
  return { ...context, scopeMatches, run };
}
