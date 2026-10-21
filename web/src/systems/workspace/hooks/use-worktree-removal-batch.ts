import { createStoreLogic } from "@xstate/store";
import { useSelector, useStore } from "@xstate/store-react";
import { useQueryClient } from "@tanstack/react-query";
import {
  dismissWorktree,
  removeDiscoveredWorktree,
  removeWorktree,
  WorktreeApiError,
} from "@/systems/workspace/adapters/worktree-api";
import { inspectWorktree } from "@/systems/workspace/adapters/worktree-exit-api";
import { workspaceKeys } from "@/systems/workspace/lib/query-keys";
import { describeBranchOutcome } from "@/systems/workspace/lib/worktree-branch-outcome";
import {
  decodeWorktreeRefusal,
  WORKTREE_ERROR_CODES,
  type WorktreeRefusal,
} from "@/systems/workspace/lib/worktree-refusal";
import {
  removeDiscoveredFromList,
  removeWorktreeFromList,
} from "@/systems/workspace/lib/worktree-list-reconciliation";
import { isStaleDiscovered } from "@/systems/workspace/lib/worktree-display";
import { activeWorkspaceStore } from "@/systems/workspace/stores/active-workspace-store";
import type {
  DiscoveredWorktreePayload,
  WorktreePayload,
  WorktreesResponse,
} from "@/systems/workspace/types";
import type { WorktreeRemovalBatch } from "@/systems/workspace/hooks/use-worktree-removal-selection";

/** One frozen batch target: a registered record, or a Git-only checkout named by its path. */
export type WorktreeRemovalTarget =
  | { kind: "worktree"; key: string; row: WorktreePayload }
  | { kind: "discovered"; key: string; discovered: DiscoveredWorktreePayload };

interface RemovalResult {
  target: WorktreeRemovalTarget;
  status: "pending" | "running" | "success" | "failed";
  message: string;
  refusal?: WorktreeRefusal | null;
}

function batchTargets(batch: WorktreeRemovalBatch): WorktreeRemovalTarget[] {
  return [
    ...batch.worktrees.map(row => ({ kind: "worktree" as const, key: row.id, row })),
    ...(batch.discovered ?? []).map(discovered => ({
      kind: "discovered" as const,
      key: `discovered:${discovered.path}`,
      discovered,
    })),
  ];
}

function canForce(result: RemovalResult) {
  const target = result.target;
  return (
    result.status === "failed" &&
    (target.kind === "discovered" || target.row.state === "ready") &&
    result.refusal?.risk != null &&
    (result.refusal.code === WORKTREE_ERROR_CODES.dirtyRequiresForce ||
      result.refusal.code === WORKTREE_ERROR_CODES.unpushedRequiresForce)
  );
}

const batchLogic = createStoreLogic({
  context: (batch: WorktreeRemovalBatch) => ({
    batch,
    deleteBranch: false,
    running: false,
    attempted: false,
    forceTargets: [] as RemovalResult[],
    results: batchTargets(batch).map(
      target => ({ target, status: "pending", message: "" }) as RemovalResult
    ),
  }),
  on: {
    requestForce: context =>
      context.running
        ? undefined
        : {
            ...context,
            forceTargets: context.results.filter(canForce),
          },
    cancelForce: context => ({ ...context, forceTargets: [] }),
    // The opt-in is part of the confirmed consequences; it freezes once a run starts.
    deleteBranch: (context, event: { value: boolean }) =>
      context.attempted ? undefined : { ...context, deleteBranch: event.value },
    start: context =>
      context.running
        ? undefined
        : { ...context, running: true, attempted: true, forceTargets: [] },
    outcome: (
      context,
      event: {
        key: string;
        status: RemovalResult["status"];
        message: string;
        refusal?: WorktreeRefusal | null;
      }
    ) => ({
      ...context,
      results: context.results.map(result =>
        result.target.key === event.key
          ? { ...result, status: event.status, message: event.message, refusal: event.refusal }
          : result
      ),
    }),
    finish: context => ({ ...context, running: false }),
  },
});

type TerminalState = "removed" | "dismissed";

/**
 * Confirms terminal receipts or rejects drift in a frozen target before mutation.
 * Owner/path/state changes require fresh selection rather than changing the confirmed action.
 * Resolves the terminal state when the record is already cleaned up, otherwise `null`.
 */
async function reconcileTarget(
  batch: WorktreeRemovalBatch,
  row: WorktreePayload
): Promise<TerminalState | null> {
  const current = (await inspectWorktree(batch.workspaceId, row.id)).worktree;
  if (
    current.id !== row.id ||
    current.workspace_id !== batch.workspaceId ||
    current.profile_id !== row.profile_id ||
    current.profile_archived
  ) {
    throw new Error("Ownership changed. Refresh the list before selecting this worktree again.");
  }
  if (current.state === "removed" || current.state === "dismissed") return current.state;
  if (current.state !== row.state || current.path !== row.path) {
    throw new Error("Worktree state changed. Refresh and confirm a new selection.");
  }
  if (current.agent_activity !== "idle") throw new Error("Worktree has an active session.");
  return null;
}

/**
 * Runs one confirmed target with its owner's authority and describes the receipt.
 * Repeating cleanup on an already-terminal record re-runs only the branch step.
 */
async function cleanupTarget(
  row: WorktreePayload,
  workspaceId: string,
  deleteBranch: boolean,
  terminal: TerminalState | null,
  force: boolean
) {
  const options = { profile: row.profile_name, force, deleteBranch };
  const dismiss = terminal ? terminal === "dismissed" : row.state === "missing";
  const branch = dismiss
    ? await dismissWorktree(workspaceId, row.id, options)
    : await removeWorktree(workspaceId, row.id, options);
  const record = terminal
    ? "Already removed or dismissed; confirmed by CompozyOS."
    : dismiss
      ? "Record dismissed. Files and history preserved."
      : "Worktree removed.";
  return branch ? `${record} ${describeBranchOutcome(branch)}` : record;
}

/**
 * Removes one discovered checkout by its listed path. Git no longer listing the
 * path is the terminal receipt, so a lost response retries into confirmation.
 */
async function cleanupDiscovered(
  discovered: DiscoveredWorktreePayload,
  workspaceId: string,
  force: boolean
) {
  try {
    await removeDiscoveredWorktree(workspaceId, discovered.path, { force });
  } catch (error) {
    if (error instanceof WorktreeApiError && error.status === 404) {
      return "No longer listed by Git; confirmed by CompozyOS.";
    }
    throw error;
  }
  return isStaleDiscovered(discovered)
    ? "Stale Git entry cleared. Branch kept."
    : "Worktree removed. Branch kept.";
}

/** Cleanup succeeded but the requested branch step is unconfirmed; retry repeats only it. */
const BRANCH_UNCONFIRMED =
  "Worktree cleaned up, but the branch result is unconfirmed. Retry repeats only the branch step.";

/**
 * Executes one confirmed batch through the singular lifecycle APIs in repository-queue order.
 * Reconciles lost responses, preserves successful receipts, and retries only unresolved
 * targets with force only after explicit confirmation. Scope and cache cleanup follow the confirmed workspace and ID.
 */
export function useWorktreeRemovalBatch(input: WorktreeRemovalBatch) {
  const store = useStore(batchLogic, input);
  const context = useSelector(store, snapshot => snapshot.context);
  const queryClient = useQueryClient();
  const run = async (forceTargets: RemovalResult[] = []) => {
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
            const force =
              forceTargets.some(target => target.target.key === result.target.key) &&
              canForce(result);
            if (forceTargets.length > 0 && !force) return;
            const target = result.target;
            if (target.kind === "discovered") {
              store.trigger.outcome({ key: target.key, status: "running", message: "Removing…" });
              try {
                const message = await cleanupDiscovered(
                  target.discovered,
                  batch.workspaceId,
                  force
                );
                store.trigger.outcome({ key: target.key, status: "success", message });
                queryClient.setQueryData<WorktreesResponse>(
                  workspaceKeys.worktrees(batch.workspaceId),
                  current => removeDiscoveredFromList(current, target.discovered.path)
                );
              } catch (error) {
                const refusal = decodeWorktreeRefusal(error);
                store.trigger.outcome({
                  key: target.key,
                  status: "failed",
                  refusal,
                  message:
                    refusal?.message ??
                    (error instanceof Error
                      ? error.message
                      : "Result unconfirmed. Retry checks CompozyOS before acting."),
                });
              }
              return;
            }
            const row = target.row;
            store.trigger.outcome({ key: row.id, status: "running", message: "Checking…" });
            let message = "Already removed or dismissed; confirmed by CompozyOS.";
            try {
              const terminal = await reconcileTarget(batch, row);
              // A terminal record still owes its branch outcome when the opt-in was confirmed.
              if (!terminal || deleteBranch) {
                message = await cleanupTarget(
                  row,
                  batch.workspaceId,
                  deleteBranch,
                  terminal,
                  force
                );
              }
            } catch (error) {
              let confirmed: TerminalState | null = null;
              try {
                confirmed = await reconcileTarget(batch, row);
              } catch {
                /* Keep the original refusal visible. */
              }
              if (!confirmed || deleteBranch) {
                store.trigger.outcome({
                  key: row.id,
                  status: "failed",
                  refusal: confirmed ? null : decodeWorktreeRefusal(error),
                  message: confirmed
                    ? BRANCH_UNCONFIRMED
                    : (decodeWorktreeRefusal(error)?.message ??
                      (error instanceof Error
                        ? error.message
                        : "Result unconfirmed. Retry checks CompozyOS before acting.")),
                });
                return;
              }
            }
            store.trigger.outcome({ key: row.id, status: "success", message });
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
    forceEligible: context.results.filter(canForce),
    requestForce: () => store.trigger.requestForce(),
    cancelForce: () => store.trigger.cancelForce(),
    confirmForce: () => {
      const targets = store.getSnapshot().context.forceTargets;
      if (targets.length > 0) return run(targets);
    },
    run: () => run(),
  };
}
