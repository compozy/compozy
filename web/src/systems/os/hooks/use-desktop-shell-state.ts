import { useShallow } from "zustand/shallow";

import { useDesktop } from "./use-desktop";

/** Selector-stable shell projection for wallpaper and mounted window hosts. */
export function useDesktopShellState() {
  return useDesktop(
    useShallow(state => ({
      wallpaper: state.wallpaper,
      windows: state.windows,
    }))
  );
}
