"use client";

import { useScroll } from "motion/react";
import { useRef } from "react";

import { useIsDesktop } from "@/components/site/hooks/use-is-desktop";
import { useReducedMotionSafe } from "@/components/landing/hooks/use-reduced-motion-safe";
import { useScrollTransform } from "@/components/landing/hooks/use-scroll-transform";

/** The hero's scroll choreography: the window straightens while the copy lifts away. */
export function useHeroMotion() {
  const ref = useRef<HTMLElement>(null);
  const reduced = useReducedMotionSafe();
  const desktop = useIsDesktop();
  const { scrollYProgress } = useScroll({ target: ref, offset: ["start start", "end start"] });
  const tilted = desktop && !reduced;
  const rotateX = useScrollTransform(scrollYProgress, [0, 0.38], tilted ? [24, 0] : [0, 0]);
  const scale = useScrollTransform(scrollYProgress, [0, 0.38], tilted ? [0.86, 1] : [1, 1]);
  const plateY = useScrollTransform(scrollYProgress, [0, 1], reduced ? [0, 0] : [0, 260]);
  const copyY = useScrollTransform(scrollYProgress, [0, 0.3], reduced ? [0, 0] : [0, -90]);
  const copyOpacity = useScrollTransform(scrollYProgress, [0, 0.22], reduced ? [1, 1] : [1, 0]);
  return { ref, reduced, desktop, rotateX, scale, plateY, copyY, copyOpacity };
}
