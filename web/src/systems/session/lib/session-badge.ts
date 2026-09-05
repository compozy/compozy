/**
 * The one session badge → tone / glyph / shape dictionary.
 *
 * Every attention surface reads from here: sidebar rows, the attention bell,
 * the palette Sessions view, and the session window status line. Two local maps
 * used to disagree about `running`, `idle`, `hung`, and `unhealthy` and knew
 * neither new badge; this module replaces both.
 *
 * The generated contract types `SessionPayload.badge` as an open `string`
 * (the daemon owns the vocabulary in `internal/session/badge.go`), so the web
 * declares the literal union itself and funnels every raw value through
 * `toSessionBadge`. `as const satisfies Record<SessionBadgeToken, …>` makes the
 * dictionary exhaustive: a new token fails `bun-typecheck` until it has an
 * entry, exactly like `TASK_STATUS_TONE` in `@/lib/status-tone`.
 *
 * Signal grammar is locked in `docs/design/opendesign/herdr-parity/DESIGN-NOTES.md`:
 * the needs-you class shares one tone (danger — "you are the blocker") and
 * differs by glyph, so colour is never the only channel. `needs-attention` is
 * the one needs-you member on warning (sessions-stability §08): the runtime,
 * not the operator, is what could not be confirmed. Two presentation
 * scales, one meaning: 7–9 px shapes on rows, 18 px tinted glyph roundels on
 * bell rows, toasts, palette rows, and the window status line. The state word
 * is the exact CLI vocabulary and is always present.
 */
import {
  Activity,
  Check,
  Circle,
  CircleHelp,
  Minus,
  Shield,
  TriangleAlert,
  X,
  type LucideIcon,
} from "lucide-react";

import type { PillTone } from "@compozy/ui";

/** The daemon's eleven-token badge vocabulary, in precedence order. */
export const SESSION_BADGES = [
  "needs-attention",
  "failed",
  "stopped",
  "waiting-for-auth",
  "waiting-for-input",
  "hung",
  "unhealthy",
  "running",
  "done",
  "idle",
  "unknown",
] as const;

export type SessionBadgeToken = (typeof SESSION_BADGES)[number];

/** Mirrors `session.AttentionClass` — the daemon derives it, the web renders it. */
export type SessionAttentionClass = "needs-you" | "finished" | "none";

/** Row-scale shape. Pairs with the tone so state never reads as colour alone. */
export type SessionBadgeShape = "dot" | "diamond" | "square" | "check" | "ring" | "dashed-ring";

export interface SessionBadgeSignal {
  /** Tone for the roundel, the state word, and the row mark. */
  tone: PillTone;
  /** 7–9 px row mark. */
  shape: SessionBadgeShape;
  /** 18 px roundel glyph. */
  glyph: LucideIcon;
  pulse: boolean;
  /** Exact CLI vocabulary — the accessible label lane. */
  label: SessionBadgeToken;
  attention: SessionAttentionClass;
}

export const SESSION_BADGE_SIGNAL = {
  // A stop whose ladder ran out without a verified death: the daemon still
  // reads `stopping` and asks the operator to retry. Needs-you by the daemon's
  // own class, but warning rather than danger — nothing in the operator's work
  // failed; the runtime is being honest about what it cannot prove.
  "needs-attention": {
    tone: "warning",
    shape: "diamond",
    glyph: TriangleAlert,
    pulse: false,
    label: "needs-attention",
    attention: "needs-you",
  },
  "waiting-for-input": {
    tone: "danger",
    shape: "dot",
    glyph: CircleHelp,
    pulse: false,
    label: "waiting-for-input",
    attention: "needs-you",
  },
  "waiting-for-auth": {
    tone: "danger",
    shape: "diamond",
    glyph: Shield,
    pulse: false,
    label: "waiting-for-auth",
    attention: "needs-you",
  },
  failed: {
    tone: "danger",
    shape: "square",
    glyph: X,
    pulse: false,
    label: "failed",
    attention: "needs-you",
  },
  done: {
    tone: "info",
    shape: "check",
    glyph: Check,
    pulse: false,
    label: "done",
    attention: "finished",
  },
  running: {
    tone: "accent",
    shape: "dot",
    glyph: Circle,
    pulse: true,
    label: "running",
    attention: "none",
  },
  idle: {
    tone: "success",
    shape: "dot",
    glyph: Circle,
    pulse: false,
    label: "idle",
    attention: "none",
  },
  hung: {
    tone: "warning",
    shape: "dot",
    glyph: Activity,
    pulse: false,
    label: "hung",
    attention: "none",
  },
  unhealthy: {
    tone: "warning",
    shape: "ring",
    glyph: Activity,
    pulse: false,
    label: "unhealthy",
    attention: "none",
  },
  stopped: {
    tone: "neutral",
    shape: "ring",
    glyph: Circle,
    pulse: false,
    label: "stopped",
    attention: "none",
  },
  unknown: {
    tone: "neutral",
    shape: "dashed-ring",
    glyph: Minus,
    pulse: false,
    label: "unknown",
    attention: "none",
  },
} as const satisfies Record<SessionBadgeToken, SessionBadgeSignal>;

const BADGE_TOKENS: ReadonlySet<string> = new Set(SESSION_BADGES);

/**
 * Narrows a raw contract badge to the rendered vocabulary. An unrecognized
 * token renders `unknown` — the honesty state — never a guessed liveness.
 */
export function toSessionBadge(badge: string | null | undefined): SessionBadgeToken {
  return badge !== null && badge !== undefined && BADGE_TOKENS.has(badge)
    ? (badge as SessionBadgeToken)
    : "unknown";
}

export function sessionBadgeSignal(badge: string | null | undefined): SessionBadgeSignal {
  return SESSION_BADGE_SIGNAL[toSessionBadge(badge)];
}

export function sessionAttentionClass(badge: string | null | undefined): SessionAttentionClass {
  return sessionBadgeSignal(badge).attention;
}

/** The needs-you class: waiting-for-input, waiting-for-auth, failed, needs-attention. */
export function isNeedsYouBadge(badge: string | null | undefined): boolean {
  return sessionAttentionClass(badge) === "needs-you";
}

/** The finished-unseen class is exactly `done`. It never counts toward needs-you. */
export function isFinishedBadge(badge: string | null | undefined): boolean {
  return sessionAttentionClass(badge) === "finished";
}
