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
  /**
   * Lifecycle actions for the focused window, routed through the routing
   * coordinator so the URL follows successor focus (Safety Invariant 13).
   * Null when no window is focused or commands are unavailable.
   */
  focusedWindowActions: OsFocusedWindowActions | null;
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
 * Window and layout affordances for surfaces that act on the focused window
 * directly — the zoom menu and the window chrome.
 *
 * Command *invocation* no longer flows through here: palette rows, menubar
 * items and chords all go through the one dispatch seam against the registry.
 * What remains is the enablement state those direct-manipulation controls need.
 */
export function useOsWindowCommands(): OsWindowCommandsModel {
  const { coordinator, manager } = useOsShell();
  const focusedWindow = useDesktop(state =>
    state.presentation === "floating" && state.focusedId !== null
      ? state.windows[state.focusedId]
      : undefined
  );
  const focusedId = useDesktop(state => state.focusedId);
  const presentation = useDesktop(state => state.presentation);
  const desktopCount = useDesktop(state => state.desktops.length);
  const commandsAvailable = useDesktop(windowManagerCommandsAvailable);
  const hasFocusedWindow = commandsAvailable && focusedId !== null;
  const focusedWindowActions: OsFocusedWindowActions | null =
    !commandsAvailable || focusedId === null
      ? null
      : {
          close: () => void coordinator.userClose(focusedId),
          minimize: () => void coordinator.userMinimize(focusedId),
          zoom: presentation === "floating" ? () => void manager.zoomWindow(focusedId) : null,
          makeFloating:
            focusedWindow && focusedWindow.placement !== "floating"
              ? () => manager.toggleFloating(focusedId)
              : null,
        };

  return {
    commandsAvailable,
    focusedWindowActions,
    canToggleFloating: hasFocusedWindow,
    toggleFloating: () => {
      if (focusedId !== null) manager.toggleFloating(focusedId);
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
