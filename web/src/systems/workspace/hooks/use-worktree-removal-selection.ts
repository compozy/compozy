import { createStoreLogic } from "@xstate/store";
import { useSelector } from "@xstate/store-react";
import { useStoreBinding } from "@/hooks/use-store-binding";
import type { WorktreeNestEntry } from "@/systems/workspace/lib/worktree-display";
import type { WorktreePayload } from "@/systems/workspace/types";

export interface WorktreeRemovalProfile {
  id: string;
  name: string;
  archived: boolean;
}

export interface WorktreeRemovalBatch {
  workspaceId: string;
  profile: WorktreeRemovalProfile;
  worktrees: readonly WorktreePayload[];
}

export function worktreeCleanupReason(
  entry: WorktreeNestEntry,
  workspaceId: string,
  profile: WorktreeRemovalProfile | null | undefined
): string | null {
  const row = entry.worktree;
  if (!row) return "Discovered checkout — adopt individually before removal";
  if (row.workspace_id !== workspaceId) return "Belongs to another workspace";
  if (!profile || row.profile_id !== profile.id) return "Switch to the owning profile to remove";
  if (profile.archived || row.profile_archived)
    return "Archived profile — restore the profile first";
  if (row.agent_activity !== "idle") return "Worktree has an active session";
  if (row.state !== "ready" && row.state !== "missing") return "Worktree is not ready for removal";
  return null;
}

interface SelectionContext {
  mode: boolean;
  selected: readonly WorktreePayload[];
  anchor: string | null;
}

const selectionLogic = createStoreLogic({
  context: (): SelectionContext => ({ mode: false, selected: [], anchor: null }),
  on: {
    start: context => ({ ...context, mode: true }),
    clear: (): SelectionContext => ({ mode: false, selected: [], anchor: null }),
    toggle: (context, event: { row: WorktreePayload; range: readonly WorktreePayload[] }) => {
      const additions = event.range.length ? event.range : [event.row];
      const selected =
        event.range.length || !context.selected.some(row => row.id === event.row.id)
          ? [
              ...context.selected,
              ...additions.filter(row => !context.selected.some(old => old.id === row.id)),
            ]
          : context.selected.filter(row => row.id !== event.row.id);
      return { mode: true, selected, anchor: event.row.id };
    },
    all: (context, event: { rows: readonly WorktreePayload[] }) => ({
      ...context,
      mode: true,
      selected: [
        ...context.selected,
        ...event.rows.filter(row => !context.selected.some(old => old.id === row.id)),
      ],
    }),
  },
});

export function useWorktreeRemovalSelection(
  workspaceId: string,
  entries: readonly WorktreeNestEntry[],
  profile: WorktreeRemovalProfile | null | undefined,
  onRemove: ((batch: WorktreeRemovalBatch) => void) | undefined
) {
  const { store } = useStoreBinding(
    JSON.stringify([workspaceId, profile?.id, profile?.name, profile?.archived]),
    () => selectionLogic.createStore()
  );
  const context = useSelector(store, snapshot => snapshot.context);
  const eligible = entries.filter(
    entry => worktreeCleanupReason(entry, workspaceId, profile) === null
  );
  const selectedIds = new Set(context.selected.map(row => row.id));
  const toggle = (entry: WorktreeNestEntry, shift = false) => {
    if (!entry.worktree || worktreeCleanupReason(entry, workspaceId, profile)) return;
    const anchor = eligible.findIndex(row => row.key === context.anchor);
    const end = eligible.findIndex(row => row.key === entry.key);
    const range =
      shift && anchor >= 0 && end >= 0
        ? eligible
            .slice(Math.min(anchor, end), Math.max(anchor, end) + 1)
            .flatMap(row => (row.worktree ? [row.worktree] : []))
        : [];
    store.trigger.toggle({ row: { ...entry.worktree }, range });
  };
  const selectAll = () =>
    store.trigger.all({
      rows: eligible.flatMap(entry => (entry.worktree ? [{ ...entry.worktree }] : [])),
    });
  const clear = () => store.trigger.clear();
  return {
    enabled: Boolean(onRemove && profile && !profile.archived),
    mode: context.mode,
    count: context.selected.length,
    eligibleCount: eligible.length,
    selectedIds,
    reason: (entry: WorktreeNestEntry) => worktreeCleanupReason(entry, workspaceId, profile),
    start: () => store.trigger.start(),
    clear,
    selectAll,
    toggle,
    confirm: () => {
      if (profile && context.selected.length)
        onRemove?.({
          workspaceId,
          profile: { ...profile },
          worktrees: context.selected.map(row => ({ ...row })),
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
