import { useSyncExternalStore } from "react";
import { useShallow } from "zustand/shallow";

import { getSystemReducedMotion, subscribeSystemReducedMotion } from "../lib/reduced-motion";
import { useDesktop } from "./use-desktop";

/** View model for the Appearance settings pane and its desktop-doc actions. */
export function useAppearanceSettingsPane() {
  const desktop = useDesktop(
    useShallow(state => ({
      dockMagnify: state.dockMagnify,
      reduceMotion: state.reduceMotion,
      setDockMagnify: state.setDockMagnify,
      setReduceMotion: state.setReduceMotion,
      setWallpaper: state.setWallpaper,
      wallpaper: state.wallpaper,
    }))
  );
  const systemReducedMotion = useSyncExternalStore(
    subscribeSystemReducedMotion,
    getSystemReducedMotion,
    () => false
  );

  return { ...desktop, systemReducedMotion };
}
