import { useSelector } from "@xstate/store-react";

import { useWorktreeScopeId } from "@/hooks/use-window-scope";

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
  };
}
