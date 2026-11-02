import { SHELL_WORKTREE_SCOPE } from "@/systems/workspace";

import {
  DEFAULT_SCOPE_STATE,
  isRecord,
  normalizedCollapsedFlags,
  normalizedScopeState,
  type SessionSidebarScopeState,
} from "./session-sidebar-scope";

export interface SessionSidebarContext {
  /**
   * Keyed by window scope (the window id, or `shell` outside a window). The
   * shell entry is the last-used state: every gesture also writes it, and a
   * window with no entry of its own starts from it.
   */
  byScope: Record<string, SessionSidebarScopeState>;
  /**
   * The window-manager partition (workspace + profile) whose snapshot listed
   * each window scope. A snapshot only lists its own partition's windows, so a
   * prune may drop only entries it owns: switching workspace or profile must
   * not erase the other partition's windows, which are still saved. An entry
   * no snapshot has listed yet is never pruned.
   */
  partitionByScope: Record<string, string>;
}

export function mergeSessionSidebarPreferences(
  persisted: Partial<SessionSidebarContext>,
  current: SessionSidebarContext
): SessionSidebarContext {
  if (!isRecord(persisted.byScope)) {
    return current;
  }
  const byScope = { ...current.byScope };
  for (const [scopeId, state] of Object.entries(persisted.byScope)) {
    const scope = normalizedScopeState(state);
    // The shell's filter was never a choice (`withScopeState`): a `false` stored by the old
    // default must not reach new windows.
    byScope[scopeId] =
      scopeId === SHELL_WORKTREE_SCOPE
        ? { ...scope, hideChildren: DEFAULT_SCOPE_STATE.hideChildren }
        : scope;
  }
  const partitionByScope = { ...current.partitionByScope };
  if (isRecord(persisted.partitionByScope)) {
    for (const [scopeId, partition] of Object.entries(persisted.partitionByScope)) {
      if (typeof partition === "string" && scopeId !== SHELL_WORKTREE_SCOPE) {
        partitionByScope[scopeId] = partition;
      }
    }
  }
  return { byScope, partitionByScope };
}

/**
 * Schema 3 keys the rail by window scope. Schema 2 was one global rail for
 * every window: it becomes the shell (last-used) entry, so every existing and
 * new window starts exactly where the operator left the old global rail.
 * Older blobs (schema 0, unversioned) persisted the whole context on every
 * transition — collapsing a thread wrote the old closed default back as
 * `open: false` — so their `open` is not a choice the operator made: drop it
 * and keep the per-thread collapse.
 */
export // Runs only for a blob older than the current schema (the persist extension
// skips it on a version match); the merge normalizes every scope entry.
function migrateSessionSidebarPreferences(
  persisted: unknown,
  version: string | number
): SessionSidebarContext {
  const legacy = isRecord(persisted) ? persisted : {};
  const shell =
    version === 2
      ? legacy
      : { collapsedThreads: normalizedCollapsedFlags(legacy.collapsedThreads) };
  return {
    byScope: { [SHELL_WORKTREE_SCOPE]: shell as unknown as SessionSidebarScopeState },
    partitionByScope: {},
  };
}

/**
 * Live scopes are claimed for the snapshot's partition; a closed scope is
 * dropped only when that same partition claimed it. `undefined` = no change.
 */
export function prunedSessionSidebarScopes(
  context: SessionSidebarContext,
  partition: string,
  liveScopeIds: readonly string[]
): SessionSidebarContext | undefined {
  const live = new Set(liveScopeIds);
  const byScope = { ...context.byScope };
  const partitionByScope = { ...context.partitionByScope };
  let changed = false;
  // Claim before seeding: a window can appear in the snapshot a commit
  // before its rail mounts and seeds an entry.
  for (const scopeId of live) {
    if (partitionByScope[scopeId] !== partition) {
      partitionByScope[scopeId] = partition;
      changed = true;
    }
  }
  for (const [scopeId, owner] of Object.entries(partitionByScope)) {
    if (owner === partition && !live.has(scopeId)) {
      delete partitionByScope[scopeId];
      delete byScope[scopeId];
      changed = true;
    }
  }
  return changed ? { byScope, partitionByScope } : undefined;
}
