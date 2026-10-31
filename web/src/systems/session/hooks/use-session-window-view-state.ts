import { useSelector } from "@xstate/store-react";

import { useWorktreeScopeId } from "@/hooks/use-window-scope";

import { SESSION_BOARD_DRAWER_DEFAULT_WIDTH } from "../lib/session-board-drawer-width";
import {
  resolveSessionSidebarScope,
  sessionSidebarStore,
  type SessionWindowViewMode,
} from "./use-session-sidebar-state";

export interface SessionWindowViewState {
  view: SessionWindowViewMode;
  setView: (view: SessionWindowViewMode) => void;
  /** The board's "Hide child sessions" filter (`children=exclude`). */
  hideChildren: boolean;
  setHideChildren: (hideChildren: boolean) => void;
  /** The board's session drawer width in this window (#1052), or the default. */
  drawerWidth: number;
  setDrawerWidth: (width: number) => void;
}

export function setSessionWindowView(scopeId: string, view: SessionWindowViewMode): void {
  sessionSidebarStore.trigger.viewChanged({ scopeId, view });
}

/**
 * List | Kanban for the session window rendering this hook, persisted per
 * window with the rest of its sessions presentation (US-011.AC-4).
 */
export function useSessionWindowViewState(): SessionWindowViewState {
  const scopeId = useWorktreeScopeId();
  const state = useSelector(sessionSidebarStore, snapshot =>
    resolveSessionSidebarScope(snapshot.context.byScope, scopeId)
  );
  return {
    view: state.view,
    setView: view => setSessionWindowView(scopeId, view),
    hideChildren: state.hideChildren,
    setHideChildren: hideChildren =>
      sessionSidebarStore.trigger.hideChildrenChanged({ scopeId, hideChildren }),
    drawerWidth: state.drawerWidth ?? SESSION_BOARD_DRAWER_DEFAULT_WIDTH,
    setDrawerWidth: width => sessionSidebarStore.trigger.drawerWidthChanged({ scopeId, width }),
  };
}

export interface SessionBoardColumnFolds {
  /** Folded board columns by id; a column absent from it is expanded. */
  collapsed: Readonly<Record<string, boolean>>;
  setCollapsed: (columnId: string, collapsed: boolean) => void;
}

/**
 * Which board columns this window folds (#1075). One fold per column covers every swimlane,
 * so the lanes' columns stay aligned, and it survives a reload with the rest of the window's
 * sessions presentation.
 */
export function useSessionBoardColumnFolds(): SessionBoardColumnFolds {
  const scopeId = useWorktreeScopeId();
  const collapsed = useSelector(
    sessionSidebarStore,
    snapshot => resolveSessionSidebarScope(snapshot.context.byScope, scopeId).collapsedBoardColumns
  );
  return {
    collapsed,
    setCollapsed: (columnId, next) =>
      sessionSidebarStore.trigger.boardColumnCollapsedChanged({
        scopeId,
        columnId,
        collapsed: next,
      }),
  };
}
