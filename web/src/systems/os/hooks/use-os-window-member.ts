import { shallowEqual } from "@xstate/store";
import type { FocusEvent, PointerEvent } from "react";

import { useDesktop } from "./use-desktop";
import { useOsShell } from "./use-os-shell";
import { useWindowBinding } from "./use-window-binding";
import { useWindowLiveDataEnabled } from "./use-window-live-data-enabled";

/** The pane's shell handles, surface metadata, live-data gate, and data binding. */
export function useOsWindowMember(windowId: string) {
  const { coordinator, manager } = useOsShell();
  const win = useDesktop(state => {
    const window = state.windows[windowId];
    if (!window) return undefined;
    return {
      app: window.app,
      instanceKey: window.instanceKey,
      shown: window.shown,
      inSplit: window.inSplit,
    };
  }, shallowEqual);
  const focused = useDesktop(state => state.focusedId === windowId);
  const liveDataEnabled = useWindowLiveDataEnabled(windowId);
  const windowBinding = useWindowBinding(windowId);

  const activate = (target: EventTarget | null) => {
    if (manager.getState().focusedId === windowId) return;
    const element = target instanceof Element ? target : null;
    void coordinator.userFocus(windowId, { viaLink: Boolean(element?.closest("a[href]")) });
  };

  return {
    coordinator,
    win,
    focused,
    liveDataEnabled,
    windowBinding,
    handleFocusCapture: (event: FocusEvent<HTMLElement>) => activate(event.target),
    handlePointerDownCapture: (event: PointerEvent<HTMLElement>) => {
      if (manager.getState().windowManagerConfig?.focusPolicy === "click_directional") {
        activate(event.target);
      }
    },
    handlePointerEnter: () => {
      if (manager.getState().windowManagerConfig?.focusFollowsPointer) {
        void coordinator.userFocus(windowId);
      }
    },
    close: () => void coordinator.userClose(windowId),
    unsplit: () => manager.unsplitWindow(windowId),
  };
}
