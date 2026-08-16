import type * as React from "react";

import { Icon, PillDot, StatusDot, type StatusDotTone } from "@compozy/ui";

import { cn } from "@/lib/utils";

import {
  sessionBadgeSignal,
  type SessionBadgeShape,
  type SessionBadgeSignal,
} from "../lib/session-badge";
import { SESSION_TONE_ROUNDEL_CLASS, SESSION_TONE_WORD_CLASS } from "../lib/session-badge-classes";

/**
 * The two rendered scales of the one badge dictionary
 * (`lib/session-badge.ts`): a 7–9 px shape for list rows and an 18 px tinted
 * glyph roundel for bell rows, toasts, palette rows, and the window status
 * line. Both pair tone with a distinct shape or glyph and carry the state word
 * as an accessible label, so no state is conveyed by colour alone.
 */

// Geometry only — the tone comes from the dictionary. At 6 px the sub-token
// radius the artboard softens these with reads identically to a sharp corner,
// so the shapes stay on the radius scale: rotation is what separates the
// diamond from the square.
const SHAPE_CLASS: Record<SessionBadgeShape, string> = {
  dot: "",
  diamond: "size-1.5 rotate-45 rounded-none",
  square: "size-1.5 rounded-none",
  check: "",
  ring: "",
  "dashed-ring": "",
};

/** Rings render through `StatusDot`, whose tone vocabulary is its own. */
const RING_TONE: Record<string, StatusDotTone> = {
  unhealthy: "warning",
  stopped: "faint",
  unknown: "faint",
};

function ShapeMark({ signal }: { signal: SessionBadgeSignal }) {
  if (signal.shape === "ring" || signal.shape === "dashed-ring") {
    return (
      <StatusDot
        tone={RING_TONE[signal.label] ?? "faint"}
        variant="ring"
        className={signal.shape === "dashed-ring" ? "border-dashed" : undefined}
      />
    );
  }
  if (signal.shape === "check") {
    return <Icon as={signal.glyph} size="xs" className={SESSION_TONE_WORD_CLASS[signal.tone]} />;
  }
  return (
    <PillDot
      tone={signal.tone}
      pulse={signal.pulse}
      size="sm"
      className={SHAPE_CLASS[signal.shape]}
    />
  );
}

export interface SessionBadgeMarkProps extends Omit<React.ComponentProps<"span">, "children"> {
  badge: string | null | undefined;
}

/** Row-scale mark: shape + tone + an accessible state word. */
export function SessionBadgeMark({ badge, className, ...props }: SessionBadgeMarkProps) {
  const signal = sessionBadgeSignal(badge);
  return (
    <span
      role="img"
      aria-label={`Session badge: ${signal.label}`}
      data-badge={signal.label}
      className={cn("grid size-2 place-items-center", className)}
      {...props}
    >
      <ShapeMark signal={signal} />
    </span>
  );
}

export interface SessionBadgeGlyphProps extends Omit<React.ComponentProps<"span">, "children"> {
  badge: string | null | undefined;
}

/** 18 px tinted glyph roundel — the bell, toast, palette, and status-line scale. */
export function SessionBadgeGlyph({ badge, className, ...props }: SessionBadgeGlyphProps) {
  const signal = sessionBadgeSignal(badge);
  const hollow = signal.shape === "dashed-ring";
  return (
    <span
      role="img"
      aria-label={`Session badge: ${signal.label}`}
      data-badge={signal.label}
      className={cn(
        "grid size-4.5 shrink-0 place-items-center rounded-full",
        hollow ? "border border-line-strong text-subtle" : SESSION_TONE_ROUNDEL_CLASS[signal.tone],
        className
      )}
      {...props}
    >
      <Icon as={signal.glyph} size="xs" className={signal.pulse ? "animate-pulse" : undefined} />
    </span>
  );
}
