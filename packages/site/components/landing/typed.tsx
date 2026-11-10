"use client";

/** Types its text the first time it scrolls into view; reduced motion shows it whole. */
import { useInView } from "motion/react";
import { useEffect, useRef, useState } from "react";
import { useReducedMotionSafe } from "@/components/site/hooks/use-reduced-motion-safe";

export function Typed({ text, className }: { text: string; className?: string }) {
  const ref = useRef<HTMLSpanElement>(null);
  const inView = useInView(ref, { once: true, margin: "-15% 0px" });
  const reduced = useReducedMotionSafe();
  const [count, setCount] = useState(0);
  useEffect(() => {
    if (!inView || reduced) return;
    const timer = window.setInterval(
      () => setCount(current => (current >= text.length ? current : current + 1)),
      24
    );
    return () => window.clearInterval(timer);
  }, [inView, reduced, text]);
  const shown = reduced ? text : text.slice(0, count);
  return (
    <span ref={ref} className={className}>
      <span className="sr-only">{text}</span>
      <span aria-hidden>
        {shown}
        <span className="ml-0.5 inline-block h-[1em] w-[0.55em] translate-y-[0.15em] bg-brand-strong [animation:landing-caret_1s_steps(2)_infinite] motion-reduce:[animation:none]" />
      </span>
    </span>
  );
}
