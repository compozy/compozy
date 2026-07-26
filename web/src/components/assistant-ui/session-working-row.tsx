import { useEffect, useRef, useState } from "react";

import { TypingDots } from "@agh/ui";
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

/**
 * Presentational streaming indicator, split from the row wrapper so Storybook can
 * render both the motion and reduced-motion variants without touching `matchMedia`.
 *
 * Motion: typing dots (`@agh/ui` `TypingDots`, wiring the `typing-bounce`
 * keyframe on `bg-subtle` dots) beside a live "Working for Xs" `tabular-nums`
 * timer. Reduced motion degrades to a resting label — no `TypingDots`, no
 * ticking timer — so no animation class reaches the DOM.
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
        className="flex items-center gap-2 text-small-body text-muted"
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
      className="flex items-center gap-2 text-small-body text-muted"
    >
      <TypingDots />
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
