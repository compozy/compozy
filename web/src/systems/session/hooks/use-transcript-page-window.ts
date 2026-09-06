import { useQueryClient } from "@tanstack/react-query";
import { useEffect } from "react";

import { sessionKeys } from "../lib/query-keys";
import type { SessionTranscriptData } from "../lib/session-transcript-query";
import {
  releaseFarTranscriptPages,
  TRANSCRIPT_KEEP_PAGES,
  transcriptPageIndexOf,
} from "../lib/session-transcript-window";

/** Minimum spacing between reading-position samples (scroll is noisy; pages are coarse). */
const PAGE_WINDOW_SAMPLE_MS = 750;

export interface UseTranscriptPageWindowOptions {
  workspaceId: string;
  sessionId: string;
  /** The scroller whose reading position bounds the window. */
  viewport: React.RefObject<HTMLDivElement | null>;
  /** The first message whose row reaches into the viewport, read from the DOM. */
  readVisibleMessageId: () => string | null;
  /** Called right before pages above the reader are released so the position can be re-anchored. */
  onBeforeRelease: () => void;
  /** Loading older history or repositioning: never release mid-flight. */
  paused: boolean;
  enabled?: boolean;
  keepPages?: number;
}

/**
 * Windowed page memory over the transcript infinite query (US-019). While the
 * reader sits near the live tail with many older pages loaded, pages beyond
 * the keep-window are released from the cache; the last kept page still names
 * its older-page request, so scrolling back up reloads them through the
 * ordinary load-older path. Samples the reading position on scroll, never
 * releases while history is loading, and re-anchors the viewport through the
 * caller so nothing moves under the reader.
 */
export function useTranscriptPageWindow({
  workspaceId,
  sessionId,
  viewport,
  readVisibleMessageId,
  onBeforeRelease,
  paused,
  enabled = true,
  keepPages = TRANSCRIPT_KEEP_PAGES,
}: UseTranscriptPageWindowOptions): void {
  const queryClient = useQueryClient();

  useEffect(() => {
    const element = viewport.current;
    if (!enabled || !element || workspaceId === "" || sessionId === "") return undefined;
    const queryKey = sessionKeys.transcript(workspaceId, sessionId);
    let lastSampleAt = 0;

    const sample = () => {
      if (paused) return;
      const data = queryClient.getQueryData<SessionTranscriptData>(queryKey);
      if (!data || data.pages.length <= keepPages) return;
      const visiblePageIndex = transcriptPageIndexOf(data, readVisibleMessageId());
      const next = releaseFarTranscriptPages(data, { visiblePageIndex, keepPages });
      if (next === data) return;
      onBeforeRelease();
      queryClient.setQueryData<SessionTranscriptData>(queryKey, next);
    };
    // Time-spaced sampling on scroll, no timers: the next scroll event carries
    // any sample a throttled one skipped, and a window that stops scrolling has
    // nothing left to release.
    const handleScroll = () => {
      const now = Date.now();
      if (now - lastSampleAt < PAGE_WINDOW_SAMPLE_MS) return;
      lastSampleAt = now;
      sample();
    };

    element.addEventListener("scroll", handleScroll, { passive: true });
    sample();
    return () => {
      element.removeEventListener("scroll", handleScroll);
    };
  }, [
    enabled,
    keepPages,
    onBeforeRelease,
    paused,
    queryClient,
    readVisibleMessageId,
    sessionId,
    viewport,
    workspaceId,
  ]);
}
