import { TypingDots } from "@compozy/ui";
import { useSecondClock } from "@/hooks/use-second-clock";
import { formatWorkingElapsed } from "./session-working-row.logic";
import type { SessionWorkingRow } from "./session-timeline.logic";

function WorkingTimer({ enabled, startedAt }: { enabled: boolean; startedAt: number }) {
  const now = useSecondClock(enabled);
  return (
    <span data-testid="session-working-timer" className="tabular-nums">
      {formatWorkingElapsed(startedAt, now)}
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
  liveDataEnabled = true,
}: {
  startedAt?: number;
  reducedMotion: boolean;
  liveDataEnabled?: boolean;
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
            Working for <WorkingTimer enabled={liveDataEnabled} startedAt={startedAt} />
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
