import { useSelector } from "@xstate/store-react";

import { useWorktreeScopeId } from "@/hooks/use-window-scope";

import type { SessionListGroupBy, SessionListSort } from "../lib/session-list-preferences";
import { useSessionListPreferences } from "./use-session-list-preferences";
import { resolveSessionWindowOrder, sessionSidebarStore } from "./use-session-sidebar-state";

export interface SessionWindowOrder {
  sort: SessionListSort;
  /** The stored grouping; each surface renders it only where its breadth allows. */
  groupBy: SessionListGroupBy;
  setSort: (sort: SessionListSort) => void;
  setGroupBy: (groupBy: SessionListGroupBy) => void;
}

/**
 * Session order and grouping for the window rendering this hook, shared by its
 * List and Kanban, so a choice made in one view holds after switching to the
 * other. Each window keeps its own choice in its persisted sessions entry; a
 * window that never chose follows the daemon's `shell.sessions` value, which
 * seeds windows but is never written from here, so two windows stay independent.
 */
export function useSessionWindowOrder(): SessionWindowOrder {
  const scopeId = useWorktreeScopeId();
  const storedSort = useSelector(
    sessionSidebarStore,
    snapshot => resolveSessionWindowOrder(snapshot.context.byScope, scopeId).sort
  );
  const storedGroupBy = useSelector(
    sessionSidebarStore,
    snapshot => resolveSessionWindowOrder(snapshot.context.byScope, scopeId).groupBy
  );
  const seed = useSessionListPreferences();
  return {
    sort: storedSort ?? seed.sort,
    groupBy: storedGroupBy ?? seed.groupBy,
    setSort: sort => sessionSidebarStore.trigger.sortChanged({ scopeId, sort }),
    setGroupBy: groupBy => sessionSidebarStore.trigger.groupByChanged({ scopeId, groupBy }),
  };
}
