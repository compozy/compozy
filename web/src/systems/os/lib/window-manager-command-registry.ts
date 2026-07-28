import type { OsArrangePreset } from "./os-types";
import type { SnapCorner, SnapSide } from "./snap-targets";

export type WindowPlacementId = SnapSide | SnapCorner;
export type WindowManagerActionId =
  | `window.tile.${WindowPlacementId}`
  | "window.close"
  | "window.minimize"
  | "window.zoom"
  | "window.toggle_floating"
  | "window.focus.left"
  | "window.focus.right"
  | "window.focus.up"
  | "window.focus.down"
  | "desktop.switch.previous"
  | "desktop.switch.next"
  | "desktop.overview"
  | "layout.arrange.two-up"
  | "layout.arrange.grid"
  | "layout.balance"
  | "layout.undo"
  | "layout.redo";

export interface WindowPlacementCommand {
  id: `window.tile.${WindowPlacementId}`;
  placement: WindowPlacementId;
  label: string;
}

export interface WindowArrangeCommand {
  id: `layout.arrange.${OsArrangePreset}`;
  preset: OsArrangePreset;
  label: string;
}

export interface WindowManagerActionDefinition {
  id: WindowManagerActionId;
  label: string;
  defaultChord?: string;
  needsFocusedWindow?: boolean;
}

export const WINDOW_PLACEMENT_COMMANDS: readonly WindowPlacementCommand[] = [
  { id: "window.tile.left", placement: "left", label: "Tile left half" },
  { id: "window.tile.right", placement: "right", label: "Tile right half" },
  { id: "window.tile.top", placement: "top", label: "Tile top half" },
  { id: "window.tile.bottom", placement: "bottom", label: "Tile bottom half" },
  {
    id: "window.tile.top-left",
    placement: "top-left",
    label: "Tile top left quarter",
  },
  {
    id: "window.tile.top-right",
    placement: "top-right",
    label: "Tile top right quarter",
  },
  {
    id: "window.tile.bottom-left",
    placement: "bottom-left",
    label: "Tile bottom left quarter",
  },
  {
    id: "window.tile.bottom-right",
    placement: "bottom-right",
    label: "Tile bottom right quarter",
  },
];

export const WINDOW_ARRANGE_COMMANDS: readonly WindowArrangeCommand[] = [
  {
    id: "layout.arrange.two-up",
    preset: "two-up",
    label: "Arrange left & right",
  },
  { id: "layout.arrange.grid", preset: "grid", label: "Arrange in grid" },
];

export const WINDOW_MANAGER_ACTIONS: readonly WindowManagerActionDefinition[] = [
  {
    id: "window.close",
    label: "Close window",
    defaultChord: "meta+KeyW",
    needsFocusedWindow: true,
  },
  {
    id: "window.minimize",
    label: "Minimize window",
    defaultChord: "meta+KeyM",
    needsFocusedWindow: true,
  },
  {
    id: "window.zoom",
    label: "Zoom window",
    defaultChord: "control+alt+ArrowUp",
    needsFocusedWindow: true,
  },
  {
    id: "window.toggle_floating",
    label: "Toggle floating",
    defaultChord: "control+alt+KeyF",
    needsFocusedWindow: true,
  },
  ...WINDOW_PLACEMENT_COMMANDS.map(command => ({
    ...command,
    defaultChord:
      command.placement === "left"
        ? "control+alt+ArrowLeft"
        : command.placement === "right"
          ? "control+alt+ArrowRight"
          : command.placement === "top-left"
            ? "control+alt+KeyU"
            : command.placement === "top-right"
              ? "control+alt+KeyI"
              : command.placement === "bottom-left"
                ? "control+alt+KeyJ"
                : command.placement === "bottom-right"
                  ? "control+alt+KeyK"
                  : undefined,
    needsFocusedWindow: true,
  })),
  {
    id: "window.focus.left",
    label: "Focus left",
    defaultChord: "control+ArrowLeft",
  },
  {
    id: "window.focus.right",
    label: "Focus right",
    defaultChord: "control+ArrowRight",
  },
  { id: "window.focus.up", label: "Focus up", defaultChord: "control+ArrowUp" },
  {
    id: "window.focus.down",
    label: "Focus down",
    defaultChord: "control+ArrowDown",
  },
  {
    id: "desktop.switch.previous",
    label: "Previous desktop",
    defaultChord: "control+alt+BracketLeft",
  },
  {
    id: "desktop.switch.next",
    label: "Next desktop",
    defaultChord: "control+alt+BracketRight",
  },
  {
    id: "desktop.overview",
    label: "Desktops overview",
    defaultChord: "meta+shift+KeyS",
  },
  ...WINDOW_ARRANGE_COMMANDS.map(command => ({
    ...command,
    needsFocusedWindow: true,
  })),
  {
    id: "layout.balance",
    label: "Balance layout",
    defaultChord: "control+alt+KeyB",
    needsFocusedWindow: true,
  },
  { id: "layout.undo", label: "Undo layout", defaultChord: "meta+KeyZ" },
  {
    id: "layout.redo",
    label: "Redo layout",
    defaultChord: "meta+shift+KeyZ",
  },
];

export const WINDOW_MANAGER_ACTION_IDS = new Set(WINDOW_MANAGER_ACTIONS.map(action => action.id));

export function isWindowManagerActionId(value: string): value is WindowManagerActionId {
  return WINDOW_MANAGER_ACTION_IDS.has(value as WindowManagerActionId);
}
