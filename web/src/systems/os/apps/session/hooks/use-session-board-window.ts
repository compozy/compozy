import { useQueryClient } from "@tanstack/react-query";
import { useRef } from "react";

import { notifyUser } from "@/lib/user-feedback";
import {
  sessionDetailOptions,
  setSessionWindowView,
  useSessionCreateActions,
  useSessionsBoard,
  useSessionSidebarState,
  useSessionWindowOrder,
  useSessionWindowViewState,
  type SessionBoardPayload,
  type SessionWindowViewMode,
} from "@/systems/session";
import { ownerDataScope, useDataScope, workspaceSelectorForScope } from "@/systems/workspace";

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
 * the session list's lifecycle verbs. The View options are this window's, shared with its List.
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
  const sidebar = useSessionSidebarState();
  const board = useSessionsBoard({
    scope,
    hideChildren: view.hideChildren,
    sort: order.sort,
    groupBy: order.groupBy,
    showEmptyProjects: sidebar.showEmptyProjects,
    showAutomation: sidebar.showAutomation,
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

  const queryClient = useQueryClient();
  // The latest open wins: a slower child lookup never replaces a later choice.
  const openRequest = useRef(0);
  const onOpen = (session: SessionBoardPayload) => {
    const request = ++openRequest.current;
    const workspaceId = session.workspace_id ?? "";
    const open = (agentName: string) =>
      openInDrawer({ sessionId: session.id, agentName, workspaceId });
    if (session.agent_name.trim()) {
      open(session.agent_name);
      return;
    }
    // A rolled-up linked-work item names only its child session: read the child's agent so it
    // still opens in this drawer rather than in another window.
    const selector = workspaceSelectorForScope(ownerDataScope(workspaceId));
    void queryClient
      .ensureQueryData(sessionDetailOptions(selector, session.id))
      .then(child => {
        if (request === openRequest.current) open(child.agent_name);
      })
      .catch(() => {
        if (request !== openRequest.current) return;
        notifyUser({ message: "Couldn't open this session. Try again.", tone: "error" });
      });
  };
  /** List shows the drawer's session full size: leaving the board takes the drawer with it. */
  const setView = (next: SessionWindowViewMode) => {
    if (next === "list" && drawerOpen) drawer.close();
    view.setView(next);
  };

  return {
    view,
    setView,
    order,
    sidebar,
    board,
    lifecycle,
    actions,
    openForAgent,
    onOpen,
    openInDrawer,
    drawer,
  };
}
