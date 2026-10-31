import type { SessionPayload } from "../types";
import { markdownLineToPlainText } from "./markdown-plain-text";

/** The daemon's reason limit in Markdown source characters (`session_later_invalid` past it). */
export const SESSION_LATER_REASON_MAX = 1000;

const BACKSLASH_ESCAPE = /\\([\\`*_{}[\]()#+\-.!~|>])/g;
const CODE_SPAN = /`([^`]*)`/g;
// A link destination may hold one balanced pair of parentheses (`a_(v2)`).
const BALANCED_LINK = /!?\[([^\]]*)\]\((?:[^()]|\([^()]*\))*\)/g;
const LITERAL_SLOT = /\uE000(\d+)\uE001/g;

/**
 * One reason line as plain text. Code spans and escaped characters are parked
 * behind private-use placeholders while the Markdown marks are stripped, so
 * their characters stay literal.
 */
function laterReasonLineToPlainText(line: string): string {
  const literals: string[] = [];
  const park = (literal: string) => `\uE000${literals.push(literal) - 1}\uE001`;
  const parked = line
    .replace(CODE_SPAN, (_match, code: string) => park(code))
    .replace(BACKSLASH_ESCAPE, (_match, char: string) => park(char))
    .replace(BALANCED_LINK, "$1");
  return markdownLineToPlainText(parked)
    .replace(LITERAL_SLOT, (slot, index: string) => literals[Number(index)] ?? slot)
    .trim();
}

/**
 * The lines of a Markdown Later reason as plain text: block markers and inline
 * delimiters drop, link text stays, code spans and escapes stay literal, empty
 * lines go. The stored reason is never rewritten.
 */
export function laterReasonLines(reason: string): string[] {
  return reason
    .split(/\r?\n/)
    .map(laterReasonLineToPlainText)
    .filter(line => line !== "");
}

export interface SessionLaterDetails {
  /** The reason read as one plain line ("a · b" for several lines). */
  reason: string | null;
  /** The reason as plain lines, for a hover title. */
  reasonLines: string[];
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
  const reasonLines = laterReasonLines(later.reason ?? "");
  return {
    reason: reasonLines.length > 0 ? reasonLines.join(" · ") : null,
    reasonLines,
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
    `Parked ${formatDate(details.parkedAt)}`,
    details.pendingMessages > 0 ? laterPendingLabel(details.pendingMessages) : null,
  ]
    .filter(Boolean)
    .join(", ");
}

/** Why the Reason field cannot be sent, or `null`; counts the Markdown source. */
export function laterReasonError(reason: string): string | null {
  return Array.from(reason.trim()).length > SESSION_LATER_REASON_MAX
    ? `Use ${SESSION_LATER_REASON_MAX} characters or fewer.`
    : null;
}

/**
 * The RFC 3339 instant the daemon stores for a picked return date, or why it
 * cannot be sent. The picker works in the operator's own zone; the instant is
 * absolute, so it travels as UTC.
 */
export function laterReturnAtFromDate(
  value: Date | null,
  now: number = Date.now()
): { returnAt: string | null; error: string | null } {
  if (value === null) return { returnAt: null, error: null };
  if (!Number.isFinite(value.getTime())) return { returnAt: null, error: "Enter a date and time." };
  if (value.getTime() <= now) return { returnAt: null, error: "Choose a time in the future." };
  return { returnAt: value.toISOString().replace(/\.\d{3}Z$/, "Z"), error: null };
}

function atNine(base: Date, daysAhead: number): Date {
  const date = new Date(base);
  date.setDate(date.getDate() + daysAhead);
  date.setHours(9, 0, 0, 0);
  return date;
}

/** One-click return dates beside the calendar, resolved when picked. */
export function laterReturnPresets(): { label: string; value: () => Date }[] {
  return [
    {
      label: "In 3 hours",
      value: () => {
        const date = new Date(Date.now() + 3 * 60 * 60_000);
        // Land on the next half hour so the time list shows it selected.
        date.setMinutes(Math.ceil(date.getMinutes() / 30) * 30, 0, 0);
        return date;
      },
    },
    { label: "Tomorrow", value: () => atNine(new Date(), 1) },
    {
      label: "Next Monday",
      value: () => {
        const now = new Date();
        return atNine(now, (8 - now.getDay()) % 7 || 7);
      },
    },
    { label: "In 2 weeks", value: () => atNine(new Date(), 14) },
  ];
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
