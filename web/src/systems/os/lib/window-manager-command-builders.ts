import type { MoveWindowInput, OsArrangePreset, OsWindow, OsWindowRoute } from "./os-types";
import type {
  PixelRect,
  WindowManagerCommandInput,
  WindowManagerWindow,
} from "./window-manager-types";
import { normalizedRectToWire, pixelRectToNormalized } from "./window-manager-view";

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

const ARRANGEMENT_BY_PRESET = {
  "two-up": "horizontal",
  grid: "grid",
  "main-stack": "main_stack",
  columns: "horizontal",
} as const satisfies Record<OsArrangePreset, string>;

/**
 * Participants per preset, anchor first: two-up pairs the anchor with one peer
 * and grid caps at four; main-stack and columns take every visible peer (the
 * active member of each tab frame), so no hidden tab is pulled out of its deck.
 */
function arrangeParticipants(
  anchor: OsWindow,
  peers: readonly OsWindow[],
  preset: OsArrangePreset
): string[] {
  switch (preset) {
    case "two-up":
      return [anchor.id, ...peers.slice(0, 1).map(window => window.id)];
    case "grid":
      return [anchor.id, ...peers.slice(0, 3).map(window => window.id)];
    case "main-stack":
    case "columns":
      return [anchor.id, ...peers.flatMap(window => (window.stackActive ? [window.id] : []))];
  }
}

export function arrangeLayoutCommand(
  anchor: OsWindow,
  peers: readonly OsWindow[],
  preset: OsArrangePreset,
  groupId: string
): WindowManagerCommandInput | null {
  const participants = arrangeParticipants(anchor, peers, preset);
  if (participants.length < 2) return null;
  return {
    commandId: "layout.arrange",
    payload: {
      desktop_id: anchor.desktopId,
      window_ids: participants,
      arrangement: ARRANGEMENT_BY_PRESET[preset],
      frame: normalizedRectToWire({ x: 0, y: 0, w: 1, h: 1 }),
      group_id: groupId,
    },
  };
}
