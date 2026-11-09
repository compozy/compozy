import {
  useInView,
  useMotionValue,
  useMotionValueEvent,
  useReducedMotion,
  useSpring,
} from "motion/react";
import { useEffect, useRef, useState } from "react";

export function useCountUp(value: number) {
  const ref = useRef<HTMLSpanElement>(null);
  const inView = useInView(ref, { once: true, margin: "-10% 0px" });
  const reduced = useReducedMotion();
  const raw = useMotionValue(reduced ? value : 0);
  const spring = useSpring(raw, { stiffness: 40, damping: 18 });
  const [shown, setShown] = useState(reduced ? value : 0);
  useEffect(() => {
    if (inView) raw.set(value);
  }, [inView, raw, value]);
  useMotionValueEvent(spring, "change", next => setShown(Math.round(next)));
  return { ref, shown };
}
