"use client";

/**
 * A line that types itself. The HTML serves the whole text; a line still below the fold at
 * hydration clears out of sight and types when it scrolls into view. Reduced motion shows it whole.
 */
import { m, useTransform } from "motion/react";

import { useArrival } from "@/components/site/hooks/use-arrival";

/** Seconds per character. */
const KEYSTROKE = 0.024;

export function Typed({ text, className }: { text: string; className?: string }) {
  const { ref, progress } = useArrival<HTMLSpanElement>(
    { duration: text.length * KEYSTROKE, ease: "linear" },
    "-15% 0px"
  );
  const shown = useTransform(progress, value => text.slice(0, Math.round(value * text.length)));
  return (
    <span ref={ref} className={className}>
      <span className="sr-only">{text}</span>
      <span aria-hidden>
        <m.span>{shown}</m.span>
        <span className="ml-0.5 inline-block h-[1em] w-[0.55em] translate-y-[0.15em] bg-brand-strong [animation:landing-caret_1s_steps(2)_infinite] motion-reduce:[animation:none]" />
      </span>
    </span>
  );
}
