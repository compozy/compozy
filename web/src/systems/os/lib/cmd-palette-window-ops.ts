import {
  currentState,
  type PaletteClientOpContext,
  type PaletteClientOpHandler,
} from "./cmd-palette-client-op-context";
import { orderedDesktops } from "./desktop-order";
import type { OsArrangePreset } from "./os-types";
import { leadWindowId, resolveActiveTabId } from "./tab-model";

/**
 * Window, tab, desktop and layout operations, keyed by the command id the
 * daemon serves as `action.op`.
 *
 * This is the whole of the retired `dispatchWindowManagerAction` if-chain,
 * re-expressed as a lookup so a new command is a registry entry plus one
 * handler rather than another branch nobody can enumerate.
 */
const TAB_JUMP_SLOTS = [1, 2, 3, 4, 5, 6, 7, 8] as const;
const DESKTOP_SWITCH_SLOTS = [1, 2, 3, 4, 5, 6, 7, 8, 9] as const;
const ARRANGE_PRESETS: readonly OsArrangePreset[] = ["two-up", "grid", "main-stack", "columns"];
/** `window.tile.*` keeps its ids and now moves the focused window to that root edge of its tab. */
const TILE_EDGES = ["left", "right", "top", "bottom"] as const;
/** `window.swap.*` trades the focused window's place with its on-screen neighbour on that side. */
export const SWAP_DIRECTIONS = ["left", "right", "up", "down"] as const;

function focusedWindowId(context: PaletteClientOpContext): string | null {
  return currentState(context).focusedId;
}

/** Activates a tab of the active desktop's bar, chosen from the bar order and the active index. */
function activateBarTab(
  context: PaletteClientOpContext,
  resolve: (count: number, activeIndex: number) => number | undefined
): void {
  const state = currentState(context);
  const desktop = state.desktops.find(candidate => candidate.id === state.activeDesktopId);
  if (desktop === undefined || desktop.tabs.length === 0) return;
  const activeTabId = state.activeTabIds[desktop.id] ?? resolveActiveTabId(desktop, null);
  const activeIndex = Math.max(
    0,
    desktop.tabs.findIndex(tab => tab.id === activeTabId)
  );
  const index = resolve(desktop.tabs.length, activeIndex);
  const tab = index === undefined ? undefined : desktop.tabs[index];
  if (tab === undefined || tab.id === activeTabId) return;
  const lead = leadWindowId(tab, state.client?.focusOrder ?? []);
  if (lead !== null) context.shell.activateWindow(lead);
}

function focusLastWindow(context: PaletteClientOpContext): void {
  const state = currentState(context);
  const target = state.client?.focusOrder.find(
    windowId => windowId !== state.focusedId && state.windows[windowId] !== undefined
  );
  if (target) context.shell.activateWindow(target);
}

/** Sends the focused window to the desktop at `slot` (1-based, in desktop order). */
/**
 * Moves the focused window's tab to a desktop by id — the one move boundary behind
 * both the numbered slot commands and named destinations past the ninth slot.
 */
export function moveFocusedWindowToDesktopId(
  manager: PaletteClientOpContext["manager"],
  desktopId: string
): void {
  const state = manager.getState();
  const focusedId = state.focusedId;
  const window = focusedId === null ? undefined : state.windows[focusedId];
  if (!window || desktopId === window.desktopId) return;
  if (!state.desktops.some(desktop => desktop.id === desktopId)) return;
  manager.moveWindowToDesktop(window.id, desktopId);
}

function moveFocusedWindowToDesktop(slot: number): PaletteClientOpHandler {
  return context => {
    const target = orderedDesktops(currentState(context).desktops)[slot - 1];
    if (target) moveFocusedWindowToDesktopId(context.manager, target.id);
  };
}

function arrangePreset(preset: OsArrangePreset): PaletteClientOpHandler {
  return context => {
    const focusedId = focusedWindowId(context);
    if (focusedId !== null) context.manager.arrangeLayout(focusedId, preset);
  };
}

function withFocusedWindow(
  run: (context: PaletteClientOpContext, focusedId: string) => void
): PaletteClientOpHandler {
  return context => {
    const focusedId = focusedWindowId(context);
    if (focusedId !== null) run(context, focusedId);
  };
}

export const CMD_PALETTE_WINDOW_OPS: ReadonlyMap<string, PaletteClientOpHandler> = new Map<
  string,
  PaletteClientOpHandler
>([
  ["window.close", withFocusedWindow((context, id) => void context.manager.closeWindow(id))],
  [
    "window.nav.back",
    withFocusedWindow((context, id) => void context.manager.popWindowRoute(id).completion),
  ],
  ["window.focus.left", context => context.manager.focusDirection("left")],
  ["window.focus.right", context => context.manager.focusDirection("right")],
  ["window.focus.up", context => context.manager.focusDirection("up")],
  ["window.focus.down", context => context.manager.focusDirection("down")],
  ["window.focus.last", focusLastWindow],
  ...SWAP_DIRECTIONS.map(
    direction =>
      [
        `window.swap.${direction}`,
        (context: PaletteClientOpContext) => context.manager.swapDirection(direction),
      ] as const
  ),
  ["window.unsplit", withFocusedWindow((context, id) => context.manager.unsplitWindow(id))],
  ["window.tab.move_left", withFocusedWindow((context, id) => context.manager.moveTab(id, "left"))],
  [
    "window.tab.move_right",
    withFocusedWindow((context, id) => context.manager.moveTab(id, "right")),
  ],
  ...TILE_EDGES.map(
    edge =>
      [
        `window.tile.${edge}`,
        withFocusedWindow((context, id) => context.manager.tileWindow(id, edge)),
      ] as const
  ),
  ["window.tab.new", context => context.shell.openNewTab(focusedWindowId(context))],
  ["window.tab.reopen", context => void context.manager.reopenWindow().completion],
  ["window.tab.next", context => activateBarTab(context, (count, index) => (index + 1) % count)],
  [
    "window.tab.previous",
    context => activateBarTab(context, (count, index) => (index - 1 + count) % count),
  ],
  ["window.tab.last", context => activateBarTab(context, count => count - 1)],
  ["desktop.create", context => context.manager.createDesktop()],
  ["desktop.overview", context => context.shell.openDesktops()],
  ["desktop.switch.previous", context => context.manager.switchDesktopDirection("previous")],
  ["desktop.switch.next", context => context.manager.switchDesktopDirection("next")],
  ["layout.balance", context => context.manager.balanceFocusedLayout()],
  ["layout.undo", context => context.manager.undoLayout()],
  ["layout.redo", context => context.manager.redoLayout()],
  ...TAB_JUMP_SLOTS.map(
    slot =>
      [
        `window.tab.jump.${slot}`,
        (context: PaletteClientOpContext) => activateBarTab(context, () => slot - 1),
      ] as const
  ),
  ...DESKTOP_SWITCH_SLOTS.map(
    slot =>
      [
        `desktop.switch.${slot}`,
        (context: PaletteClientOpContext) => {
          const target = orderedDesktops(currentState(context).desktops)[slot - 1];
          if (target) context.manager.switchDesktop(target.id);
        },
      ] as const
  ),
  ...DESKTOP_SWITCH_SLOTS.map(
    slot => [`window.move_to_desktop.${slot}`, moveFocusedWindowToDesktop(slot)] as const
  ),
  ...ARRANGE_PRESETS.map(preset => [`layout.arrange.${preset}`, arrangePreset(preset)] as const),
]);
