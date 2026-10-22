/** Per-window worktree picks plus the partition (workspace + profile) that claimed each window. */
export interface WorktreeScopeState {
  worktreeByScope: Record<string, string | null>;
  worktreePartitionByScope: Record<string, string>;
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

/**
 * Live scopes are claimed for the snapshot's partition; a closed scope is
 * dropped only when that same partition claimed it, so another destination's
 * windows survive while it is not mounted. `undefined` = no change.
 */
export function prunedWorktreeScopes<T extends WorktreeScopeState>(
  state: T,
  partition: string,
  liveScopeIds: readonly string[],
  shellScopeId: string
): T | undefined {
  const live = new Set(liveScopeIds);
  const worktreeByScope = { ...state.worktreeByScope };
  const worktreePartitionByScope = { ...state.worktreePartitionByScope };
  let changed = false;
  for (const scopeId of live) {
    if (scopeId !== shellScopeId && worktreePartitionByScope[scopeId] !== partition) {
      worktreePartitionByScope[scopeId] = partition;
      changed = true;
    }
  }
  for (const [scopeId, owner] of Object.entries(state.worktreePartitionByScope)) {
    if (owner !== partition || live.has(scopeId)) continue;
    delete worktreePartitionByScope[scopeId];
    delete worktreeByScope[scopeId];
    changed = true;
  }
  return changed ? { ...state, worktreeByScope, worktreePartitionByScope } : undefined;
}

/**
 * The store's persist filter skips prunes so this document's possibly stale
 * picks never overwrite another tab's. Claims and cleanup still have to
 * survive a reload — otherwise a window closed before the next gesture keeps
 * its entry forever — so apply the same prune to the stored blob alone.
 */
export function persistWorktreePrune(
  storageKey: string,
  schemaVersion: number,
  prune: (state: WorktreeScopeState) => WorktreeScopeState | undefined
): void {
  if (typeof window === "undefined") return;
  try {
    const raw = window.localStorage.getItem(storageKey);
    const stored: unknown = raw === null ? null : JSON.parse(raw);
    // Nothing stored yet, or an older schema the next gesture migrates.
    if (!isRecord(stored) || stored.version !== schemaVersion || !isRecord(stored.context)) return;
    const context = stored.context;
    const next = prune({
      worktreeByScope: isRecord(context.worktreeByScope)
        ? (context.worktreeByScope as Record<string, string | null>)
        : {},
      worktreePartitionByScope: isRecord(context.worktreePartitionByScope)
        ? (context.worktreePartitionByScope as Record<string, string>)
        : {},
    });
    if (next) {
      window.localStorage.setItem(
        storageKey,
        JSON.stringify({ ...stored, context: { ...context, ...next } })
      );
    }
  } catch {
    // Storage unavailable or corrupt: the next gesture rewrites the blob.
  }
}
