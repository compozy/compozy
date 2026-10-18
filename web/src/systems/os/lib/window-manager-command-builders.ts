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
 * One participant per frame, in peer order: a lone window, or a deck named by
 * its active tab. The daemon arranges each named window's whole tab frame, so
 * a hidden tab stays in its deck and the anchor's own deck is never named twice.
 */
function framePeers(anchor: OsWindow, peers: readonly OsWindow[]): OsWindow[] {
  const claimedStacks = new Set<string>(anchor.stackId === null ? [] : [anchor.stackId]);
  const participants: OsWindow[] = [];
  for (const window of peers) {
    const stackId = window.stackId;
    if (stackId === null) {
      participants.push(window);
      continue;
    }
    if (claimedStacks.has(stackId)) continue;
    claimedStacks.add(stackId);
    participants.push(
      peers.find(member => member.stackId === stackId && member.stackActive) ?? window
    );
  }
  return participants;
}

/**
 * Participants per preset, anchor first: two-up pairs the anchor with one peer
 * frame and grid caps at four frames; main-stack and columns take every frame.
 */
function arrangeParticipants(
  anchor: OsWindow,
  peers: readonly OsWindow[],
  preset: OsArrangePreset
): string[] {
  const frames = framePeers(anchor, peers).map(window => window.id);
  switch (preset) {
    case "two-up":
      return [anchor.id, ...frames.slice(0, 1)];
    case "grid":
      return [anchor.id, ...frames.slice(0, 3)];
    case "main-stack":
    case "columns":
      return [anchor.id, ...frames];
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
      // Participants are frames, one name per deck: the daemon keeps each
      // named deck whole instead of pulling the named tab out of it.
      keep_frames: true,
    },
  };
}
