/**
 * The one session badge → state glyph / tone / word dictionary.
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
 * Signal grammar follows the shell-rail brand (`StateGlyph` canonical
 * mapping): needs-you reads as the Compozy-orange attention dot, a failure as
 * the danger ring, work in flight as the mint running ring, and resting states
 * as neutral marks. The word tone agrees with the glyph. Where two badges share
 * a glyph (the needs-you trio, the failure trio, idle/unknown) the entry
 * `speaks`: every surface that shows the mark also shows the plain state word,
 * so no state rests on colour or a shared shape alone. The exact CLI vocabulary
 * stays one step deeper, in the accessible label and `data-*` attributes.
 */
import type { PillTone, StateGlyphState } from "@compozy/ui";

import type { SessionPayload, SessionState } from "../types";

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

export interface SessionBadgeSignal {
  /** The shared work-state glyph (`StateGlyph`) at every scale. */
  state: StateGlyphState;
  /** Tone for the state word; it agrees with the glyph. */
  tone: PillTone;
  /**
   * The glyph is shared with another badge, so every surface that shows the
   * mark also shows the state word.
   */
  speaks: boolean;
  /** Exact CLI vocabulary — the accessible label and `data-*` lane. */
  label: SessionBadgeToken;
  /** Plain-language state word shown on screen. */
  displayLabel: string;
  attention: SessionAttentionClass;
}

export const SESSION_BADGE_SIGNAL = {
  // A stop whose ladder ran out without a verified death: the daemon still
  // reads `stopping` and asks the operator to retry. Needs-you by the daemon's
  // own class; nothing in the operator's work failed, the runtime is being
  // honest about what it cannot prove, so its word names that.
  "needs-attention": {
    state: "attention",
    tone: "accent",
    speaks: true,
    label: "needs-attention",
    displayLabel: "Needs attention",
    attention: "needs-you",
  },
  "waiting-for-input": {
    state: "attention",
    tone: "accent",
    speaks: true,
    label: "waiting-for-input",
    displayLabel: "Needs your answer",
    attention: "needs-you",
  },
  "waiting-for-auth": {
    state: "attention",
    tone: "accent",
    speaks: true,
    label: "waiting-for-auth",
    displayLabel: "Needs sign-in",
    attention: "needs-you",
  },
  failed: {
    state: "failed",
    tone: "danger",
    speaks: true,
    label: "failed",
    displayLabel: "Failed",
    attention: "needs-you",
  },
  done: {
    state: "done",
    tone: "neutral",
    speaks: false,
    label: "done",
    displayLabel: "Done",
    attention: "finished",
  },
  running: {
    state: "running",
    tone: "neutral",
    speaks: false,
    label: "running",
    displayLabel: "Working",
    attention: "none",
  },
  idle: {
    state: "idle",
    tone: "neutral",
    speaks: false,
    label: "idle",
    displayLabel: "Idle",
    attention: "none",
  },
  hung: {
    state: "failed",
    tone: "danger",
    speaks: true,
    label: "hung",
    displayLabel: "Stuck",
    attention: "none",
  },
  unhealthy: {
    state: "failed",
    tone: "danger",
    speaks: true,
    label: "unhealthy",
    displayLabel: "Having trouble",
    attention: "none",
  },
  stopped: {
    state: "stopped",
    tone: "neutral",
    speaks: false,
    label: "stopped",
    displayLabel: "Stopped",
    attention: "none",
  },
  unknown: {
    state: "idle",
    tone: "neutral",
    speaks: true,
    label: "unknown",
    displayLabel: "Unknown",
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

const STATE_BADGE_FALLBACK: Record<SessionState, SessionBadgeToken> = {
  active: "idle",
  starting: "running",
  stopping: "running",
  stopped: "stopped",
};

/** The session's badge, falling back to its lifecycle state when the daemon sent none. */
export function sessionBadgeOf(session: Pick<SessionPayload, "badge" | "state">): string {
  return session.badge || STATE_BADGE_FALLBACK[session.state];
}
