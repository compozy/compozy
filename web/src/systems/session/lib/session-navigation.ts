import { SESSION_TRANSCRIPT_SEARCH_MAX_BYTES } from "../adapters/session-navigation-api";
import { findFoldedOccurrences } from "./session-find-text";
import type { SessionTranscriptOutlineEntry, SessionTranscriptSearchMatch } from "../types";

/*
 * Pure derivations behind find (S8) and the message trail (S9). Nothing here
 * reads the DOM or the query cache: the host passes what it knows (loaded
 * range, reading anchor, viewport height) and gets back what to render.
 */

// ---------------------------------------------------------------------------
// Find
// ---------------------------------------------------------------------------

/** A contiguous run of the loaded transcript, by entry start sequence. */
export interface SessionSequenceRange {
  from: number;
  to: number;
}

/** The literal the daemon will receive: trimmed, and never past the route's byte bound. */
export function normalizeTranscriptSearchQuery(query: string): string {
  const trimmed = query.trim();
  if (trimmed.length === 0) return "";
  return new TextEncoder().encode(trimmed).length > SESSION_TRANSCRIPT_SEARCH_MAX_BYTES
    ? ""
    : trimmed;
}

/** One segment of a snippet: the literal match glazed, the rest plain. */
export interface SessionFindSnippetSegment {
  match: boolean;
  text: string;
}

/**
 * Splits a snippet around every occurrence of the query under the daemon's
 * per-code-point folding (`session-find-text`), so the list glazes exactly the
 * characters the daemon matched, without a regex on untrusted text.
 */
export function findSnippetSegments(snippet: string, query: string): SessionFindSnippetSegment[] {
  if (snippet.length === 0) return [];
  const spans = findFoldedOccurrences(snippet, query.trim());
  if (spans.length === 0) return [{ match: false, text: snippet }];
  const segments: SessionFindSnippetSegment[] = [];
  let cursor = 0;
  for (const span of spans) {
    if (span.start > cursor)
      segments.push({ match: false, text: snippet.slice(cursor, span.start) });
    segments.push({ match: true, text: snippet.slice(span.start, span.end) });
    cursor = span.end;
  }
  if (cursor < snippet.length) segments.push({ match: false, text: snippet.slice(cursor) });
  return segments;
}

/** Who said it, in the reader's words: the operator, the agent by name, or a tool. */
export function findMatchSpeaker(role: string, agentName: string): string {
  switch (role.trim().toLowerCase()) {
    case "user":
      return "You";
    case "assistant":
      return agentName.trim() || "Agent";
    case "tool":
      return "Tool";
    default:
      return role.trim() || "Session";
  }
}

/** Whether the entry that starts at `sequence` is in the loaded window. */
export function isSequenceLoaded(range: SessionSequenceRange | null, sequence: number): boolean {
  return range !== null && sequence >= range.from && sequence <= range.to;
}

/** Index of the match at `sequence`, or -1; matches are the daemon's ascending list. */
export function findMatchIndex(
  matches: readonly SessionTranscriptSearchMatch[],
  sequence: number | null
): number {
  return sequence === null ? -1 : matches.findIndex(match => match.sequence === sequence);
}

/**
 * The active match after a step. Enter steps forward, Shift+Enter back; both
 * wrap. With nothing active the first step lands on the first (or last) match.
 */
export function stepFindMatch(
  matches: readonly SessionTranscriptSearchMatch[],
  activeSequence: number | null,
  direction: 1 | -1
): number | null {
  if (matches.length === 0) return null;
  const current = findMatchIndex(matches, activeSequence);
  if (current < 0) {
    return (direction === 1 ? matches[0] : matches[matches.length - 1])!.sequence;
  }
  const next = (current + direction + matches.length) % matches.length;
  return matches[next]!.sequence;
}

/**
 * What a refreshed match list means for the reader (US-028.EC-2): the active
 * match survives by identity when it is still present, and matches appended
 * past the previous tail are counted as "new" without moving anything.
 */
export function reconcileFindMatches(
  previous: readonly SessionTranscriptSearchMatch[],
  next: readonly SessionTranscriptSearchMatch[],
  activeSequence: number | null
): { activeSequence: number | null; appended: number } {
  const active =
    activeSequence !== null && next.some(match => match.sequence === activeSequence)
      ? activeSequence
      : null;
  const previousTail = previous.length > 0 ? previous[previous.length - 1]!.sequence : null;
  const appended =
    previousTail === null && previous.length === 0
      ? 0
      : next.filter(match => previousTail !== null && match.sequence > previousTail).length;
  return { activeSequence: active, appended };
}

/** The count slot's sentence for the whole session, never the loaded window. */
export function findCountLabel(activeIndex: number, total: number, truncated: boolean): string {
  if (total === 0) return "No matches";
  const shown = truncated ? `${total}+` : String(total);
  return activeIndex < 0
    ? `${shown} ${total === 1 && !truncated ? "match" : "matches"}`
    : `${activeIndex + 1} of ${shown}`;
}

// ---------------------------------------------------------------------------
// Trail (UT-111)
// ---------------------------------------------------------------------------

/** The rail appears only with two or more sent messages; one is not a trail. */
export const TRAIL_MIN_ENTRIES = 2;
/** The trail's compression shape: spacing from the count alone, no measurement. */
export const TRAIL_TICK_HEIGHT_PX = 2;
export const TRAIL_HIT_HEIGHT_PX = 8;
export const TRAIL_MAX_GAP_PX = 9;
export const TRAIL_MIN_GAP_PX = 3;
/** Spacing starts shrinking past this many ticks and reaches the minimum at four times it. */
export const TRAIL_COMPRESS_FROM = 12;
/** The rail may use this share of the pane height before ticks are distributed proportionally. */
export const TRAIL_MAX_HEIGHT_SHARE = 0.8;
export const TRAIL_TICK_WIDTH_PX = 6;
export const TRAIL_TICK_WIDTH_HOVER_PX = 18;
export const TRAIL_TICK_WIDTH_ANCHOR_PX = 14;

/** Gap between ticks for `count` messages: 9px up to the threshold, easing to 3px. */
export function trailGap(count: number): number {
  if (count <= TRAIL_COMPRESS_FROM) return TRAIL_MAX_GAP_PX;
  const span = TRAIL_COMPRESS_FROM * 3;
  const progress = Math.min(1, (count - TRAIL_COMPRESS_FROM) / span);
  return Math.round(TRAIL_MAX_GAP_PX - (TRAIL_MAX_GAP_PX - TRAIL_MIN_GAP_PX) * progress);
}

export interface SessionTrailLayout {
  /** Vertical offset of each tick's hit slot, in px from the rail top. */
  offsets: number[];
  /** True once the count no longer fits at the minimum gap and slots are spread proportionally. */
  compressed: boolean;
  /** Slot pitch (hit height + gap) or the proportional pitch when compressed. */
  pitch: number;
  /** Total rail height used. */
  height: number;
}

/**
 * Where each tick sits. Layout depends only on the count and the pane height
 * (US-029.EC-1): the natural pitch is the hit height plus the count-driven
 * gap; when that would exceed the rail's share of the pane, ticks are spread
 * proportionally across it instead — every tick keeps its own 8px hit target.
 */
export function trailLayout(count: number, paneHeightPx: number): SessionTrailLayout {
  if (count <= 0) return { compressed: false, height: 0, offsets: [], pitch: 0 };
  const naturalPitch = TRAIL_HIT_HEIGHT_PX + trailGap(count);
  const naturalHeight = naturalPitch * count - trailGap(count);
  const limit = Math.max(TRAIL_HIT_HEIGHT_PX, Math.floor(paneHeightPx * TRAIL_MAX_HEIGHT_SHARE));
  if (naturalHeight <= limit || count === 1) {
    return {
      compressed: false,
      height: naturalHeight,
      offsets: Array.from({ length: count }, (_, index) => index * naturalPitch),
      pitch: naturalPitch,
    };
  }
  const pitch = (limit - TRAIL_HIT_HEIGHT_PX) / (count - 1);
  return {
    compressed: true,
    height: limit,
    offsets: Array.from({ length: count }, (_, index) => Math.round(index * pitch)),
    pitch,
  };
}

/**
 * Dock magnification around the pointer: a Gaussian window whose width grows
 * as the rail gets denser (σ = clamp(pitch × 1.5, 8, 22)), widths easing from
 * the rest width to the hover width. Colour never moves — only width.
 */
export function trailTickWidth(
  offset: number,
  hoveredOffset: number | null,
  pitch: number
): number {
  if (hoveredOffset === null) return TRAIL_TICK_WIDTH_PX;
  const sigma = Math.min(22, Math.max(8, pitch * 1.5));
  const distance = Math.abs(offset - hoveredOffset);
  const weight = Math.exp(-(distance * distance) / (2 * sigma * sigma));
  return Math.round(
    TRAIL_TICK_WIDTH_PX + (TRAIL_TICK_WIDTH_HOVER_PX - TRAIL_TICK_WIDTH_PX) * weight
  );
}

export type SessionTrailTickState = "anchor" | "in-view" | "rest";

/**
 * The reading anchor is the last sent message at or above the viewport top;
 * ticks for messages currently in view read as such; everything else rests.
 */
export function trailAnchorSequence(
  entries: readonly SessionTranscriptOutlineEntry[],
  viewportTopSequence: number | null
): number | null {
  if (viewportTopSequence === null) return null;
  let anchor: number | null = null;
  for (const entry of entries) {
    if (entry.sequence <= viewportTopSequence) anchor = entry.sequence;
    else break;
  }
  return anchor;
}

export function trailTickState(
  entry: SessionTranscriptOutlineEntry,
  anchorSequence: number | null,
  visibleRange: SessionSequenceRange | null
): SessionTrailTickState {
  if (entry.sequence === anchorSequence) return "anchor";
  return isSequenceLoaded(visibleRange, entry.sequence) ? "in-view" : "rest";
}

/** Roving focus in the rail: arrows move, Home/End jump to the ends; `null` for other keys. */
export function trailKeyTarget(key: string, index: number, count: number): number | null {
  if (count === 0) return null;
  switch (key) {
    case "ArrowDown":
    case "ArrowRight":
      return Math.min(count - 1, index + 1);
    case "ArrowUp":
    case "ArrowLeft":
      return Math.max(0, index - 1);
    case "Home":
      return 0;
    case "End":
      return count - 1;
    default:
      return null;
  }
}

/** Accessible name for a tick: ordinal plus the first line of what was asked. */
export function trailTickLabel(entry: SessionTranscriptOutlineEntry, ordinal: number): string {
  const preview =
    entry.preview
      .split(/\r?\n/)
      .find(line => line.trim().length > 0)
      ?.trim() ?? "";
  return preview.length > 0 ? `Message ${ordinal}: ${preview}` : `Message ${ordinal}`;
}

// ---------------------------------------------------------------------------
// Host (viewport integration): refresh cadence and the find shortcut
// ---------------------------------------------------------------------------

/**
 * When the navigation reads refetch: the durable fences (epoch/generation),
 * the entry count (a new ask or a landed page), the streaming edge settling,
 * and a slow tick while a turn streams — never a token delta.
 */
export function navigationRefreshKey(input: {
  epoch: number | null;
  generation: number | null;
  entryCount: number;
  streaming: boolean;
  streamTick: number;
}): string {
  return [
    input.epoch ?? "-",
    input.generation ?? "-",
    input.entryCount,
    input.streaming ? `live:${input.streamTick}` : "settled",
  ].join(":");
}

/** The find shortcut: ⌘F on macOS, Ctrl+F elsewhere, no other modifier. */
export function isFindShortcut(event: {
  key: string;
  metaKey: boolean;
  ctrlKey: boolean;
  altKey: boolean;
  shiftKey: boolean;
}): boolean {
  if (event.key.toLowerCase() !== "f" || event.altKey || event.shiftKey) return false;
  return event.metaKey !== event.ctrlKey;
}

/** Selected text as a find seed: one line, trimmed, bounded; empty when nothing usable. */
export const FIND_SEED_MAX_CHARS = 120;
export function findShortcutSeed(selection: string | null | undefined): string {
  if (!selection) return "";
  const line = selection.split(/\r?\n/, 1)[0]?.trim() ?? "";
  return line.length > FIND_SEED_MAX_CHARS ? "" : line;
}
