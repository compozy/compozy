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

interface WindowManagerActionContext {
  manager: WindowManagerController;
  state: OsDesktopRuntimeStore;
  openDesktops: () => void;
  /** Tab activation and new-tab creation own one URL write each (US-020). */
  activateWindow: (windowId: string) => void;
  openNewTab: (stackTargetWindowId: string | null) => void;
}

/**
 * Tab shortcut targets resolve against the focused frame's deck order:
 * next/previous cycle from the active tab with wrap, jumps are positional,
 * and every action quietly no-ops on a single-tab frame (US-020 AC-3).
 */
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
  if (target !== undefined && target !== frame.activeWindowId) {
    context.activateWindow(target);
  }
}

export function dispatchWindowManagerAction(
  actionId: WindowManagerActionId,
  context: WindowManagerActionContext
): void {
  const { manager, state } = context;
  const focusedId = state.focusedId;
  if (actionId === "desktop.overview") return context.openDesktops();
  if (actionId === "window.tab.new") {
    context.openNewTab(focusedId);
    return;
  }
  if (actionId === "window.tab.reopen") {
    void manager.reopenWindow().completion;
    return;
  }
  if (actionId === "desktop.switch.previous") return manager.switchDesktopDirection("previous");
  if (actionId === "desktop.switch.next") return manager.switchDesktopDirection("next");
  if (actionId === "layout.undo") return manager.undoLayout();
  if (actionId === "layout.redo") return manager.redoLayout();
  if (actionId.startsWith("window.focus.")) {
    return manager.focusDirection(
      actionId.slice("window.focus.".length) as "left" | "right" | "up" | "down"
    );
  }
  if (focusedId === null) return;
  if (actionId.startsWith("window.tab.")) {
    return dispatchTabAction(actionId, focusedId, context);
  }
  if (actionId === "window.close") {
    void manager.closeWindow(focusedId);
    return;
  }
  if (actionId === "window.minimize") {
    void manager.minimizeWindow(focusedId);
    return;
  }
  if (actionId === "window.zoom") {
    void manager.zoomWindow(focusedId);
    return;
  }
  if (actionId === "window.toggle_floating") {
    void manager.toggleFloating(focusedId).completion;
    return;
  }
  if (actionId === "layout.balance") return manager.balanceFocusedLayout();
  const placement = WINDOW_PLACEMENT_COMMANDS.find(command => command.id === actionId);
  if (placement && state.windowManagerConfig) {
    return dispatchWindowPlacement(manager, focusedId, placement);
  }
  const arrangement = WINDOW_ARRANGE_COMMANDS.find(command => command.id === actionId);
  if (arrangement) manager.arrangeLayout(focusedId, arrangement.preset);
}
