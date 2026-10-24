import { shallowEqual } from "@xstate/store";

import { useDesktop } from "./use-desktop";
import { useOsShell } from "./use-os-shell";
import { useWindowBinding } from "./use-window-binding";
import { useWindowLiveDataEnabled } from "./use-window-live-data-enabled";

/** The member's shell handles, window record, live-data gate, and data binding. */
export function useOsWindowMember(windowId: string) {
  const { coordinator } = useOsShell();
  const win = useDesktop(state => {
    const window = state.windows[windowId];
    if (!window) return undefined;
    // Geometry belongs to the outer frame, not the retained app surface.
    return {
      app: window.app,
      instanceKey: window.instanceKey,
      minimized: window.minimized,
      placement: window.placement,
    };
  }, shallowEqual);
  const liveDataEnabled = useWindowLiveDataEnabled(windowId);
  const windowBinding = useWindowBinding(windowId);
  return { coordinator, win, liveDataEnabled, windowBinding };
}
