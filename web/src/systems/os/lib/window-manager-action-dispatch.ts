import { orderedDesktops } from "./desktop-order";
import { frameForWindow } from "./group-projection";
import type { OsDesktopRuntimeStore, WindowManagerController } from "./os-types";
import {
  WINDOW_ARRANGE_COMMANDS,
  WINDOW_PLACEMENT_COMMANDS,
  type WindowManagerActionId,
  type WindowPlacementCommand,
} from "./window-manager-command-registry";

export function dispatchWindowPlacement(
  manager: WindowManagerController,
  windowId: string,
  command: WindowPlacementCommand
): void {
  manager.tileWindow(windowId, command.placement);
}

export interface WindowManagerActionContext {
  manager: WindowManagerController;
  state: OsDesktopRuntimeStore;
  openPalette: () => void;
  openPaletteSessions: () => void;
  openNewSession: () => void;
  toggleGlobalScope: () => void;
  openCheatsheet: () => void;
  openDesktops: () => void;
  openWorkspaces: () => void;
  cycleWorkspace: (direction: "previous" | "next") => void;
  cycleSession: (direction: "previous" | "next") => void;
  focusAttention: () => void;
  toggleSidebar: () => void;
  /** Tab/window activation owns one URL write (US-020). */
  activateWindow: (windowId: string) => void;
  openNewTab: (stackTargetWindowId: string | null) => void;
}

function dispatchTabAction(
  actionId: WindowManagerActionId,
  focusedId: string,
  context: WindowManagerActionContext
): void {
  const frame = frameForWindow(context.state.frames, focusedId);
  if (frame === null || frame.members.length < 2) return;
  const activeIndex = Math.max(0, frame.members.indexOf(frame.activeWindowId));
  let target: string | undefined;
  if (actionId === "window.tab.next") {
    target = frame.members[(activeIndex + 1) % frame.members.length];
  } else if (actionId === "window.tab.previous") {
    target = frame.members[(activeIndex - 1 + frame.members.length) % frame.members.length];
  } else if (actionId === "window.tab.last") {
    target = frame.members.at(-1);
  } else {
    const slot = Number(actionId.slice("window.tab.jump.".length));
    target = frame.members[slot - 1];
  }
  if (target !== undefined && target !== frame.activeWindowId) context.activateWindow(target);
}

function dispatchDesktopSlot(actionId: WindowManagerActionId, context: WindowManagerActionContext) {
  if (!actionId.startsWith("desktop.switch.")) return false;
  const slot = Number(actionId.slice("desktop.switch.".length));
  if (!Number.isInteger(slot)) return false;
  const target = orderedDesktops(context.state.desktops)[slot - 1];
  if (target) context.manager.switchDesktop(target.id);
  return true;
}

function focusLastWindow(context: WindowManagerActionContext): void {
  const target = context.state.client?.focusOrder.find(
    windowId => windowId !== context.state.focusedId && !context.state.windows[windowId]?.minimized
  );
  if (target) context.activateWindow(target);
}

export function dispatchWindowManagerAction(
  actionId: WindowManagerActionId,
  context: WindowManagerActionContext
): void {
  const { manager, state } = context;
  const focusedId = state.focusedId;
  if (actionId === "palette.open") return context.openPalette();
  if (actionId === "palette.view.sessions") return context.openPaletteSessions();
  if (actionId === "session.new") return context.openNewSession();
  if (actionId === "scope.global.toggle") return context.toggleGlobalScope();
  if (actionId === "shortcuts.cheatsheet") return context.openCheatsheet();
  if (actionId === "sidebar.toggle") return context.toggleSidebar();
  if (actionId === "desktop.overview") return context.openDesktops();
  if (actionId === "workspace.picker") return context.openWorkspaces();
  if (actionId === "session.focus.attention") return context.focusAttention();
  if (actionId === "session.cycle.previous") return context.cycleSession("previous");
  if (actionId === "session.cycle.next") return context.cycleSession("next");
  if (actionId === "workspace.cycle.previous") return context.cycleWorkspace("previous");
  if (actionId === "workspace.cycle.next") return context.cycleWorkspace("next");
  if (actionId === "desktop.create") return manager.createDesktop();
  if (actionId === "desktop.switch.previous") return manager.switchDesktopDirection("previous");
  if (actionId === "desktop.switch.next") return manager.switchDesktopDirection("next");
  if (dispatchDesktopSlot(actionId, context)) return;
  if (actionId === "window.focus.last") return focusLastWindow(context);
  if (actionId === "window.tab.new") return context.openNewTab(focusedId);
  if (actionId === "window.tab.reopen") return void manager.reopenWindow().completion;
  if (actionId === "layout.undo") return manager.undoLayout();
  if (actionId === "layout.redo") return manager.redoLayout();
  if (actionId === "window.nav.back") {
    if (focusedId !== null) void manager.popWindowRoute(focusedId).completion;
    return;
  }
  if (actionId.startsWith("window.focus.")) {
    return manager.focusDirection(
      actionId.slice("window.focus.".length) as "left" | "right" | "up" | "down"
    );
  }
  if (focusedId === null) return;
  if (actionId.startsWith("window.tab.")) return dispatchTabAction(actionId, focusedId, context);
  if (actionId === "window.close") return void manager.closeWindow(focusedId);
  if (actionId === "window.minimize") return void manager.minimizeWindow(focusedId);
  if (actionId === "window.zoom") return void manager.zoomWindow(focusedId);
  if (actionId === "window.toggle_floating") {
    void manager.toggleFloating(focusedId).completion;
    return;
  }
  if (actionId === "layout.balance") return manager.balanceFocusedLayout();
  const placement = WINDOW_PLACEMENT_COMMANDS.find(command => command.id === actionId);
  if (placement && state.windowManagerConfig)
    return dispatchWindowPlacement(manager, focusedId, placement);
  const arrangement = WINDOW_ARRANGE_COMMANDS.find(command => command.id === actionId);
  if (arrangement) manager.arrangeLayout(focusedId, arrangement.preset);
}
