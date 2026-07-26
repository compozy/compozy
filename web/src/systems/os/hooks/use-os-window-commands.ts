import {
  WINDOW_ARRANGE_COMMANDS,
  WINDOW_PLACEMENT_COMMANDS,
  type WindowArrangeCommand,
  type WindowManagerActionId,
  type WindowPlacementCommand,
} from "../lib/window-manager-command-registry";
import { dispatchWindowPlacement } from "../lib/window-manager-action-dispatch";
import { resolveWindowManagerActions } from "../lib/window-manager-shortcuts";
import { windowManagerCommandsAvailable } from "../lib/window-manager-command-availability";
import type { FocusDirection } from "../lib/window-manager-types";
import { useDesktop } from "./use-desktop";
import { useOsShell } from "./use-os-shell";

export interface OsFocusedWindowActions {
  close(): void;
  minimize(): void;
  /** Null in compact presentation — zoom has no meaning in a stack. */
  zoom: (() => void) | null;
  /** Null unless the focused window is currently tiled. */
  makeFloating: (() => void) | null;
}

export interface OsWindowCommandsModel {
  /** The daemon fence: an authoritative snapshot plus a live registered client. */
  commandsAvailable: boolean;
  shortcutLabels: Readonly<Partial<Record<WindowManagerActionId, string>>>;
  /**
   * Lifecycle actions for the focused window, routed through the routing
   * coordinator so the URL follows successor focus (Safety Invariant 13).
   * Null when no window is focused or commands are unavailable.
   */
  focusedWindowActions: OsFocusedWindowActions | null;
  /** Placement presets; empty without a focused floating window (truthful UI). */
  placementCommands: readonly WindowPlacementCommand[];
  /** Arrange presets; empty without a second visible window (truthful UI). */
  arrangeCommands: readonly WindowArrangeCommand[];
  dispatchPlacement(command: WindowPlacementCommand): void;
  dispatchArrange(command: WindowArrangeCommand): void;
  canToggleFloating: boolean;
  toggleFloating(): void;
  canBalanceLayout: boolean;
  balanceLayout(): void;
  /** Undo/redo are daemon-mediated; the client holds no layout history. */
  canEditLayoutHistory: boolean;
  undoLayout(): void;
  redoLayout(): void;
  canFocusDirection: boolean;
  focusDirection(direction: FocusDirection): void;
  canSwitchDesktop: boolean;
  switchDesktop(direction: "previous" | "next"): void;
}

/**
 * The one window / layout / desktop command model. The ⌘K palette and the
 * menubar's Window menu both read it, so their enablement predicates and their
 * dispatch path cannot drift.
 *
 * Actions here run immediately; a caller that must dismiss its own surface
 * first (the palette) wraps them. The keyboard registry keeps its own
 * `dispatchWindowManagerAction` path, which still bypasses the coordinator for
 * close/minimize — a pre-existing divergence, tracked separately.
 */
export function useOsWindowCommands(): OsWindowCommandsModel {
  const { coordinator, manager, store } = useOsShell();
  const focusedWindow = useDesktop(state =>
    state.presentation === "floating" && state.focusedId !== null
      ? state.windows[state.focusedId]
      : undefined
  );
  const focusedId = useDesktop(state => state.focusedId);
  const presentation = useDesktop(state => state.presentation);
  const windowManagerConfig = useDesktop(state => state.windowManagerConfig);
  const desktopCount = useDesktop(state => state.desktops.length);
  const commandsAvailable = useDesktop(windowManagerCommandsAvailable);
  const hasArrangePeer = useDesktop(state => {
    if (state.presentation !== "floating" || state.focusedId === null) return false;
    for (const win of Object.values(state.windows)) {
      if (win.id !== state.focusedId && win.desktopId === state.activeDesktopId && !win.minimized) {
        return true;
      }
    }
    return false;
  });

  const shortcutLabels = Object.fromEntries(
    resolveWindowManagerActions(windowManagerConfig?.shortcuts ?? {}).flatMap(action =>
      action.shortcutLabel ? [[action.id, action.shortcutLabel] as const] : []
    )
  ) as Partial<Record<WindowManagerActionId, string>>;

  const hasFocusedWindow = commandsAvailable && focusedId !== null;
  const focusedWindowActions: OsFocusedWindowActions | null =
    !commandsAvailable || focusedId === null
      ? null
      : {
          close: () => void coordinator.userClose(focusedId),
          minimize: () => void coordinator.userMinimize(focusedId),
          zoom:
            presentation === "floating"
              ? () => void manager.getState().zoomWindow(focusedId)
              : null,
          makeFloating:
            focusedWindow && focusedWindow.placement !== "floating"
              ? () => manager.getState().toggleFloating(focusedId)
              : null,
        };

  return {
    commandsAvailable,
    shortcutLabels,
    focusedWindowActions,
    placementCommands:
      commandsAvailable && focusedWindow && windowManagerConfig ? WINDOW_PLACEMENT_COMMANDS : [],
    arrangeCommands: commandsAvailable && hasArrangePeer ? WINDOW_ARRANGE_COMMANDS : [],
    dispatchPlacement: command => {
      const state = store.getState();
      if (
        !windowManagerCommandsAvailable(state) ||
        state.focusedId === null ||
        state.windowManagerConfig === null
      ) {
        return;
      }
      dispatchWindowPlacement(manager, state.focusedId, command);
    },
    dispatchArrange: command => {
      const state = store.getState();
      if (!windowManagerCommandsAvailable(state) || state.focusedId === null) return;
      state.arrangeLayout(state.focusedId, command.preset);
    },
    canToggleFloating: hasFocusedWindow,
    toggleFloating: () => {
      if (focusedId !== null) manager.getState().toggleFloating(focusedId);
    },
    canBalanceLayout: hasFocusedWindow,
    balanceLayout: () => manager.balanceFocusedLayout(),
    canEditLayoutHistory: commandsAvailable,
    undoLayout: () => manager.undoLayout(),
    redoLayout: () => manager.redoLayout(),
    canFocusDirection: commandsAvailable,
    focusDirection: direction => manager.focusDirection(direction),
    canSwitchDesktop: commandsAvailable && desktopCount > 1,
    switchDesktop: direction => manager.switchDesktopDirection(direction),
  };
}
