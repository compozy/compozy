/** Scroll and entrance motion for the landing. Every effect settles to its final state under reduced motion. */
import { cn } from "@compozy/ui";
import { m, useReducedMotion, useScroll, useTransform, type MotionValue } from "motion/react";
import { Fragment, useRef, type ReactNode } from "react";

import { useCountUp } from "./hooks/use-count-up";

const EASE = [0.16, 1, 0.3, 1] as const;

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
  const reduced = useReducedMotion();
  return (
    <m.div
      className={className}
      initial={reduced ? false : { opacity: 0, y, filter: "blur(8px)" }}
      whileInView={{ opacity: 1, y: 0, filter: "blur(0px)" }}
      viewport={{ once: true, margin: "-12% 0px" }}
      transition={{ duration: 0.9, delay, ease: EASE }}
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
  const reduced = useReducedMotion();
  return (
    <>
      <span className="sr-only">{text}</span>
      <span aria-hidden>
        {text.split(" ").map((word, index) => (
          <Fragment key={`${word}-${index}`}>
            {index === 0 ? null : " "}
            <m.span
              className="inline-block"
              initial={reduced ? false : { opacity: 0, y: 26, filter: "blur(14px)" }}
              animate={{ opacity: 1, y: 0, filter: "blur(0px)" }}
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
}: {
  word: string;
  index: number;
  total: number;
  progress: MotionValue<number>;
}) {
  const start = index / total;
  const opacity = useTransform(progress, [start, Math.min(1, start + 2.5 / total)], [0.25, 1]);
  return (
    <>
      {index === 0 ? null : " "}
      <m.span style={{ opacity }} className="inline-block">
        {word}
      </m.span>
    </>
  );
}

/** Copy that lights up word by word as it crosses the viewport. */
export function ScrollWords({ text, className }: { text: string; className?: string }) {
  const ref = useRef<HTMLSpanElement>(null);
  const { scrollYProgress } = useScroll({ target: ref, offset: ["start 92%", "end 62%"] });
  const reduced = useReducedMotion();
  if (reduced) {
    return (
      <span ref={ref} className={className}>
        {text}
      </span>
    );
  }
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
          />
        ))}
      </span>
    </span>
  );
}

export function CountUp({ value, className }: { value: number; className?: string }) {
  const { ref, shown } = useCountUp(value);
  return (
    <span ref={ref} className={cn("tabular-nums", className)}>
      {shown.toLocaleString("en-US")}
    </span>
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
            } as React.CSSProperties
          }
        >
          {children}
        </div>
      ))}
    </div>
  );
}
