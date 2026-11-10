"use client";

/**
 * Scroll and entrance motion for the marketing surfaces. Every effect settles to its final state
 * under reduced motion. Above-the-fold entrances (`Entrance`, `WordsIn`) are CSS, so the served
 * HTML shows its content with or without JavaScript. Scroll effects read the preference
 * hydration-safe: the first client render matches the server's HTML and the reduced state applies
 * right after, so no visitor hydrates different markup.
 */
import { cn } from "@compozy/ui";
import { m, useScroll, type MotionValue } from "motion/react";
import { Fragment, useRef, type CSSProperties, type ReactNode } from "react";

import { useCountUp } from "@/components/site/hooks/use-count-up";
import { useReducedMotionSafe } from "@/components/site/hooks/use-reduced-motion-safe";
import { useScrollTransform } from "@/components/site/hooks/use-scroll-transform";

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
      // The no-JavaScript fallback in `SiteShell` shows what this hides until it scrolls into view.
      data-site-reveal=""
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

type EnterStyle = CSSProperties & Record<`--site-enter-${string}`, string>;

function enterStyle({
  delay,
  y,
  blur,
  duration,
}: {
  delay: number;
  y: number;
  blur: number;
  duration: number;
}): EnterStyle {
  return {
    "--site-enter-delay": `${delay}s`,
    "--site-enter-y": `${y}px`,
    "--site-enter-blur": `${blur}px`,
    "--site-enter-duration": `${duration}s`,
  };
}

/**
 * `Reveal` for content above the fold: the same rise and focus, played by CSS from the first paint
 * instead of on scroll, so the served HTML is visible without JavaScript and before hydration.
 */
export function Entrance({
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
  return (
    <div
      className={cn("site-enter", className)}
      style={enterStyle({ delay, y, blur: 8, duration: 0.9 })}
    >
      {children}
    </div>
  );
}

/** A headline whose words arrive one by one, out of focus and into place, from the first paint. */
export function WordsIn({
  text,
  delay = 0,
  step = 0.06,
}: {
  text: string;
  delay?: number;
  step?: number;
}) {
  return (
    <>
      <span className="sr-only">{text}</span>
      <span aria-hidden>
        {text.split(" ").map((word, index) => (
          <Fragment key={`${word}-${index}`}>
            {index === 0 ? null : " "}
            <span
              className="site-enter inline-block"
              style={enterStyle({ delay: delay + index * step, y: 26, blur: 14, duration: 1 })}
            >
              {word}
            </span>
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

/** Serves `value` in the HTML; a stat that starts out of view counts up to it once scrolled in. */
export function CountUp({ value, className }: { value: number; className?: string }) {
  const { ref, text } = useCountUp(value);
  return (
    <m.span ref={ref} className={cn("tabular-nums", className)}>
      {text}
    </m.span>
  );
}

export function Marquee({
  children,
  seconds = 40,
  reverse = false,
  className,
}: {
  children: ReactNode;
  seconds?: number;
  reverse?: boolean;
  className?: string;
}) {
  return (
    <div
      className={cn(
        "group flex overflow-hidden [mask-image:linear-gradient(90deg,transparent,black_12%,black_88%,transparent)]",
        className
      )}
    >
      {[0, 1].map(copy => (
        <div
          key={copy}
          aria-hidden={copy === 1}
          className="flex shrink-0 items-center gap-3 pr-3 [animation:landing-marquee_var(--d)_linear_infinite] group-hover:[animation-play-state:paused] motion-reduce:[animation:none]"
          style={
            {
              "--d": `${seconds}s`,
              animationDirection: reverse ? "reverse" : "normal",
            } as CSSProperties
          }
        >
          {children}
        </div>
      ))}
    </div>
  );
}
