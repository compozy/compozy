import { createStoreLogic } from "@xstate/store";
import { useSelector } from "@xstate/store-react";
import { useStoreBinding } from "@/hooks/use-store-binding";
import type { WorktreeNestEntry } from "@/systems/workspace/lib/worktree-display";
import type { DiscoveredWorktreePayload, WorktreePayload } from "@/systems/workspace/types";

export interface WorktreeRemovalBatch {
  workspaceId: string;
  /** Frozen rows; each target acts with its own owner profile, as single cleanup does. */
  worktrees: readonly WorktreePayload[];
  /** Git-only checkouts, removed by their listed path. They have no owner and keep their branch. */
  discovered?: readonly DiscoveredWorktreePayload[];
}

/**
 * Explains why a represented row cannot be cleaned up, or `null` when it can.
 * Cleanup acts with the row's owner profile, so ownership refuses only when that
 * owner is unresolvable or archived — never because another profile is active.
 * Discovered checkouts have no record or owner; the daemon guards their loss risk.
 */
export function worktreeCleanupReason(
  entry: WorktreeNestEntry,
  workspaceId: string
): string | null {
  if (entry.discovered) return null;
  const row = entry.worktree;
  if (!row) return "Worktree is not ready for removal";
  if (row.workspace_id !== workspaceId) return "Belongs to another workspace";
  if (!row.profile_name) return "Owning profile is unavailable";
  if (row.profile_archived) return "Archived profile — restore the profile first";
  if (row.agent_activity !== "idle") return "Worktree has an active session";
  if (row.state !== "ready" && row.state !== "missing") return "Worktree is not ready for removal";
  return null;
}

/** A frozen copy of one selected row, keyed like its nest entry. */
interface SelectedTarget {
  key: string;
  worktree: WorktreePayload | null;
  discovered: DiscoveredWorktreePayload | null;
}

function selectedTarget(entry: WorktreeNestEntry): SelectedTarget {
  return {
    key: entry.key,
    worktree: entry.worktree ? { ...entry.worktree } : null,
    discovered: entry.discovered ? { ...entry.discovered } : null,
  };
}

interface SelectionContext {
  mode: boolean;
  selected: readonly SelectedTarget[];
  anchor: string | null;
}

const selectionLogic = createStoreLogic({
  context: (): SelectionContext => ({ mode: false, selected: [], anchor: null }),
  on: {
    start: context => ({ ...context, mode: true }),
    clear: (): SelectionContext => ({ mode: false, selected: [], anchor: null }),
    toggle: (context, event: { row: SelectedTarget; range: readonly SelectedTarget[] }) => {
      const additions = event.range.length ? event.range : [event.row];
      const selected =
        event.range.length || !context.selected.some(row => row.key === event.row.key)
          ? [
              ...context.selected,
              ...additions.filter(row => !context.selected.some(old => old.key === row.key)),
            ]
          : context.selected.filter(row => row.key !== event.row.key);
      return { mode: true, selected, anchor: event.row.key };
    },
    all: (context, event: { rows: readonly SelectedTarget[] }) => ({
      ...context,
      mode: true,
      selected: [
        ...context.selected,
        ...event.rows.filter(row => !context.selected.some(old => old.key === row.key)),
      ],
    }),
  },
});

/**
 * Holds copied row identities within one workspace selection scope. Select-all
 * includes only currently represented eligible rows; later catalog updates
 * never silently add targets or replace the saved selection payloads.
 */
export function useWorktreeRemovalSelection(
  workspaceId: string,
  entries: readonly WorktreeNestEntry[],
  onRemove: ((batch: WorktreeRemovalBatch) => void) | undefined
) {
  const { store } = useStoreBinding(workspaceId, () => selectionLogic.createStore());
  const context = useSelector(store, snapshot => snapshot.context);
  const eligible = entries.filter(entry => worktreeCleanupReason(entry, workspaceId) === null);
  const selectedIds = new Set(context.selected.map(row => row.key));
  const toggle = (entry: WorktreeNestEntry, shift = false) => {
    if (worktreeCleanupReason(entry, workspaceId)) return;
    const anchor = eligible.findIndex(row => row.key === context.anchor);
    const end = eligible.findIndex(row => row.key === entry.key);
    const range =
      shift && anchor >= 0 && end >= 0
        ? eligible.slice(Math.min(anchor, end), Math.max(anchor, end) + 1).map(selectedTarget)
        : [];
    store.trigger.toggle({ row: selectedTarget(entry), range });
  };
  const selectAll = () => store.trigger.all({ rows: eligible.map(selectedTarget) });
  const clear = () => store.trigger.clear();
  return {
    enabled: Boolean(onRemove),
    mode: context.mode,
    count: context.selected.length,
    eligibleCount: eligible.length,
    // By identity, not count: a saved row that left the list must not make the
    // remaining unchecked rows look selected.
    allEligibleSelected: eligible.length > 0 && eligible.every(row => selectedIds.has(row.key)),
    selectedIds,
    reason: (entry: WorktreeNestEntry) => worktreeCleanupReason(entry, workspaceId),
    start: () => store.trigger.start(),
    clear,
    selectAll,
    toggle,
    confirm: () => {
      if (!context.selected.length) return;
      onRemove?.({
        workspaceId,
        worktrees: context.selected.flatMap(row => (row.worktree ? [{ ...row.worktree }] : [])),
        discovered: context.selected.flatMap(row =>
          row.discovered ? [{ ...row.discovered }] : []
        ),
      });
    },
    onKeyDown: (event: React.KeyboardEvent) => {
      if (
        !context.mode ||
        (event.target instanceof Element &&
          event.target.closest('input, textarea, [contenteditable="true"]'))
      )
        return;
      if ((event.ctrlKey || event.metaKey) && event.key.toLowerCase() === "a") {
        event.preventDefault();
        event.stopPropagation();
        selectAll();
      } else if (event.key === "Escape") {
        event.preventDefault();
        event.stopPropagation();
        clear();
      }
    },
  };
}
export type WorktreeRemovalSelection = ReturnType<typeof useWorktreeRemovalSelection>;
