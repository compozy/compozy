import { useReducedMotion, useScroll, useTransform } from "motion/react";
import { useRef } from "react";

import { useIsDesktop } from "./use-is-desktop";

/** The hero's scroll choreography: the window straightens while the copy lifts away. */
export function useHeroMotion() {
  const ref = useRef<HTMLElement>(null);
  const reduced = useReducedMotion();
  const desktop = useIsDesktop();
  const { scrollYProgress } = useScroll({ target: ref, offset: ["start start", "end start"] });
  const tilted = desktop && !reduced;
  const rotateX = useTransform(scrollYProgress, [0, 0.38], tilted ? [24, 0] : [0, 0]);
  const scale = useTransform(scrollYProgress, [0, 0.38], tilted ? [0.86, 1] : [1, 1]);
  const plateY = useTransform(scrollYProgress, [0, 1], reduced ? [0, 0] : [0, 260]);
  const copyY = useTransform(scrollYProgress, [0, 0.3], reduced ? [0, 0] : [0, -90]);
  const copyOpacity = useTransform(scrollYProgress, [0, 0.22], reduced ? [1, 1] : [1, 0]);
  return { ref, reduced, desktop, rotateX, scale, plateY, copyY, copyOpacity };
}
