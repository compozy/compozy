"use client";

import { useInView, useMotionValue, useSpring, useTransform } from "motion/react";
import { useEffect, useRef } from "react";

import { useReducedMotionSafe } from "@/components/site/hooks/use-reduced-motion-safe";

const NUMBER = new Intl.NumberFormat("en-US");

function onScreen(node: Element) {
  const { top, bottom } = node.getBoundingClientRect();
  return top < window.innerHeight && bottom > 0;
}

/**
 * A number that counts up once it scrolls into view. The server HTML and the first client render
 * carry `value`, so the figure reads right without JavaScript and hydrates without a mismatch. A
 * count still below the fold at hydration drops to zero out of sight and counts up when it
 * arrives; one already on screen, or under reduced motion, stays settled. `text` is a motion
 * value, so the count updates the DOM without re-rendering.
 */
export function useCountUp(value: number) {
  const ref = useRef<HTMLSpanElement>(null);
  const inView = useInView(ref, { once: true, margin: "-10% 0px" });
  const reduced = useReducedMotionSafe();
  const raw = useMotionValue(value);
  const spring = useSpring(raw, { stiffness: 40, damping: 18 });
  const text = useTransform(spring, latest => NUMBER.format(Math.round(latest)));
  /** Decided once, at hydration: was the count out of sight, so it may count up? */
  const counts = useRef<boolean | null>(null);

  useEffect(() => {
    const node = ref.current;
    if (!node) return;
    if (counts.current === null) counts.current = !onScreen(node);
    if (reduced || !counts.current) {
      raw.jump(value);
      spring.jump(value);
    } else if (inView) {
      raw.set(value);
    } else {
      raw.jump(0);
      spring.jump(0);
    }
  }, [inView, reduced, value, raw, spring]);

  return { ref, text };
}
