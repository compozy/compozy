// Windowed transcript memory (US-019 / Part II key decision: windowed memory over
// an engine swap). TanStack keeps the transcript as pages (head = live tail,
// older pages appended as the reader loads history). Pages far above the
// reader are released — trimmed from the infinite query — while the head and
// the pages around the reading position stay. Releasing never loses anything:
// the last kept page still carries its `next_before_sequence`, so the ordinary
// load-older path reloads a released page on demand. The assistant-ui host and
// the virtualizer stay as they are; only the page list is bounded.

import { nextTranscriptPageParam, type SessionTranscriptData } from "./session-transcript-query";

/** Pages kept around the reading position, the head included. */
export const TRANSCRIPT_KEEP_PAGES = 6;

/** Index of the page holding `messageId`, or `-1` when no loaded page does. */
export function transcriptPageIndexOf(
  data: SessionTranscriptData | undefined,
  messageId: string | null
): number {
  if (!data || messageId === null) return -1;
  return data.pages.findIndex(page => page.entries.some(entry => entry.message.id === messageId));
}

/**
 * Release pages further than `keepPages` beyond the reading position. The head
 * (index 0) is never released; a visible index of `-1` (nothing measured yet)
 * keeps everything. Returns the same object when nothing is released so cache
 * writes stay identity-stable.
 */
export function releaseFarTranscriptPages(
  data: SessionTranscriptData,
  options: { visiblePageIndex: number; keepPages?: number }
): SessionTranscriptData {
  const keepPages = Math.max(1, options.keepPages ?? TRANSCRIPT_KEEP_PAGES);
  if (options.visiblePageIndex < 0) return data;
  const keepThrough = Math.max(keepPages, options.visiblePageIndex + keepPages);
  if (data.pages.length <= keepThrough) return data;
  return {
    pages: data.pages.slice(0, keepThrough),
    pageParams: data.pageParams.slice(0, keepThrough),
  };
}

/** How a released page comes back: the standard older-page request of the last kept page. */
export function releasedPageReloadParam(data: SessionTranscriptData) {
  const last = data.pages.at(-1);
  return last ? nextTranscriptPageParam(last) : undefined;
}

/** Total loaded entries — the number the memory bound is about. */
export function loadedTranscriptEntryCount(data: SessionTranscriptData | undefined): number {
  return data?.pages.reduce((sum, page) => sum + page.entries.length, 0) ?? 0;
}
