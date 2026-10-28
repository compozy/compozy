import { useSyncExternalStore } from "react";
import { shallowEqual } from "@xstate/store";

import { useFontAppearance, useThemePreference } from "@/systems/theme";

import { getSystemReducedMotion, subscribeSystemReducedMotion } from "../lib/reduced-motion";
import { useDesktop } from "./use-desktop";
import { useOsShell } from "./use-os-shell";

/** View model for the Appearance settings pane and its desktop-doc actions. */
export function useAppearanceSettingsPane() {
  const { manager } = useOsShell();
  const desktop = useDesktop(
    state => ({
      reduceMotion: state.reduceMotion,
      setReduceMotion: manager.setReduceMotion,
      setWallpaper: manager.setWallpaper,
      wallpaper: state.wallpaper,
    }),
    shallowEqual
  );
  const systemReducedMotion = useSyncExternalStore(
    subscribeSystemReducedMotion,
    getSystemReducedMotion,
    () => false
  );

  const { preference: theme, setPreference: setTheme } = useThemePreference();
  const { appearance: fonts, setFont } = useFontAppearance();

  return { ...desktop, fonts, setFont, setTheme, systemReducedMotion, theme };
}
