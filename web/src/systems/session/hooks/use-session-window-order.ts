import { useSelector } from "@xstate/store-react";

import { useWorktreeScopeId } from "@/hooks/use-window-scope";

import type {
  SessionListComputerLayout,
  SessionListGroupBy,
  SessionListSort,
} from "../lib/session-list-preferences";
import type { SessionWindowComputerScope } from "../lib/session-sidebar-scope";
import { useSessionListPreferences } from "./use-session-list-preferences";
import { resolveSessionWindowOrder, sessionSidebarStore } from "./use-session-sidebar-state";

export interface SessionWindowOrder {
  sort: SessionListSort;
  /** The stored grouping; each surface renders it only where its breadth allows. */
  groupBy: SessionListGroupBy;
  /** How remote computers share the window: Together or Separate (seed from `shell.sessions.computers`). */
  computers: SessionListComputerLayout;
  /** Per-window computer filter: `"all"`, `"local"`, or a remote computer id. */
  computerScope: SessionWindowComputerScope;
  setSort: (sort: SessionListSort) => void;
  setGroupBy: (groupBy: SessionListGroupBy) => void;
  setComputers: (computers: SessionListComputerLayout) => void;
  setComputerScope: (computerScope: SessionWindowComputerScope) => void;
}

/**
 * Session order, grouping, computers layer and computer scope for the window
 * rendering this hook, shared by its List and Kanban, so a choice made in one
 * view holds after switching to the other. Each window keeps its own choice in
 * its persisted sessions entry; a window that never chose follows the daemon's
 * `shell.sessions` value for order/grouping/layer and starts at All computers.
 */
export function useSessionWindowOrder(): SessionWindowOrder {
  const scopeId = useWorktreeScopeId();
  const stored = useSelector(sessionSidebarStore, snapshot =>
    resolveSessionWindowOrder(snapshot.context.byScope, scopeId)
  );
  const seed = useSessionListPreferences();
  return {
    sort: stored.sort ?? seed.sort,
    groupBy: stored.groupBy ?? seed.groupBy,
    computers: stored.computers ?? seed.computers,
    computerScope: stored.computerScope,
    setSort: sort => sessionSidebarStore.trigger.sortChanged({ scopeId, sort }),
    setGroupBy: groupBy => sessionSidebarStore.trigger.groupByChanged({ scopeId, groupBy }),
    setComputers: computers => sessionSidebarStore.trigger.computersChanged({ scopeId, computers }),
    setComputerScope: computerScope =>
      sessionSidebarStore.trigger.computerScopeChanged({ scopeId, computerScope }),
  };
}
