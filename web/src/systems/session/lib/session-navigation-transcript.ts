import { isDeliberateTerminalTool } from "./session-terminal-tools";
import type { NormalizedSessionTranscriptEntry, SessionTranscriptSearchMatch } from "../types";
import type { SessionSequenceRange } from "./session-navigation";

/*
 * The loaded transcript as navigation sees it: entries by start sequence, the
 * row each lands on, and — for a search match — the exact part and field the
 * daemon matched, read from the loaded entry's projected parts.
 */

/** One loaded entry: its start cursor, row identity, turn, parts and time. */
export interface SessionSequenceEntry {
  sequence: number;
  messageId: string;
  role: string;
  /** The turn the parts name; the message id when none does (the timeline's own fallback). */
  turnId: string;
  /** The canonical projected parts, in the daemon's order (`part_index` indexes into them). */
  parts: readonly unknown[];
  /** Epoch milliseconds of the earliest timestamped part, when any part carries one. */
  at: number | null;
}

export interface SessionSequenceIndex {
  /** Ascending by start sequence. */
  entries: readonly SessionSequenceEntry[];
  range: SessionSequenceRange | null;
  bySequence: ReadonlyMap<number, SessionSequenceEntry>;
  sequenceOfMessage: ReadonlyMap<string, number>;
}

const EMPTY_SEQUENCE_INDEX: SessionSequenceIndex = {
  bySequence: new Map(),
  entries: [],
  range: null,
  sequenceOfMessage: new Map(),
};

function readString(record: unknown, key: string): string | undefined {
  if (typeof record !== "object" || record === null) return undefined;
  const value = (record as Record<string, unknown>)[key];
  return typeof value === "string" && value.length > 0 ? value : undefined;
}

function partTurnId(parts: readonly unknown[]): string | undefined {
  for (const part of parts) {
    const turnId = readString(part, "turn_id") ?? readString(part, "turnId");
    if (turnId) return turnId;
  }
  return undefined;
}

function partsEarliestAt(parts: readonly unknown[]): number | null {
  let earliest: number | null = null;
  for (const part of parts) {
    const stamp = readString(part, "timestamp");
    if (!stamp) continue;
    const ms = Date.parse(stamp);
    if (!Number.isFinite(ms)) continue;
    if (earliest === null || ms < earliest) earliest = ms;
  }
  return earliest;
}

/**
 * The loaded transcript pages as a sequence index. Pure over the cache data;
 * the host recomputes it when the pages change.
 */
export function indexTranscriptSequences(
  entries: readonly NormalizedSessionTranscriptEntry[]
): SessionSequenceIndex {
  if (entries.length === 0) return EMPTY_SEQUENCE_INDEX;
  const bySequence = new Map<number, SessionSequenceEntry>();
  const sequenceOfMessage = new Map<string, number>();
  const indexed: SessionSequenceEntry[] = [];
  for (const entry of entries) {
    const parts: readonly unknown[] = entry.message.parts ?? [];
    const item: SessionSequenceEntry = {
      at: partsEarliestAt(parts),
      messageId: entry.message.id,
      parts,
      role: entry.message.role,
      sequence: entry.start_sequence,
      turnId: partTurnId(parts) ?? entry.message.id,
    };
    indexed.push(item);
    bySequence.set(item.sequence, item);
    sequenceOfMessage.set(item.messageId, item.sequence);
  }
  indexed.sort((a, b) => a.sequence - b.sequence);
  return {
    bySequence,
    entries: indexed,
    range: { from: indexed[0]!.sequence, to: indexed[indexed.length - 1]!.sequence },
    sequenceOfMessage,
  };
}

export type SessionFindMatchPartKind = "text" | "reasoning" | "tool" | "data" | "file" | "other";

/** Where a search match lives in the loaded entry, as the daemon reported it. */
export interface SessionFindMatchSource {
  partIndex: number;
  /** `text | title | tool_name | filename | error | input | output` as the daemon names them. */
  field: string;
  kind: SessionFindMatchPartKind;
  toolCallId: string | null;
  /** The part rests behind the turn's settled fold (reasoning and transient tool work). */
  insideFold: boolean;
  /** Seeing the matched field needs the part's collapsed body open (reasoning, tool input/output/error). */
  opensBody: boolean;
}

function partKind(type: string): SessionFindMatchPartKind {
  if (type === "text") return "text";
  if (type === "reasoning") return "reasoning";
  if (type.startsWith("tool-")) return "tool";
  if (type === "data" || type.startsWith("data-")) return "data";
  if (type === "file") return "file";
  return "other";
}

/**
 * The matched part and field, located in the loaded entry from the daemon's
 * `part_index`/`field`. `null` when the match carries no source (an older
 * daemon) or the index names no loaded part — callers then know nothing about
 * the part and must not claim one.
 */
export function findMatchSource(
  match: Pick<SessionTranscriptSearchMatch, "part_index" | "field">,
  entry: SessionSequenceEntry | undefined
): SessionFindMatchSource | null {
  const partIndex = match.part_index;
  if (!entry || partIndex === undefined || partIndex === null || partIndex < 0) return null;
  const part = entry.parts[partIndex];
  if (part === undefined) return null;
  const kind = partKind(readString(part, "type") ?? "");
  const field = match.field ?? "";
  const toolName = kind === "tool" ? (readString(part, "type") ?? "").slice("tool-".length) : "";
  return {
    field,
    insideFold:
      entry.role === "assistant" &&
      (kind === "reasoning" || (kind === "tool" && !isDeliberateTerminalTool(toolName))),
    kind,
    opensBody:
      kind === "reasoning" ||
      (kind === "tool" &&
        (field === "input" ||
          field === "output" ||
          field === "error" ||
          field === "title" ||
          field === "tool_name")),
    partIndex,
    toolCallId: kind === "tool" ? (readString(part, "toolCallId") ?? null) : null,
  };
}

/** Matches behind each turn's fold, by the daemon's `turn_id`, over the loaded entries. */
export function findMatchesInsideFolds(
  matches: readonly SessionTranscriptSearchMatch[],
  index: SessionSequenceIndex
): ReadonlyMap<string, number> {
  const counts = new Map<string, number>();
  for (const match of matches) {
    const source = findMatchSource(match, index.bySequence.get(match.sequence));
    if (!source?.insideFold) continue;
    counts.set(match.turn_id, (counts.get(match.turn_id) ?? 0) + 1);
  }
  return counts;
}

/**
 * The row a jump lands on: the entry that starts at `sequence`, or the nearest
 * loaded entry above it when the cursor names none (the thread renders one row
 * per entry, whatever its role).
 */
export function landingMessageId(index: SessionSequenceIndex, sequence: number): string | null {
  let landing: string | null = null;
  for (const entry of index.entries) {
    if (entry.sequence > sequence) break;
    landing = entry.messageId;
  }
  return landing;
}

/** The trail's reading inputs from the first/last rows in view, as sequences. */
export function visibleSequenceWindow(
  index: SessionSequenceIndex,
  firstVisibleMessageId: string | null,
  lastVisibleMessageId: string | null
): { top: number | null; range: SessionSequenceRange | null } {
  const top =
    firstVisibleMessageId === null
      ? null
      : (index.sequenceOfMessage.get(firstVisibleMessageId) ?? null);
  const bottom =
    lastVisibleMessageId === null
      ? null
      : (index.sequenceOfMessage.get(lastVisibleMessageId) ?? null);
  if (top === null) return { range: null, top: null };
  return { range: { from: top, to: bottom ?? top }, top };
}
