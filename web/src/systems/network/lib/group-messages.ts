import type { NetworkConversationMessage } from "../types";
import { TIMELINE_GROUP_WINDOW_SECONDS, isWithinSeconds } from "./format-timestamp";

export type TimelineRowVariant = "full" | "collapsed" | "system";

export const SYSTEM_KINDS: ReadonlySet<string> = new Set([
  "greet",
  "whois",
  "capability",
  "receipt",
  "trace",
]);

export interface TimelineDatePillEntry {
  kind: "date-pill";
  /** Stable id for keys; uses ISO date prefix. */
  id: string;
  timestamp: string;
}

export interface TimelineNewDividerEntry {
  kind: "new-divider";
  /** Stable id for keys; uses the message id below the divider. */
  id: string;
}

export interface TimelineMessageEntry {
  kind: "message";
  message: NetworkConversationMessage;
  variant: TimelineRowVariant;
  /** True when the row is the first message of an author group (after pills/dividers reset state). */
  startsGroup: boolean;
  id: string;
}

export type TimelineEntry = TimelineDatePillEntry | TimelineNewDividerEntry | TimelineMessageEntry;

export interface BuildTimelineOptions {
  messages: ReadonlyArray<NetworkConversationMessage>;
  /** Reference moment used to format date pills. */
  now?: Date;
  /** Last-read timestamp used to position the "New" divider. */
  lastReadAt?: string | null;
  /**
   * Window override for tests; default is 60s per `_design.md` §5.3.
   */
  windowSeconds?: number;
}

function getTimestamp(message: NetworkConversationMessage): string {
  return message.timestamp ?? "";
}

function shouldStartGroup(
  current: NetworkConversationMessage,
  previous: NetworkConversationMessage | null,
  windowSeconds: number
): boolean {
  if (!previous) {
    return true;
  }
  if (current.peer_from !== previous.peer_from) {
    return true;
  }
  if (current.kind !== previous.kind) {
    return true;
  }
  if (!isWithinSeconds(getTimestamp(current), getTimestamp(previous), windowSeconds)) {
    return true;
  }
  return false;
}

function pickVariant(
  current: NetworkConversationMessage,
  startsGroup: boolean
): TimelineRowVariant {
  if (SYSTEM_KINDS.has(current.kind)) {
    return "system";
  }
  return startsGroup ? "full" : "collapsed";
}

function isoDateKey(timestamp: string): string {
  if (!timestamp) {
    return "";
  }
  const parsed = new Date(timestamp);
  if (Number.isNaN(parsed.getTime())) {
    return "";
  }
  return parsed.toISOString().slice(0, 10);
}

export function buildTimelineEntries({
  messages,
  now,
  lastReadAt,
  windowSeconds = TIMELINE_GROUP_WINDOW_SECONDS,
}: BuildTimelineOptions): TimelineEntry[] {
  const entries: TimelineEntry[] = [];
  if (messages.length === 0) {
    return entries;
  }

  const lastReadMs = lastReadAt ? new Date(lastReadAt).getTime() : Number.NaN;
  const hasLastRead = !Number.isNaN(lastReadMs);
  let dividerEmitted = false;
  let previousForGrouping: NetworkConversationMessage | null = null;
  let previousDateKey = "";

  for (const message of messages) {
    const timestamp = getTimestamp(message);
    const dateKey = isoDateKey(timestamp);

    const dayChanged = !!dateKey && dateKey !== previousDateKey;
    if (dayChanged) {
      entries.push({
        kind: "date-pill",
        id: `date:${dateKey}`,
        timestamp,
      });
      previousDateKey = dateKey;
      previousForGrouping = null;
    }

    if (hasLastRead && !dividerEmitted && timestamp && new Date(timestamp).getTime() > lastReadMs) {
      entries.push({ kind: "new-divider", id: `new:${message.message_id}` });
      dividerEmitted = true;
      previousForGrouping = null;
    }

    const startsGroup = shouldStartGroup(message, previousForGrouping, windowSeconds);
    const variant = pickVariant(message, startsGroup);
    entries.push({
      kind: "message",
      message,
      variant,
      startsGroup,
      id: `msg:${message.message_id}`,
    });
    previousForGrouping = message;
  }

  // sanity log only useful in unit tests / dev — referenced by callers using `now`
  void now;

  return entries;
}

export function isSystemKind(kind: string): boolean {
  return SYSTEM_KINDS.has(kind);
}
