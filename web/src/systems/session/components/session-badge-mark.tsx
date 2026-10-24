import type * as React from "react";

import { StateGlyph } from "@compozy/ui";

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
}

/** Row-scale mark: the state glyph + an accessible state token. */
export function SessionBadgeMark({ badge, waitingOn, className, ...props }: SessionBadgeMarkProps) {
  const signal = sessionBadgeSignal(badge, waitingOn);
  return (
    <span
      role="img"
      aria-label={`Session badge: ${signal.label}`}
      data-badge={signal.label}
      data-waiting-on={signal.label === "waiting" ? (waitingOn ?? "sessions") : undefined}
      className={cn("grid size-3 shrink-0 place-items-center", className)}
      {...props}
    >
      <StateGlyph size="sm" state={signal.state} />
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
