import { useEffect } from "react";
import { createStoreLogic } from "@xstate/store";
import { useSelector } from "@xstate/store-react";

import { useStoreBinding } from "@/hooks/use-store-binding";

import { isSubagentSession } from "../lib/session-hierarchy";
import { sessionIsInLater } from "../lib/session-later";
import type { SessionListShelf } from "../lib/session-list-shelf";
import type { SessionPayload } from "../types";
import type { SessionSelectionVerbs } from "./use-session-list-bulk-selection";

interface SessionSelectionContext {
  selectedIds: readonly string[];
  anchorId: string | null;
}

export const sessionSelectionLogic = createStoreLogic({
  context: (): SessionSelectionContext => ({ selectedIds: [], anchorId: null }),
  on: {
    toggle: (context, { id }: { id: string }) => ({
      selectedIds: context.selectedIds.includes(id)
        ? context.selectedIds.filter(selected => selected !== id)
        : [...context.selectedIds, id],
      anchorId: id,
    }),
    toggleRange: (
      context,
      { toId, visibleOrder }: { toId: string; visibleOrder: readonly string[] }
    ) => {
      const to = visibleOrder.indexOf(toId);
      if (to < 0) return context;
      const anchor = context.anchorId === null ? -1 : visibleOrder.indexOf(context.anchorId);
      const from = anchor < 0 ? to : anchor;
      return {
        selectedIds: [
          ...new Set([
            ...context.selectedIds,
            ...visibleOrder.slice(Math.min(from, to), Math.max(from, to) + 1),
          ]),
        ],
        anchorId: anchor < 0 ? toId : context.anchorId,
      };
    },
    /** Finder's retarget: the one row a context click lands on outside the selection. */
    selectOnly: (_context, { id }: { id: string }) => ({ selectedIds: [id], anchorId: id }),
    selectAll: (context, { ids }: { ids: readonly string[] }) => ({
      ...context,
      selectedIds: [...new Set([...context.selectedIds, ...ids])],
    }),
    /** Clears a group's checkbox: its sessions leave, the rest of the selection stays. */
    deselect: (context, { ids }: { ids: readonly string[] }) => {
      const leaving = new Set(ids);
      const selectedIds = context.selectedIds.filter(id => !leaving.has(id));
      if (selectedIds.length === context.selectedIds.length) return context;
      const anchorId =
        context.anchorId !== null && leaving.has(context.anchorId) ? null : context.anchorId;
      return { selectedIds, anchorId };
    },
    clear: (): SessionSelectionContext => ({ selectedIds: [], anchorId: null }),
    prune: (context, { ids }: { ids: readonly string[] }) => {
      const current = new Set(ids);
      const selectedIds = context.selectedIds.filter(id => current.has(id));
      const anchorId =
        context.anchorId !== null && current.has(context.anchorId) ? context.anchorId : null;
      return selectedIds.length === context.selectedIds.length && anchorId === context.anchorId
        ? context
        : { selectedIds, anchorId };
    },
  },
});

/** Derive eligible verbs from current payloads and count selected rows outside the visible order. */
export function sessionSelectionCounts(
  sessions: readonly SessionPayload[],
  visibleIds: readonly string[]
) {
  const visible = new Set(visibleIds);
  return {
    stoppable: sessions.filter(
      session =>
        session.archived_at === null && (session.state === "active" || session.state === "starting")
    ).length,
    archivable: sessions.filter(
      session => session.archived_at === null && !isSubagentSession(session)
    ).length,
    // Archivable sessions the daemon must stop first.
    archivableLive: sessions.filter(
      session =>
        session.archived_at === null && session.state !== "stopped" && !isSubagentSession(session)
    ).length,
    unarchivable: sessions.filter(
      session => session.archived_at !== null && !isSubagentSession(session)
    ).length,
    // Subagent sessions move to and come back from Later with their parent.
    laterable: sessions.filter(
      session =>
        session.archived_at === null && !sessionIsInLater(session) && !isSubagentSession(session)
    ).length,
    backFromLaterable: sessions.filter(
      session => sessionIsInLater(session) && !isSubagentSession(session)
    ).length,
    hiddenByFilter: sessions.filter(session => !visible.has(session.id)).length,
  };
}

export type SessionSelectionStore = ReturnType<typeof sessionSelectionLogic.createStore>;

/**
 * One transient selection per list identity; filters and folds do not change that identity.
 * Selected sessions missing from `sessions` leave the selection, unless `holdMissing` keeps
 * them until their absence is confirmed (a board card between two column reads).
 */
export function useSessionSelection(
  scope: string,
  /** The list's shelf (active, archive, Later): each is its own selection. */
  shelf: SessionListShelf,
  sessions?: readonly SessionPayload[],
  holdMissing?: (missingIds: readonly string[]) => boolean
) {
  const { store } = useStoreBinding(JSON.stringify([scope, shelf]), () =>
    sessionSelectionLogic.createStore()
  );
  const context = useSelector(store, snapshot => snapshot.context);
  const currentIds = sessions ? new Set(sessions.map(session => session.id)) : null;
  const missing = currentIds ? context.selectedIds.filter(id => !currentIds.has(id)) : [];
  const held = missing.length > 0 && holdMissing?.(missing) === true;
  const pruneTo = held ? undefined : sessions;
  useEffect(() => {
    if (pruneTo) store.trigger.prune({ ids: pruneTo.map(session => session.id) });
  }, [store, pruneTo]);
  const selectedIds =
    currentIds && !held
      ? context.selectedIds.filter(id => currentIds.has(id))
      : context.selectedIds;
  const anchorId =
    currentIds && !held && !currentIds.has(context.anchorId ?? "") ? null : context.anchorId;
  return {
    /** The store itself, for hosts whose items subscribe to their own membership. */
    store,
    selectedIds,
    anchorId,
    mode: selectedIds.length > 0,
    toggle: (id: string) => store.trigger.toggle({ id }),
    toggleRange: (toId: string, visibleOrder: readonly string[]) =>
      store.trigger.toggleRange({ toId, visibleOrder }),
    selectOnly: (id: string) => store.trigger.selectOnly({ id }),
    selectAll: (ids: readonly string[]) => store.trigger.selectAll({ ids }),
    deselect: (ids: readonly string[]) => store.trigger.deselect({ ids }),
    clear: () => store.trigger.clear(),
    prune: (ids: readonly string[]) => store.trigger.prune({ ids }),
  };
}

export interface SessionRowSelection {
  selectedIds: ReadonlySet<string>;
  mode: boolean;
  toggle: (id: string) => void;
  toggleRange: (id: string) => void;
  /** Replaces the selection with one row (a context click outside it). */
  selectOnly: (id: string) => void;
  /** The selection's verbs; a selected row's context menu acts on all of them. */
  verbs?: SessionSelectionVerbs;
}
