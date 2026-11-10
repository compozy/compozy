"use client";

import { useIsDesktop } from "@/components/site/hooks/use-is-desktop";
import { useReducedMotionSafe } from "@/components/site/hooks/use-reduced-motion-safe";

/** Scroll-driven sequences only run where there is room and motion is welcome. */
export function usePinning() {
  const desktop = useIsDesktop();
  const reduced = useReducedMotionSafe();
  return desktop && !reduced;
}
