import { useSyncExternalStore } from "react";

import { getSystemReducedMotion, subscribeSystemReducedMotion } from "../lib/reduced-motion";
import { useDesktop } from "./use-desktop";

/**
 * Motion collapse signal: the system `prefers-reduced-motion` preference OR
 * the desktop-doc in-product toggle (`desktop.reduceMotion`, agent-writable).
 */
export function useOsReducedMotion(): boolean {
  const docPreference = useDesktop(state => state.reduceMotion);
  const systemPreference = useSyncExternalStore(
    subscribeSystemReducedMotion,
    getSystemReducedMotion,
    () => false
  );
  return docPreference || systemPreference;
}
