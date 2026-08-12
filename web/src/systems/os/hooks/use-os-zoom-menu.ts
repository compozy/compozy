import { useEffect } from "react";
import type { PointerEvent as ReactPointerEvent } from "react";
import { useSelector, useStore } from "@xstate/store-react";

import type { OsArrangePreset } from "../lib/os-types";
import { arrangePeerWindows } from "../lib/window-manager-navigation";
import { dispatchWindowPlacement } from "../lib/window-manager-action-dispatch";
import type { WindowPlacementCommand } from "../lib/window-manager-command-registry";
import { windowManagerCommandsAvailable } from "../lib/window-manager-command-availability";
import { useDesktop } from "./use-desktop";
import { useOsShell } from "./use-os-shell";
import {
  OS_ZOOM_MENU_CLOSE_GRACE_MS,
  OS_ZOOM_MENU_OPEN_DELAY_MS,
  osZoomMenuLogic,
} from "./os-zoom-menu-store";

export { OS_ZOOM_MENU_CLOSE_GRACE_MS, OS_ZOOM_MENU_OPEN_DELAY_MS };

export interface OsZoomMenuModel {
  open: boolean;
  onOpenChange(open: boolean): void;
  onHoverEnter(event: ReactPointerEvent<HTMLElement>): void;
  onHoverLeave(): void;
  onContentEnter(): void;
  floating: boolean;
  placementEnabled: boolean;
  /** Arrange presets need at least one other visible window (truthful UI). */
  arrangeEnabled: boolean;
  dispatchPlacement(command: WindowPlacementCommand): void;
  dispatchMakeFloating(): void;
  dispatchFill(): void;
  dispatchArrange(preset: OsArrangePreset): void;
}

/**
 * Zoom-menu behavior: hover intent (250ms open / 300ms close grace, mouse
 * pointers only — touch never hover-opens), dispatches against THIS window
 * (not the focused one), and close-on-dispatch. Click on the zoom button
 * stays `toggleZoom`; the menu is the pointer-affordance mirror of the
 * palette/chords surface, which remains the guaranteed keyboard path.
 */
export function useOsZoomMenu(windowId: string): OsZoomMenuModel {
  const { manager } = useOsShell();
  const store = useStore(osZoomMenuLogic);
  const phase = useSelector(store, snapshot => snapshot.context.phase);
  const open = phase === "open" || phase === "closing";
  const floating = useDesktop(state => state.windows[windowId]?.placement === "floating");
  const commandsAvailable = useDesktop(windowManagerCommandsAvailable);
  const placementEnabled = useDesktop(
    state => windowManagerCommandsAvailable(state) && state.windowManagerConfig !== null
  );
  const arrangeEnabled = useDesktop(
    state =>
      windowManagerCommandsAvailable(state) &&
      arrangePeerWindows(state.windows, windowId).length > 0
  );

  useEffect(() => () => store.trigger.disposed(), [store]);

  const dispatch = (action: () => void) => {
    store.trigger.dismissed();
    action();
  };

  return {
    open,
    floating,
    placementEnabled,
    arrangeEnabled,
    onOpenChange: next => {
      store.trigger.openChanged({ open: next });
    },
    onHoverEnter: event => {
      if (event.pointerType !== "mouse") return;
      store.trigger.hoverEntered();
    },
    onHoverLeave: () => store.trigger.hoverLeft(),
    onContentEnter: () => store.trigger.contentEntered(),
    dispatchPlacement: command =>
      dispatch(() => {
        const state = manager.getState();
        if (windowManagerCommandsAvailable(state) && state.windowManagerConfig !== null) {
          dispatchWindowPlacement(manager, windowId, command);
        }
      }),
    dispatchMakeFloating: () =>
      dispatch(() => {
        if (commandsAvailable) manager.toggleFloating(windowId);
      }),
    dispatchFill: () =>
      dispatch(() => {
        if (commandsAvailable) manager.zoomWindow(windowId);
      }),
    dispatchArrange: preset =>
      dispatch(() => {
        if (commandsAvailable) manager.arrangeLayout(windowId, preset);
      }),
  };
}
