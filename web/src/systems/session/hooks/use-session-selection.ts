import { useEffect } from "react";
import { createStoreLogic } from "@xstate/store";
import { useSelector } from "@xstate/store-react";

import { useStoreBinding } from "@/hooks/use-store-binding";

import type { SessionPayload } from "../types";

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
    selectAll: (context, { ids }: { ids: readonly string[] }) => ({
      ...context,
      selectedIds: [...new Set([...context.selectedIds, ...ids])],
    }),
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
      session => session.archived_at === null && session.state === "stopped"
    ).length,
    unarchivable: sessions.filter(session => session.archived_at !== null).length,
    hiddenByFilter: sessions.filter(session => !visible.has(session.id)).length,
  };
}

/** One transient selection per list identity; filters and folds do not change that identity. */
export function useSessionSelection(
  scope: string,
  archived: boolean,
  sessions?: readonly SessionPayload[]
) {
  const { store } = useStoreBinding(JSON.stringify([scope, archived]), () =>
    sessionSelectionLogic.createStore()
  );
  const context = useSelector(store, snapshot => snapshot.context);
  useEffect(() => {
    if (sessions) store.trigger.prune({ ids: sessions.map(session => session.id) });
  }, [store, sessions]);
  return {
    selectedIds: context.selectedIds,
    anchorId: context.anchorId,
    mode: context.selectedIds.length > 0,
    toggle: (id: string) => store.trigger.toggle({ id }),
    toggleRange: (toId: string, visibleOrder: readonly string[]) =>
      store.trigger.toggleRange({ toId, visibleOrder }),
    selectAll: (ids: readonly string[]) => store.trigger.selectAll({ ids }),
    clear: () => store.trigger.clear(),
    prune: (ids: readonly string[]) => store.trigger.prune({ ids }),
  };
}

export interface SessionRowSelection {
  selectedIds: ReadonlySet<string>;
  mode: boolean;
  toggle: (id: string) => void;
  toggleRange: (id: string) => void;
}
