import { shallowEqual } from "@xstate/store";

import { useDesktop } from "./use-desktop";

/** Selector-stable shell projection for wallpaper and mounted window hosts. */
export function useDesktopShellState() {
  return useDesktop(
    state => ({
      wallpaper: state.wallpaper,
      windows: state.windows,
    }),
    shallowEqual
  );
}
