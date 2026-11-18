import { useReducedMotion } from "motion/react";

import { useIsDesktop } from "./use-is-desktop";

/** Scroll-driven sequences only run where there is room and motion is welcome. */
export function usePinning() {
  const desktop = useIsDesktop();
  const reduced = useReducedMotion();
  return desktop && !reduced;
}
