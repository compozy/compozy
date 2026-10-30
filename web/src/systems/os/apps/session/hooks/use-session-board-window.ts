import {
  setSessionWindowView,
  useSessionCreateActions,
  useSessionsBoard,
  useSessionWindowOrder,
  useSessionWindowViewState,
  type SessionBoardPayload,
  type SessionWindowViewMode,
} from "@/systems/session";
import { useDataScope } from "@/systems/workspace";

import { useOsShell } from "../../../hooks/use-os-shell";
import { useSessionListLifecycleActions } from "../../../hooks/use-session-list-lifecycle-actions";
import { mruWindowInstance } from "../../../lib/window-instance-lookup";

import {
  useSessionWindowOpenSession,
  type SessionWindowSidebarTarget,
} from "../use-session-window-open-session";
import { useSessionBoardDrawer } from "./use-session-board-drawer";

/**
 * Board reads plus the card gestures: a card opens its session in this window's drawer
 * (#1052) — or focuses the window that already shows it, in List — and the card menus run
 * the session list's lifecycle verbs. Sort and Group by are this window's, shared with its List.
 */
export function useSessionBoardWindow(
  windowId: string,
  sessionId: string | null,
  drawerOpen: boolean
) {
  const scope = useDataScope();
  const { manager } = useOsShell();
  const view = useSessionWindowViewState();
  const order = useSessionWindowOrder();
  const board = useSessionsBoard({
    scope,
    hideChildren: view.hideChildren,
    sort: order.sort,
    groupBy: order.groupBy,
  });
  const { openForAgent } = useSessionCreateActions();
  const openSession = useSessionWindowOpenSession({ windowId, sessionId: sessionId ?? undefined });

  const openInDrawer = (target: SessionWindowSidebarTarget) => {
    const state = manager.getState();
    const owner = mruWindowInstance(state.windows, state.client?.focusOrder ?? [], {
      app: "session",
      instanceKey: target.sessionId,
    });
    // One window per session: a session another window shows is focused there, in List.
    if (owner && owner.id !== windowId) {
      setSessionWindowView(owner.id, "list");
      openSession(target, { newWindow: false, drawer: false });
      return;
    }
    openSession(target, { newWindow: false, drawer: true });
  };
  const drawer = useSessionBoardDrawer({
    windowId,
    sessionId,
    open: drawerOpen,
    openSession: openInDrawer,
  });
  const lifecycle = useSessionListLifecycleActions({ onRemoved: drawer.onSessionRemoved });
  const actions = drawer.withLanding(lifecycle.actions);

  const onOpen = (session: SessionBoardPayload) =>
    openInDrawer({
      sessionId: session.id,
      agentName: session.agent_name,
      workspaceId: session.workspace_id ?? "",
    });
  /** List shows the drawer's session full size: leaving the board takes the drawer with it. */
  const setView = (next: SessionWindowViewMode) => {
    if (next === "list" && drawerOpen) drawer.close();
    view.setView(next);
  };

  return {
    view,
    setView,
    order,
    board,
    lifecycle,
    actions,
    openForAgent,
    onOpen,
    openInDrawer,
    drawer,
  };
}
