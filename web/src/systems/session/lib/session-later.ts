import type { SessionPayload } from "../types";

/** The daemon's one-line reason limit (`session_later_invalid` past it). */
export const SESSION_LATER_REASON_MAX = 200;

export interface SessionLaterDetails {
  reason: string | null;
  /** When the daemon brings the session back to Needs you; `null` without a reminder. */
  returnAt: string | null;
  parkedAt: string;
  /** Agent messages queued for the session while it waits. */
  pendingMessages: number;
}

/** The Later facts of a parked session, or `null` while it is in the normal flow. */
export function sessionLaterDetails(
  session: Pick<SessionPayload, "later">
): SessionLaterDetails | null {
  const later = session.later;
  if (!later) return null;
  return {
    reason: later.reason?.trim() || null,
    returnAt: later.return_at ?? null,
    parkedAt: later.parked_at,
    pendingMessages: Math.max(0, later.pending_messages),
  };
}

export function sessionIsInLater(session: Pick<SessionPayload, "later">): boolean {
  return session.later != null;
}

/** "2 messages waiting". */
export function laterPendingLabel(count: number): string {
  return `${count} ${count === 1 ? "message" : "messages"} waiting`;
}

/** The parked facts in reading order, for an accessible name or description. */
export function sessionLaterSpoken(
  details: SessionLaterDetails | null,
  formatDate: (iso: string) => string
): string | null {
  if (details === null) return null;
  return [
    "In Later",
    details.reason,
    details.returnAt ? `Back ${formatDate(details.returnAt)}` : null,
    details.pendingMessages > 0 ? laterPendingLabel(details.pendingMessages) : null,
  ]
    .filter(Boolean)
    .join(", ");
}

/** Why the Reason field cannot be sent, or `null`. */
export function laterReasonError(reason: string): string | null {
  return Array.from(reason.trim()).length > SESSION_LATER_REASON_MAX
    ? `Use ${SESSION_LATER_REASON_MAX} characters or fewer.`
    : null;
}

/**
 * Reads a `datetime-local` value as the operator's own wall-clock time and
 * returns the RFC 3339 instant the daemon stores. Automation's
 * `localInputToDate` reads the same input as UTC because automations carry a
 * separate time zone; Later's `return_at` is one absolute instant, so it is
 * parsed in the browser's zone instead.
 */
export function laterReturnAtFromInput(
  value: string,
  now: number = Date.now()
): { returnAt: string | null; error: string | null } {
  const trimmed = value.trim();
  if (trimmed === "") return { returnAt: null, error: null };
  const match = /^(\d{4})-(\d{2})-(\d{2})T(\d{2}):(\d{2})/.exec(trimmed);
  if (!match) return { returnAt: null, error: "Enter a date and time." };
  const date = new Date(
    Number(match[1]),
    Number(match[2]) - 1,
    Number(match[3]),
    Number(match[4]),
    Number(match[5])
  );
  if (!Number.isFinite(date.getTime())) return { returnAt: null, error: "Enter a date and time." };
  if (date.getTime() <= now) return { returnAt: null, error: "Choose a time in the future." };
  return { returnAt: date.toISOString().replace(/\.\d{3}Z$/, "Z"), error: null };
}

const pad2 = (value: number) => String(value).padStart(2, "0");

/** The `datetime-local` value of an instant in the operator's own zone (the input's `min`). */
export function laterInputValue(date: Date): string {
  return (
    `${date.getFullYear()}-${pad2(date.getMonth() + 1)}-${pad2(date.getDate())}` +
    `T${pad2(date.getHours())}:${pad2(date.getMinutes())}`
  );
}

const RETURN_FORMATTER = new Intl.DateTimeFormat(undefined, {
  month: "short",
  day: "numeric",
  hour: "2-digit",
  minute: "2-digit",
  hour12: false,
});

/** The return date in a dense row: "Nov 3, 09:00"; the hover title carries the full date. */
export function formatLaterReturn(iso: string): string {
  const time = Date.parse(iso);
  return Number.isFinite(time) ? RETURN_FORMATTER.format(new Date(time)) : "—";
}
