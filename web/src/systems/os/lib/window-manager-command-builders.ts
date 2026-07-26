import type {
  MoveWindowInput,
  OsArrangePreset,
  OsOpenTarget,
  OsWindow,
  OsWindowRoute,
} from "./os-types";
import type {
  PixelRect,
  WindowManagerCommandInput,
  WindowManagerWindow,
} from "./window-manager-types";
import {
  DEFAULT_WINDOW_MANAGER_FLOATING_RECT,
  defaultOsWindowRoute,
  normalizedRectToWire,
  pixelRectToNormalized,
} from "./window-manager-view";

export function openWindowCommand(
  target: OsOpenTarget,
  id: string,
  desktopId: string
): WindowManagerCommandInput {
  return {
    commandId: "window.open",
    payload: {
      window: {
        id,
        app: target.app,
        ...(target.instanceKey ? { instance_key: target.instanceKey } : {}),
        route: target.route ?? defaultOsWindowRoute(target.app),
        desktop_id: desktopId,
        floating_rect: normalizedRectToWire(DEFAULT_WINDOW_MANAGER_FLOATING_RECT),
        insert_tiled: false,
      },
    },
  };
}

export function restoreWindowCommand(
  window: WindowManagerWindow,
  route: OsWindowRoute = window.route
): WindowManagerCommandInput {
  return {
    commandId: "window.open",
    payload: {
      window: {
        id: window.id,
        app: window.app,
        ...(window.instanceKey ? { instance_key: window.instanceKey } : {}),
        route,
        desktop_id: window.desktopId,
        floating_rect: normalizedRectToWire(window.floatingRect),
        insert_tiled: false,
      },
      restore_window_id: window.id,
    },
  };
}

export function moveWindowCommand(
  id: string,
  input: MoveWindowInput,
  workArea: PixelRect
): WindowManagerCommandInput {
  const normalized = input.floatingRect
    ? pixelRectToNormalized(input.floatingRect, workArea)
    : undefined;
  return {
    commandId: "window.move",
    payload: {
      window_id: id,
      destination_desktop_id: input.destinationDesktopId,
      ...(input.targetWindowId ? { target_window_id: input.targetWindowId } : {}),
      placement: input.placement,
      ...(normalized ? { floating_rect: normalizedRectToWire(normalized) } : {}),
      move_group: input.moveGroup ?? false,
    },
  };
}

export function swapWindowsCommand(
  firstWindowId: string,
  secondWindowId: string
): WindowManagerCommandInput {
  return {
    commandId: "window.swap",
    payload: {
      first_window_id: firstWindowId,
      second_window_id: secondWindowId,
    },
  };
}

export function arrangeLayoutCommand(
  anchor: OsWindow,
  peers: readonly OsWindow[],
  preset: OsArrangePreset,
  groupId: string
): WindowManagerCommandInput | null {
  const participants =
    preset === "two-up"
      ? [anchor.id, ...peers.slice(0, 1).map(window => window.id)]
      : [anchor.id, ...peers.slice(0, 3).map(window => window.id)];
  if (participants.length < 2) return null;
  return {
    commandId: "layout.arrange",
    payload: {
      desktop_id: anchor.desktopId,
      window_ids: participants,
      arrangement: preset === "two-up" ? "horizontal" : "grid",
      frame: normalizedRectToWire({ x: 0, y: 0, w: 1, h: 1 }),
      group_id: groupId,
    },
  };
}
