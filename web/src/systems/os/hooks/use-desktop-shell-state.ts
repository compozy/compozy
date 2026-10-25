import { shallowEqual } from "@xstate/store";

import { useDesktop } from "./use-desktop";

/** Selector-stable shell projection for wallpaper and mounted window hosts. */
export function useDesktopShellState() {
  const wallpaper = useDesktop(state => state.wallpaper);
  const windowIds = useDesktop(state => Object.keys(state.windows), shallowEqual);
  return { wallpaper, windowIds };
}
