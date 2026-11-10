"use client";

/**
 * Hydration-safe versions of the shared marketing motion (`@/components/site/motion`). The shared
 * helpers branch on `useReducedMotion()` during render, so a reduced-motion visitor hydrates
 * different markup than the server sent. These render the server's markup first and settle to the
 * final state right after hydration. Delete them once the shared helpers are fixed.
 */
import { cn } from "@compozy/ui";
import { m, useScroll, type MotionValue } from "motion/react";
import { Fragment, useRef, type ReactNode } from "react";

import { useCountUpSafe } from "@/components/landing/hooks/use-count-up-safe";
import { useReducedMotionSafe } from "@/components/landing/hooks/use-reduced-motion-safe";
import { useScrollTransform } from "@/components/landing/hooks/use-scroll-transform";

const EASE = [0.16, 1, 0.3, 1] as const;
const SETTLED = { opacity: 1, y: 0, filter: "blur(0px)" };
const INSTANT = { duration: 0 };

export function Reveal({
  children,
  className,
  delay = 0,
  y = 28,
}: {
  children: ReactNode;
  className?: string;
  delay?: number;
  y?: number;
}) {
  const reduced = useReducedMotionSafe();
  return (
    <m.div
      className={className}
      initial={{ opacity: 0, y, filter: "blur(8px)" }}
      animate={reduced ? SETTLED : undefined}
      whileInView={SETTLED}
      viewport={{ once: true, margin: "-12% 0px" }}
      transition={reduced ? INSTANT : { duration: 0.9, delay, ease: EASE }}
    >
      {children}
    </m.div>
  );
}

/** A headline whose words arrive one by one, out of focus and into place. */
export function WordsIn({
  text,
  delay = 0,
  step = 0.06,
}: {
  text: string;
  delay?: number;
  step?: number;
}) {
  const reduced = useReducedMotionSafe();
  return (
    <>
      <span className="sr-only">{text}</span>
      <span aria-hidden>
        {text.split(" ").map((word, index) => (
          <Fragment key={`${word}-${index}`}>
            {index === 0 ? null : " "}
            <m.span
              key={reduced ? "settled" : "moving"}
              className="inline-block"
              initial={reduced ? false : { opacity: 0, y: 26, filter: "blur(14px)" }}
              animate={SETTLED}
              transition={{ duration: 1, delay: delay + index * step, ease: EASE }}
            >
              {word}
            </m.span>
          </Fragment>
        ))}
      </span>
    </>
  );
}

function ScrollWord({
  word,
  index,
  total,
  progress,
  reduced,
}: {
  word: string;
  index: number;
  total: number;
  progress: MotionValue<number>;
  reduced: boolean;
}) {
  const start = index / total;
  const opacity = useScrollTransform(
    progress,
    [start, Math.min(1, start + 2.5 / total)],
    [0.25, 1]
  );
  return (
    <>
      {index === 0 ? null : " "}
      <m.span style={{ opacity: reduced ? 1 : opacity }} className="inline-block">
        {word}
      </m.span>
    </>
  );
}

/** Copy that lights up word by word as it crosses the viewport; reduced motion shows it lit. */
export function ScrollWords({ text, className }: { text: string; className?: string }) {
  const ref = useRef<HTMLSpanElement>(null);
  const { scrollYProgress } = useScroll({ target: ref, offset: ["start 92%", "end 62%"] });
  const reduced = useReducedMotionSafe();
  const words = text.split(" ");
  return (
    <span ref={ref} className={className}>
      <span className="sr-only">{text}</span>
      <span aria-hidden>
        {words.map((word, index) => (
          <ScrollWord
            key={`${word}-${index}`}
            word={word}
            index={index}
            total={words.length}
            progress={scrollYProgress}
            reduced={reduced}
          />
        ))}
      </span>
    </span>
  );
}

/** Counts from zero to `value` once in view; reduced motion shows it settled after hydration. */
export function CountUp({ value, className }: { value: number; className?: string }) {
  const { ref, shown } = useCountUpSafe(value);
  return (
    <span ref={ref} className={cn("tabular-nums", className)}>
      {shown.toLocaleString("en-US")}
    </span>
  );
}
