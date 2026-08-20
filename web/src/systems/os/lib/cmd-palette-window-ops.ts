import {
  currentState,
  type PaletteClientOpContext,
  type PaletteClientOpHandler,
} from "./cmd-palette-client-op-context";
import { orderedDesktops } from "./desktop-order";
import { frameForWindow } from "./group-projection";
import type { OsArrangePreset } from "./os-types";
import type { SnapCorner, SnapSide } from "./snap-targets";

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
const PLACEMENTS: readonly (SnapSide | SnapCorner)[] = [
  "left",
  "right",
  "top",
  "bottom",
  "top-left",
  "top-right",
  "bottom-left",
  "bottom-right",
];
const ARRANGE_PRESETS: readonly OsArrangePreset[] = ["two-up", "grid"];

function focusedWindowId(context: PaletteClientOpContext): string | null {
  return currentState(context).focusedId;
}

function activateStackMember(
  context: PaletteClientOpContext,
  resolve: (members: readonly string[], activeIndex: number) => string | undefined
): void {
  const state = currentState(context);
  const focusedId = state.focusedId;
  if (focusedId === null) return;
  const frame = frameForWindow(state.frames, focusedId);
  if (frame === null || frame.members.length < 2) return;
  const activeIndex = Math.max(0, frame.members.indexOf(frame.activeWindowId));
  const target = resolve(frame.members, activeIndex);
  if (target !== undefined && target !== frame.activeWindowId) {
    context.shell.activateWindow(target);
  }
}

function focusLastWindow(context: PaletteClientOpContext): void {
  const state = currentState(context);
  const target = state.client?.focusOrder.find(
    windowId => windowId !== state.focusedId && !state.windows[windowId]?.minimized
  );
  if (target) context.shell.activateWindow(target);
}

/** Every visible peer on the focused desktop joins the focused window's frame. */
function mergeAllWindows(context: PaletteClientOpContext): void {
  const state = currentState(context);
  const activeDesktopId = state.activeDesktopId;
  const anchor = state.focusedId;
  if (activeDesktopId === null || anchor === null) return;
  const joiners = Object.values(state.windows)
    .filter(win => win.desktopId === activeDesktopId && win.id !== anchor && !win.minimized)
    .sort((left, right) => left.id.localeCompare(right.id))
    .map(win => win.id);
  if (joiners.length > 0) context.manager.groupWindows(anchor, joiners);
}

/** Floats the focused tab out of its stack and follows it. */
function detachFocusedTab(context: PaletteClientOpContext): void {
  const state = currentState(context);
  const focusedId = state.focusedId;
  if (focusedId === null) return;
  const frame = frameForWindow(state.frames, focusedId);
  if (frame === null || frame.members.length < 2) return;
  const outcome = context.manager.toggleFloating(focusedId);
  if (!outcome.accepted) return;
  void outcome.completion.then(applied => {
    if (applied) context.shell.activateWindow(focusedId);
  });
}

function tilePlacement(placement: SnapSide | SnapCorner): PaletteClientOpHandler {
  return context => {
    const state = currentState(context);
    if (state.focusedId === null || state.windowManagerConfig === null) return;
    context.manager.tileWindow(state.focusedId, placement);
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
  ["window.minimize", withFocusedWindow((context, id) => void context.manager.minimizeWindow(id))],
  ["window.zoom", withFocusedWindow((context, id) => void context.manager.zoomWindow(id))],
  [
    "window.toggle_floating",
    withFocusedWindow((context, id) => void context.manager.toggleFloating(id).completion),
  ],
  [
    "window.nav.back",
    withFocusedWindow((context, id) => void context.manager.popWindowRoute(id).completion),
  ],
  ["window.focus.left", context => context.manager.focusDirection("left")],
  ["window.focus.right", context => context.manager.focusDirection("right")],
  ["window.focus.up", context => context.manager.focusDirection("up")],
  ["window.focus.down", context => context.manager.focusDirection("down")],
  ["window.focus.last", focusLastWindow],
  ["window.merge_all", mergeAllWindows],
  ["window.tab.detach", detachFocusedTab],
  ["window.tab.new", context => context.shell.openNewTab(focusedWindowId(context))],
  ["window.tab.reopen", context => void context.manager.reopenWindow().completion],
  [
    "window.tab.next",
    context =>
      activateStackMember(context, (members, index) => members[(index + 1) % members.length]),
  ],
  [
    "window.tab.previous",
    context =>
      activateStackMember(
        context,
        (members, index) => members[(index - 1 + members.length) % members.length]
      ),
  ],
  ["window.tab.last", context => activateStackMember(context, members => members.at(-1))],
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
        (context: PaletteClientOpContext) =>
          activateStackMember(context, members => members[slot - 1]),
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
  ...PLACEMENTS.map(placement => [`window.tile.${placement}`, tilePlacement(placement)] as const),
  ...ARRANGE_PRESETS.map(preset => [`layout.arrange.${preset}`, arrangePreset(preset)] as const),
]);
