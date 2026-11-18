import { shallowEqual } from "@xstate/store";
import type { FocusEvent, PointerEvent, SyntheticEvent } from "react";

import { useDesktop } from "./use-desktop";
import { useOsShell } from "./use-os-shell";
import { useWindowBinding } from "./use-window-binding";
import { useWindowLiveDataEnabled } from "./use-window-live-data-enabled";

/**
 * Whether the event happened in the pane's own DOM. React propagates events through portals to
 * their React ancestors, so focus or a pointer in an overlay portaled outside the pane (to the
 * shell, as a hidden editor's unsaved-changes question is) still reaches the pane's handlers;
 * that is not the operator choosing this window.
 */
function inPane(event: SyntheticEvent<HTMLElement>): boolean {
  return event.target instanceof Node && event.currentTarget.contains(event.target);
}

/** The pane's shell handles, surface metadata, live-data gate, and data binding. */
export function useOsWindowMember(windowId: string) {
  const { coordinator, manager } = useOsShell();
  const win = useDesktop(state => {
    const window = state.windows[windowId];
    if (!window) return undefined;
    return {
      app: window.app,
      instanceKey: window.instanceKey,
      tabId: window.tabId,
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
    handleFocusCapture: (event: FocusEvent<HTMLElement>) => {
      if (inPane(event)) activate(event.target);
    },
    handlePointerDownCapture: (event: PointerEvent<HTMLElement>) => {
      if (!inPane(event)) return;
      if (manager.getState().windowManagerConfig?.focusPolicy === "click_directional") {
        activate(event.target);
      }
    },
    handlePointerEnter: (event: PointerEvent<HTMLElement>) => {
      if (!inPane(event)) return;
      if (manager.getState().windowManagerConfig?.focusFollowsPointer) {
        void coordinator.userFocus(windowId);
      }
    },
    close: () => void coordinator.userClose(windowId),
    unsplit: () => manager.unsplitWindow(windowId),
  };
}
