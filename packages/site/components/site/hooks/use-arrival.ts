"use client";

import {
  animate,
  useInView,
  useMotionValue,
  type UseInViewOptions,
  type ValueAnimationTransition,
} from "motion/react";
import { useEffect, useRef } from "react";

import { useReducedMotionSafe } from "@/components/site/hooks/use-reduced-motion-safe";

/** Whether any part of the element is inside the viewport right now. */
export function onScreen(node: Element) {
  const { top, bottom } = node.getBoundingClientRect();
  return top < window.innerHeight && bottom > 0;
}

/**
 * Motion that plays the first time an element scrolls into view, served settled. `progress` reads
 * `1` in the server HTML and the first client render, so the content shows without JavaScript and
 * hydrates without a mismatch. An element still out of sight at hydration drops to `0` out of view
 * and plays to `1` with `transition` when it arrives; one already on screen, or under reduced
 * motion, stays settled.
 */
export function useArrival<T extends Element>(
  transition: ValueAnimationTransition<number>,
  margin: UseInViewOptions["margin"] = "0px"
) {
  const ref = useRef<T>(null);
  const inView = useInView(ref, { once: true, margin });
  const reduced = useReducedMotionSafe();
  const progress = useMotionValue(1);
  /** Decided once, at hydration: was the element out of sight, so it may play? */
  const plays = useRef<boolean | null>(null);

  useEffect(() => {
    const node = ref.current;
    if (!node) return;
    if (plays.current === null) plays.current = !onScreen(node);
    if (reduced || !plays.current) {
      progress.jump(1);
      return;
    }
    if (!inView) {
      progress.jump(0);
      return;
    }
    const controls = animate(progress, 1, transition);
    return () => controls.stop();
  }, [inView, reduced, progress, transition]);

  return { ref, progress };
}
