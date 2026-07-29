import { useEffect, useRef, useState } from "react";

import { cn } from "@/lib/utils";
import { usePrefersReducedMotion } from "./hooks/use-prefers-reduced-motion";
import { formatWorkingElapsed } from "./session-working-row.logic";
import type { SessionWorkingRow } from "./session-timeline.logic";

// Self-ticking elapsed label: the span mutates its own text node on a one-second
// interval, so a streaming turn never forces a React commit per second — the
// whole transcript tree stays put while the clock advances (proven by the
// render-count probe in the thread suite).
function WorkingTimer({ startedAt }: { startedAt: number }) {
  const textRef = useRef<HTMLSpanElement>(null);
  const [initialText] = useState(() => formatWorkingElapsed(startedAt, Date.now()));

  useEffect(() => {
    const update = () => {
      if (textRef.current) {
        textRef.current.textContent = formatWorkingElapsed(startedAt, Date.now());
      }
    };
    update();
    const id = window.setInterval(update, 1000);
    return () => window.clearInterval(id);
  }, [startedAt]);

  return (
    <span ref={textRef} data-testid="session-working-timer" className="tabular-nums">
      {initialText}
    </span>
  );
}

const WORKING_DOT_DELAYS = [
  "[animation-delay:0ms]",
  "[animation-delay:200ms]",
  "[animation-delay:400ms]",
];

/**
 * Presentational streaming indicator, split from the row wrapper so Storybook can
 * render both the motion and reduced-motion variants without touching `matchMedia`.
 *
 * Motion: three 4px dots on a stepped 2s duty cycle beside a live
 * "Working for Xs" `tabular-nums` timer — the smallest, dimmest text on the
 * surface. Reduced motion degrades to a resting label — no dots, no ticking
 * timer — so no animation class reaches the DOM.
 */
export function WorkingIndicator({
  startedAt,
  reducedMotion,
}: {
  startedAt?: number;
  reducedMotion: boolean;
}) {
  if (reducedMotion) {
    return (
      <div
        role="status"
        aria-label="Working"
        data-testid="session-working-row"
        className="flex min-h-[22px] items-center gap-2 px-1 py-0.5 text-[11.5px] text-subtle"
      >
        <span>Working…</span>
      </div>
    );
  }

  return (
    <div
      role="status"
      aria-label="Working"
      data-testid="session-working-row"
      className="flex min-h-[22px] items-center gap-2 px-1 py-0.5 text-[11.5px] text-subtle"
    >
      <span aria-hidden="true" className="flex items-center gap-[3px]">
        {WORKING_DOT_DELAYS.map(delay => (
          <span
            key={delay}
            className={cn("session-working-dot size-1 rounded-full bg-faint", delay)}
          />
        ))}
      </span>
      <span aria-hidden="true" className="tabular-nums">
        {startedAt !== undefined ? (
          <>
            Working for <WorkingTimer startedAt={startedAt} />
          </>
        ) : (
          "Working…"
        )}
      </span>
    </div>
  );
}

export function SessionWorkingRowView({ row }: { row: SessionWorkingRow }) {
  const reducedMotion = usePrefersReducedMotion();
  return <WorkingIndicator startedAt={row.startedAt} reducedMotion={reducedMotion} />;
}
