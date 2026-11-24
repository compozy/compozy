import type * as React from "react";

import { StateGlyph, type StateGlyphState } from "@compozy/ui";

import { cn } from "@/lib/utils";

import { sessionBadgeSignal } from "../lib/session-badge";
import type { SessionWaitingOn } from "../lib/session-pull-request";

/**
 * The two rendered scales of the one badge dictionary
 * (`lib/session-badge.ts`), both drawn with the shared `StateGlyph`: a 12 px
 * mark for list rows and a 14 px mark on an 18 px footprint for bell rows,
 * toasts, palette rows, and the window status line. Both carry the exact state
 * token as an accessible label, so no state is conveyed by colour alone.
 */

export interface SessionBadgeMarkProps extends Omit<React.ComponentProps<"span">, "children"> {
  badge: string | null | undefined;
  /** Why a `waiting` session waits (the daemon's `waiting_on`); picks the parked glyph. */
  waitingOn?: SessionWaitingOn | null;
  /** A child session or subagent below the row is running (#1926). */
  descendantsRunning?: boolean;
}

/**
 * Escalations a running descendant never covers: needs you, failed and stuck keep
 * their tone. Every other own state (done, idle, stopped, waiting on sessions,
 * monitoring) lifts to `running`, so running work below always shows here; the
 * state word on line 2 still names the row's own state.
 */
const ESCALATION_STATES: ReadonlySet<StateGlyphState> = new Set(["attention", "failed"]);

/**
 * Row-scale mark: the state glyph + an accessible state token. The sidebar's one
 * "work is running" column, so a session with running descendants draws the running
 * glyph unless its own state escalates; its badge token stays its own.
 */
export function SessionBadgeMark({
  badge,
  waitingOn,
  descendantsRunning = false,
  className,
  ...props
}: SessionBadgeMarkProps) {
  const signal = sessionBadgeSignal(badge, waitingOn);
  const lifted =
    descendantsRunning && signal.state !== "running" && !ESCALATION_STATES.has(signal.state);
  return (
    <span
      role="img"
      aria-label={`Session badge: ${signal.label}${lifted ? ", working below" : ""}`}
      data-badge={signal.label}
      data-waiting-on={signal.label === "waiting" ? (waitingOn ?? "sessions") : undefined}
      data-descendants={lifted ? "running" : undefined}
      className={cn("grid size-3 shrink-0 place-items-center", className)}
      {...props}
    >
      <StateGlyph size="sm" state={lifted ? "running" : signal.state} />
    </span>
  );
}

export interface SessionBadgeGlyphProps extends Omit<React.ComponentProps<"span">, "children"> {
  badge: string | null | undefined;
  waitingOn?: SessionWaitingOn | null;
}

/** 18 px footprint — the bell, toast, palette, and status-line scale. */
export function SessionBadgeGlyph({
  badge,
  waitingOn,
  className,
  ...props
}: SessionBadgeGlyphProps) {
  const signal = sessionBadgeSignal(badge, waitingOn);
  return (
    <span
      role="img"
      aria-label={`Session badge: ${signal.label}`}
      data-badge={signal.label}
      className={cn("grid size-4.5 shrink-0 place-items-center", className)}
      {...props}
    >
      <StateGlyph state={signal.state} />
    </span>
  );
}
