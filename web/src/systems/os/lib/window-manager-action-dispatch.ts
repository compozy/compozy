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

export function dispatchWindowManagerAction(
  actionId: WindowManagerActionId,
  context: {
    manager: WindowManagerController;
    state: OsDesktopRuntimeStore;
    openDesktops: () => void;
  }
): void {
  const { manager, state } = context;
  const focusedId = state.focusedId;
  if (actionId === "desktop.overview") return context.openDesktops();
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
  if (actionId === "window.close") {
    void state.closeWindow(focusedId);
    return;
  }
  if (actionId === "window.minimize") {
    void state.minimizeWindow(focusedId);
    return;
  }
  if (actionId === "window.zoom") {
    void state.zoomWindow(focusedId);
    return;
  }
  if (actionId === "window.toggle_floating") return state.toggleFloating(focusedId);
  if (actionId === "layout.balance") return manager.balanceFocusedLayout();
  const placement = WINDOW_PLACEMENT_COMMANDS.find(command => command.id === actionId);
  if (placement && state.windowManagerConfig) {
    return dispatchWindowPlacement(manager, focusedId, placement);
  }
  const arrangement = WINDOW_ARRANGE_COMMANDS.find(command => command.id === actionId);
  if (arrangement) state.arrangeLayout(focusedId, arrangement.preset);
}
