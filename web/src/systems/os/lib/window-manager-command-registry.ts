import type { OsArrangePreset } from "./os-types";
import type { SnapCorner, SnapSide } from "./snap-targets";

export type WindowPlacementId = SnapSide | SnapCorner;
export type WindowTabJumpSlot = 1 | 2 | 3 | 4 | 5 | 6 | 7 | 8;
export type DesktopSwitchSlot = 1 | 2 | 3 | 4 | 5 | 6 | 7 | 8 | 9;
export type WindowManagerActionSection =
  | "Shell"
  | "Window"
  | "Tiling"
  | "Tabs"
  | "Sessions"
  | "Desktops"
  | "Workspaces"
  | "Layout";

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
  | "window.focus.last"
  | "window.nav.back"
  | "window.tab.new"
  | "window.tab.next"
  | "window.tab.previous"
  | "window.tab.last"
  | "window.tab.reopen"
  | `window.tab.jump.${WindowTabJumpSlot}`
  | `desktop.switch.${DesktopSwitchSlot}`
  | "desktop.switch.previous"
  | "desktop.switch.next"
  | "desktop.create"
  | "desktop.overview"
  | "workspace.picker"
  | "workspace.cycle.previous"
  | "workspace.cycle.next"
  | "session.cycle.previous"
  | "session.cycle.next"
  | "session.focus.attention"
  | "sidebar.toggle"
  | "palette.open"
  | "palette.view.sessions"
  | "session.new"
  | "scope.global.toggle"
  | "shortcuts.cheatsheet"
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
  section: WindowManagerActionSection;
  needsFocusedWindow?: boolean;
  /** Shell-owned actions remain usable while the daemon window manager reconnects. */
  availabilityExempt?: boolean;
  /** Focused text controls keep every other global shortcut. */
  allowInEditable?: boolean;
}

export const WINDOW_PLACEMENT_COMMANDS: readonly WindowPlacementCommand[] = [
  { id: "window.tile.left", placement: "left", label: "Tile left half" },
  { id: "window.tile.right", placement: "right", label: "Tile right half" },
  { id: "window.tile.top", placement: "top", label: "Tile top half" },
  { id: "window.tile.bottom", placement: "bottom", label: "Tile bottom half" },
  { id: "window.tile.top-left", placement: "top-left", label: "Tile top left quarter" },
  { id: "window.tile.top-right", placement: "top-right", label: "Tile top right quarter" },
  { id: "window.tile.bottom-left", placement: "bottom-left", label: "Tile bottom left quarter" },
  {
    id: "window.tile.bottom-right",
    placement: "bottom-right",
    label: "Tile bottom right quarter",
  },
];

export const WINDOW_ARRANGE_COMMANDS: readonly WindowArrangeCommand[] = [
  { id: "layout.arrange.two-up", preset: "two-up", label: "Arrange left & right" },
  { id: "layout.arrange.grid", preset: "grid", label: "Arrange in grid" },
];

const TAB_JUMP_SLOTS = [1, 2, 3, 4, 5, 6, 7, 8] as const;
const DESKTOP_SWITCH_SLOTS = [1, 2, 3, 4, 5, 6, 7, 8, 9] as const;

/** UI metadata only. Binding truth is served by the daemon. */
export const WINDOW_MANAGER_ACTIONS: readonly WindowManagerActionDefinition[] = [
  {
    id: "palette.open",
    label: "Command palette",
    section: "Shell",
    availabilityExempt: true,
    allowInEditable: true,
  },
  {
    id: "palette.view.sessions",
    label: "Sessions palette view",
    section: "Shell",
    availabilityExempt: true,
  },
  {
    id: "session.new",
    label: "New session",
    section: "Shell",
    availabilityExempt: true,
    allowInEditable: true,
  },
  {
    id: "scope.global.toggle",
    label: "Global scope",
    section: "Shell",
    availabilityExempt: true,
  },
  {
    id: "window.nav.back",
    label: "Back",
    section: "Shell",
    needsFocusedWindow: true,
    availabilityExempt: true,
  },
  { id: "sidebar.toggle", label: "Toggle sidebar", section: "Shell" },
  {
    id: "shortcuts.cheatsheet",
    label: "This sheet",
    section: "Shell",
    availabilityExempt: true,
  },
  {
    id: "window.close",
    label: "Close window",
    section: "Window",
    needsFocusedWindow: true,
    allowInEditable: true,
  },
  { id: "window.minimize", label: "Minimize window", section: "Window", needsFocusedWindow: true },
  { id: "window.zoom", label: "Zoom window", section: "Window", needsFocusedWindow: true },
  {
    id: "window.toggle_floating",
    label: "Toggle floating",
    section: "Window",
    needsFocusedWindow: true,
  },
  { id: "window.focus.left", label: "Focus left", section: "Window" },
  { id: "window.focus.right", label: "Focus right", section: "Window" },
  { id: "window.focus.up", label: "Focus up", section: "Window" },
  { id: "window.focus.down", label: "Focus down", section: "Window" },
  { id: "window.focus.last", label: "Focus last window", section: "Window" },
  ...WINDOW_PLACEMENT_COMMANDS.map(command => ({
    ...command,
    section: "Tiling" as const,
    needsFocusedWindow: true,
  })),
  { id: "window.tab.new", label: "New tab", section: "Tabs", allowInEditable: true },
  {
    id: "window.tab.next",
    label: "Next tab",
    section: "Tabs",
    needsFocusedWindow: true,
    allowInEditable: true,
  },
  {
    id: "window.tab.previous",
    label: "Previous tab",
    section: "Tabs",
    needsFocusedWindow: true,
    allowInEditable: true,
  },
  {
    id: "window.tab.last",
    label: "Last tab",
    section: "Tabs",
    needsFocusedWindow: true,
    allowInEditable: true,
  },
  { id: "window.tab.reopen", label: "Reopen closed tab", section: "Tabs", allowInEditable: true },
  ...TAB_JUMP_SLOTS.map(slot => ({
    id: `window.tab.jump.${slot}` as const,
    label: `Go to tab ${slot}`,
    section: "Tabs" as const,
    needsFocusedWindow: true,
    allowInEditable: true,
  })),
  { id: "session.cycle.previous", label: "Previous session", section: "Sessions" },
  { id: "session.cycle.next", label: "Next session", section: "Sessions" },
  { id: "session.focus.attention", label: "Jump to attention", section: "Sessions" },
  {
    id: "desktop.switch.previous",
    label: "Previous desktop",
    section: "Desktops",
  },
  { id: "desktop.switch.next", label: "Next desktop", section: "Desktops" },
  { id: "desktop.create", label: "Create desktop", section: "Desktops" },
  { id: "desktop.overview", label: "Desktops overview", section: "Desktops" },
  ...DESKTOP_SWITCH_SLOTS.map(slot => ({
    id: `desktop.switch.${slot}` as const,
    label: `Switch to desktop ${slot}`,
    section: "Desktops" as const,
  })),
  { id: "workspace.picker", label: "Workspace picker", section: "Workspaces" },
  { id: "workspace.cycle.previous", label: "Previous workspace", section: "Workspaces" },
  { id: "workspace.cycle.next", label: "Next workspace", section: "Workspaces" },
  ...WINDOW_ARRANGE_COMMANDS.map(command => ({
    ...command,
    section: "Layout" as const,
    needsFocusedWindow: true,
  })),
  { id: "layout.balance", label: "Balance layout", section: "Layout", needsFocusedWindow: true },
  { id: "layout.undo", label: "Undo layout", section: "Layout" },
  { id: "layout.redo", label: "Redo layout", section: "Layout" },
];

export const WINDOW_MANAGER_ACTION_IDS = new Set(WINDOW_MANAGER_ACTIONS.map(action => action.id));

export function isWindowManagerActionId(value: string): value is WindowManagerActionId {
  return WINDOW_MANAGER_ACTION_IDS.has(value as WindowManagerActionId);
}
