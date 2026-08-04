import { useEffect, useRef, useState } from "react";

import { TypingDots } from "@compozy/ui";
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
 * Motion: three 4px dots on a stepped 2s duty cycle beside a live
 * "Working for Xs" `tabular-nums` timer — the smallest, dimmest text on the
 * surface. Reduced motion removes the dots while retaining the same elapsed
 * runtime fact.
 */
export function WorkingIndicator({
  startedAt,
  reducedMotion,
}: {
  startedAt?: number;
  reducedMotion: boolean;
}) {
  return (
    <div
      role="status"
      aria-label="Working"
      data-testid="session-working-row"
      className="flex min-h-transcript-row items-center gap-2 px-1 py-0.5 text-transcript-meta text-subtle"
    >
      {reducedMotion ? null : (
        <TypingDots className="session-working-dots gap-transcript-meta-gap [&>span]:bg-faint" />
      )}
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
  // Timeline working rows still keep live turns from being folded. The visible
  // status is intentionally rendered once in SessionThread above the composer.
  void row;
  return null;
}
