import { describe, expect, it } from "vitest";

import type { SessionMessage } from "../../types";
import type { SessionTranscriptData } from "../session-transcript-query";
import {
  loadedTranscriptEntryCount,
  releasedPageReloadParam,
  releaseFarTranscriptPages,
  TRANSCRIPT_KEEP_PAGES,
  transcriptPageIndexOf,
} from "../session-transcript-window";

// Suite: windowed transcript memory model (US-019, UT-104).
// Invariant: pages beyond the keep-window around the reading position are
// released from the infinite query while the head always stays; the last kept
// page still names the older-page request, so a released page reloads through
// the ordinary load-older path. Nothing is released before the position is known.
function page(index: number, size: number): SessionTranscriptData["pages"][number] {
  const base = 10_000 - index * size;
  const entries = Array.from({ length: size }, (_, offset) => {
    const sequence = base - offset;
    return {
      message: {
        id: `msg-${index}-${offset}`,
        role: "assistant",
        parts: [{ type: "text", text: `entry ${sequence}` }],
      } as SessionMessage,
      sequence,
      start_sequence: sequence,
    };
  });
  return {
    cursor: base,
    entries,
    epoch: 1,
    generation: 1,
    has_older: true,
    limit: size,
    max_sequence: base,
    next_before_sequence: base - size,
  };
}

function data(pageCount: number, size = 200): SessionTranscriptData {
  return {
    pages: Array.from({ length: pageCount }, (_, index) => page(index, size)),
    pageParams: Array.from({ length: pageCount }, (_, index) =>
      index === 0 ? undefined : { beforeSequence: 10_000 - index * size, epoch: 1, generation: 1 }
    ),
  };
}

describe("windowed transcript memory", () => {
  it("Should find the page a visible message belongs to", () => {
    const loaded = data(15);
    expect(transcriptPageIndexOf(loaded, "msg-0-3")).toBe(0);
    expect(transcriptPageIndexOf(loaded, "msg-9-120")).toBe(9);
    expect(transcriptPageIndexOf(loaded, "missing")).toBe(-1);
    expect(transcriptPageIndexOf(undefined, "msg-0-0")).toBe(-1);
    expect(loadedTranscriptEntryCount(loaded)).toBe(3_000);
  });

  it("Should release pages beyond the keep-window while the head and the reading position stay", () => {
    const loaded = data(15);
    const nearHead = releaseFarTranscriptPages(loaded, { visiblePageIndex: 0 });
    expect(nearHead.pages).toHaveLength(TRANSCRIPT_KEEP_PAGES);
    expect(nearHead.pageParams).toHaveLength(TRANSCRIPT_KEEP_PAGES);
    expect(nearHead.pages[0]).toBe(loaded.pages[0]);
    expect(loadedTranscriptEntryCount(nearHead)).toBe(TRANSCRIPT_KEEP_PAGES * 200);

    const midway = releaseFarTranscriptPages(loaded, { visiblePageIndex: 6 });
    expect(midway.pages).toHaveLength(6 + TRANSCRIPT_KEEP_PAGES);
    expect(transcriptPageIndexOf(midway, "msg-6-10")).toBe(6);
  });

  it("Should reload a released page through the ordinary older-page request", () => {
    const loaded = data(15);
    const trimmed = releaseFarTranscriptPages(loaded, { visiblePageIndex: 0 });
    expect(releasedPageReloadParam(trimmed)).toEqual({
      beforeSequence: 10_000 - TRANSCRIPT_KEEP_PAGES * 200,
      epoch: 1,
      generation: 1,
    });
  });

  it("Should keep the identity when nothing is far enough to release", () => {
    const loaded = data(4);
    expect(releaseFarTranscriptPages(loaded, { visiblePageIndex: 0 })).toBe(loaded);
    expect(releaseFarTranscriptPages(data(15), { visiblePageIndex: -1 })).toEqual(data(15));
    expect(releaseFarTranscriptPages(loaded, { visiblePageIndex: 3, keepPages: 1 })).toBe(loaded);
  });
});
