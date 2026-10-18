import type * as React from "react";

import { StateGlyph } from "@compozy/ui";

import { cn } from "@/lib/utils";

import { sessionBadgeSignal } from "../lib/session-badge";

/**
 * The two rendered scales of the one badge dictionary
 * (`lib/session-badge.ts`), both drawn with the shared `StateGlyph`: a 12 px
 * mark for list rows and a 14 px mark on an 18 px footprint for bell rows,
 * toasts, palette rows, and the window status line. Both carry the exact state
 * token as an accessible label, so no state is conveyed by colour alone.
 */

export interface SessionBadgeMarkProps extends Omit<React.ComponentProps<"span">, "children"> {
  badge: string | null | undefined;
  /** The turn ended but subagents still work: the parent reads `delegated`, not done. */
  delegated?: boolean;
}

/** Row-scale mark: the state glyph + an accessible state token. */
export function SessionBadgeMark({
  badge,
  delegated = false,
  className,
  ...props
}: SessionBadgeMarkProps) {
  const signal = sessionBadgeSignal(badge);
  const label = delegated ? "delegated" : signal.label;
  return (
    <span
      role="img"
      aria-label={`Session badge: ${label}`}
      data-badge={label}
      className={cn("grid size-3 shrink-0 place-items-center", className)}
      {...props}
    >
      <StateGlyph size="sm" state={delegated ? "delegated" : signal.state} />
    </span>
  );
}

export interface SessionBadgeGlyphProps extends Omit<React.ComponentProps<"span">, "children"> {
  badge: string | null | undefined;
}

/** 18 px footprint — the bell, toast, palette, and status-line scale. */
export function SessionBadgeGlyph({ badge, className, ...props }: SessionBadgeGlyphProps) {
  const signal = sessionBadgeSignal(badge);
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
