import { replaceEqualDeep } from "@tanstack/react-query";

import type { NormalizedSessionTranscriptEntry, SessionTranscriptPage } from "../types";
import type { SessionTranscriptData } from "./session-transcript-query";

function isTranscriptData(value: unknown): value is SessionTranscriptData {
  return (
    typeof value === "object" &&
    value !== null &&
    Array.isArray((value as { pages?: unknown }).pages) &&
    Array.isArray((value as { pageParams?: unknown }).pageParams)
  );
}

function sameItems<T>(previous: readonly T[] | undefined, next: readonly T[]): boolean {
  return (
    previous !== undefined &&
    previous.length === next.length &&
    previous.every((item, index) => item === next[index])
  );
}

function pageFields(page: SessionTranscriptPage): Omit<SessionTranscriptPage, "entries"> {
  const { entries: _entries, ...fields } = page;
  return fields;
}

function sharePage(
  previous: SessionTranscriptPage | undefined,
  next: SessionTranscriptPage,
  entries: NormalizedSessionTranscriptEntry[]
): SessionTranscriptPage {
  if (previous === undefined) return { ...next, entries };
  const sharedEntries = sameItems(previous.entries, entries) ? previous.entries : entries;
  const previousFields = pageFields(previous);
  const fields = replaceEqualDeep(previousFields, pageFields(next));
  if (fields === previousFields && sharedEntries === previous.entries) return previous;
  return { ...fields, entries: sharedEntries };
}

/**
 * Structural sharing for the transcript infinite query, keyed by entry
 * `start_sequence` instead of array position (#1559).
 *
 * The head page is bounded: one new live entry pushes the oldest head entry
 * into the next page, and every full page passes one entry on. TanStack's
 * default positional `replaceEqualDeep` then compares each entry with its
 * former neighbour and returns a fresh copy of every loaded message, so one
 * appended row re-projected, re-estimated and re-rendered the whole history.
 * Matching entries by their stable start sequence keeps an unchanged entry the
 * same object wherever its page boundary moved, and still shares the equal
 * parts of an entry the stream updated in place.
 */
export function shareTranscriptData(previous: unknown, next: unknown): unknown {
  if (!isTranscriptData(previous) || !isTranscriptData(next)) {
    return replaceEqualDeep(previous, next);
  }
  // A live update usually rewrites the head only; older pages arrive as the
  // same objects and need no work. Index only the pages that were replaced.
  const carried = new Set(next.pages);
  const kept = new Set(previous.pages);
  const known = new Map<number, NormalizedSessionTranscriptEntry>();
  for (const page of previous.pages) {
    if (carried.has(page)) continue;
    for (const entry of page.entries) known.set(entry.start_sequence, entry);
  }
  const pages = next.pages.map((page, index) => {
    if (kept.has(page)) return page;
    const entries = page.entries.map(entry => {
      const former = known.get(entry.start_sequence);
      return former === undefined ? entry : replaceEqualDeep(former, entry);
    });
    return sharePage(previous.pages[index], page, entries);
  });
  const sharedPages = sameItems(previous.pages, pages) ? previous.pages : pages;
  const pageParams = replaceEqualDeep(previous.pageParams, next.pageParams);
  if (sharedPages === previous.pages && pageParams === previous.pageParams) {
    return previous;
  }
  return { pages: sharedPages, pageParams };
}
