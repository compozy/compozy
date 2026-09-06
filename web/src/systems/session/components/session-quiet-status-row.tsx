import { Clock } from "lucide-react";

import { useSecondClock } from "@/hooks/use-second-clock";

import { type SessionQuietWarning, sessionQuietWarningClock } from "../lib/session-quiet-warning";

export interface SessionQuietStatusRowProps {
  /** The daemon's quiet episode, as read from `supervision.quiet_warning`. */
  warning: SessionQuietWarning;
  /** Pauses the 1 Hz clock while the window is not live (background, detached). */
  liveDataEnabled?: boolean;
}

/**
 * The status row while a session is quiet: elapsed since the last work signal
 * and, when the daemon scheduled one, the time left before the inactivity
 * stop. Both clocks derive from the daemon's instants, never from a browser
 * stopwatch, so a reconnect recomputes rather than resets. Neutral ink with a
 * clock glyph: the row informs, the notice above carries the warning tone.
 * Absent the moment the daemon clears the episode (US-014.AC-3).
 */
export function SessionQuietStatusRow({
  warning,
  liveDataEnabled = true,
}: SessionQuietStatusRowProps) {
  const now = useSecondClock(liveDataEnabled);
  const clock = sessionQuietWarningClock(warning, now);
  return (
    <div
      aria-label="Quiet"
      className="flex min-h-transcript-row items-center gap-2 px-1 py-0.5 text-transcript-meta text-subtle tabular-nums"
      data-quiet-stop={warning.stopAtMs === null ? "off" : clock.stopDue ? "due" : "scheduled"}
      data-testid="session-quiet-status-row"
      role="status"
    >
      <Clock aria-hidden="true" className="size-3 shrink-0 text-faint" />
      <span>
        Quiet for{" "}
        <span className="font-medium text-muted" data-testid="session-quiet-elapsed">
          {clock.quietFor}
        </span>
        {clock.stopsIn !== null ? (
          <>
            <span aria-hidden="true" className="px-1.5 text-faint">
              ·
            </span>
            stops in{" "}
            <span className="font-medium text-muted" data-testid="session-quiet-remaining">
              {clock.stopsIn}
            </span>
          </>
        ) : clock.stopDue ? (
          <>
            <span aria-hidden="true" className="px-1.5 text-faint">
              ·
            </span>
            stop due
          </>
        ) : null}
      </span>
    </div>
  );
}
