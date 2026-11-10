"use client";

import { useInView, useMotionValue, useMotionValueEvent, useSpring } from "motion/react";
import { useEffect, useRef, useState } from "react";

import { useReducedMotionSafe } from "@/components/site/hooks/use-reduced-motion-safe";

/** Counts from zero to `value` once in view; reduced motion shows it settled after hydration. */
export function useCountUp(value: number) {
  const ref = useRef<HTMLSpanElement>(null);
  const inView = useInView(ref, { once: true, margin: "-10% 0px" });
  const reduced = useReducedMotionSafe();
  const raw = useMotionValue(0);
  const spring = useSpring(raw, { stiffness: 40, damping: 18 });
  const [shown, setShown] = useState(0);
  useEffect(() => {
    if (inView && !reduced) raw.set(value);
  }, [inView, reduced, raw, value]);
  useMotionValueEvent(spring, "change", next => setShown(Math.round(next)));
  return { ref, shown: reduced ? value : shown };
}
