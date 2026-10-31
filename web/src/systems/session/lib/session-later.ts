import type { SessionPayload } from "../types";
import { Lexer, type Token, type Tokens } from "marked";

/** The daemon's reason limit in Markdown source characters (`session_later_invalid` past it). */
export const SESSION_LATER_REASON_MAX = 1000;

// Reasons parse as GFM, the dialect the editor writes (`@tiptap/markdown` uses
// the same parser).
const REASON_LEXER_OPTIONS = { gfm: true } as const;

const LINE_BREAK_TAG = /^<br\s*\/?>$/i;

// A complete Markdown character reference: Markdown, unlike HTML, requires the
// final semicolon, so `&copy handling` stays literal.
const CHARACTER_REFERENCE = /&(?:#[0-9]{1,7}|#[xX][0-9a-fA-F]{1,6}|[A-Za-z][A-Za-z0-9]*);/g;

/**
 * Decodes complete character references (`&amp;`, `&#169;`) with the
 * browser's own table, one reference at a time so decoded text is never read
 * again. A detached `<textarea>` parses its content as RCDATA, so tags are
 * never interpreted, only references.
 */
function decodeCharacterReferences(text: string): string {
  if (!text.includes("&") || typeof document === "undefined") return text;
  const textarea = document.createElement("textarea");
  return text.replace(CHARACTER_REFERENCE, reference => {
    textarea.innerHTML = reference;
    return textarea.value;
  });
}

/**
 * The literal text of inline tokens: markup drops, link labels stay, prose
 * character references decode, and code spans and escapes stay as written.
 */
function inlinePlainText(tokens: readonly Token[] | undefined): string {
  return (tokens ?? [])
    .map(token => {
      switch (token.type) {
        case "codespan":
        case "escape":
          return token.text;
        case "br":
          return "\n";
        case "html":
          // Inline tags are markup; the text between them is read as usual. A
          // line-break tag still separates words.
          return LINE_BREAK_TAG.test(token.raw.trim()) ? "\n" : "";
        case "text":
          return "tokens" in token && token.tokens
            ? inlinePlainText(token.tokens)
            : decodeCharacterReferences(token.text);
        default:
          return "tokens" in token && token.tokens
            ? inlinePlainText(token.tokens)
            : "text" in token
              ? String(token.text)
              : token.raw;
      }
    })
    .join("");
}

/** Block tokens as plain lines: one per paragraph, list item, code line, or table row. */
function blockPlainLines(tokens: readonly Token[]): string[] {
  return tokens.flatMap(token => {
    switch (token.type) {
      case "space":
      case "hr":
        return [];
      case "list":
        return (token as Tokens.List).items.flatMap(item => blockPlainLines(item.tokens));
      case "blockquote":
        return blockPlainLines((token as Tokens.Blockquote).tokens);
      case "html":
        return [];
      case "code":
        return String(token.text).split("\n");
      case "table": {
        const table = token as Tokens.Table;
        return [table.header, ...table.rows].map(row =>
          row.map(cell => inlinePlainText(cell.tokens)).join(" · ")
        );
      }
      default:
        return "tokens" in token && token.tokens
          ? inlinePlainText(token.tokens).split("\n")
          : [token.raw];
    }
  });
}

/**
 * The lines of a Markdown Later reason as plain text, read through a GFM
 * parser: markup (including raw HTML) drops; link labels, code spans, escaped characters, and code
 * lines stay literal; empty lines go. The stored reason is never rewritten.
 */
export function laterReasonLines(reason: string): string[] {
  return blockPlainLines(Lexer.lex(reason, REASON_LEXER_OPTIONS)).flatMap(line => {
    const collapsed = line.replace(/\s+/g, " ").trim();
    return collapsed === "" ? [] : [collapsed];
  });
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

const HALF_HOUR_MS = 30 * 60_000;

/**
 * Three hours from `now`, moved forward to the next local half hour so the
 * time list shows it selected. The move is added as elapsed time, never by
 * setting clock fields, so a repeated hour (clocks going back) cannot pull the
 * instant earlier than three hours out.
 */
export function laterInThreeHours(now: number): Date {
  const base = new Date(now + 3 * 60 * 60_000);
  const intoHalfHour =
    (base.getMinutes() % 30) * 60_000 + base.getSeconds() * 1000 + base.getMilliseconds();
  return new Date(base.getTime() + (intoHalfHour === 0 ? 0 : HALF_HOUR_MS - intoHalfHour));
}

/** One-click return dates beside the calendar, resolved when picked. */
export function laterReturnPresets(): { label: string; value: () => Date }[] {
  return [
    {
      label: "In 3 hours",
      value: () => laterInThreeHours(Date.now()),
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
