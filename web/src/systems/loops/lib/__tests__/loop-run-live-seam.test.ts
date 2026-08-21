import { describe, expect, it } from "vitest";

import {
  advanceTimelineSnapshot,
  emptyTimelineSnapshot,
  latchLoopStreamSeam,
  loopStreamSeam,
  loopTimelineSnapshotKey,
  mergeTimelineEntries,
} from "../loop-run-live-seam";
import type { LoopTimelineEntry, LoopTimelinePage } from "../../types";

function entry(seq: number, overrides: Partial<LoopTimelineEntry> = {}): LoopTimelineEntry {
  return {
    at: `2026-08-21T12:00:${String(seq % 60).padStart(2, "0")}.000Z`,
    kind: "node_succeeded",
    seq,
    title: `Step ${seq} finished`,
    ...overrides,
  };
}

function entries(...seqs: number[]): LoopTimelineEntry[] {
  return seqs.map(seq => entry(seq));
}

function page(rows: LoopTimelineEntry[], headSeq: number, nextCursor?: string): LoopTimelinePage {
  return {
    entries: rows,
    head_seq: headSeq,
    run_id: "looprun-seam",
    ...(nextCursor === undefined ? {} : { next_cursor: nextCursor }),
  };
}

function seqsOf(rows: readonly LoopTimelineEntry[]): number[] {
  return rows.map(row => row.seq);
}

const KEY = loopTimelineSnapshotKey("ws_alpha", "looprun-seam", "notable");

// The no-gap guarantee, at its narrowest point. Everything else about the story
// is paging; this is the one moment where an event can be lost outright.
describe("loopStreamSeam", () => {
  it("Should hold the stream closed until the fence is known", () => {
    // Subscribing before the newest page lands means starting from nothing and
    // silently dropping every event in between.
    expect(loopStreamSeam(null)).toEqual({ afterSequence: undefined, ready: false });
  });

  it("Should resume at the fence the newest page published", () => {
    expect(loopStreamSeam(90)).toEqual({ afterSequence: 90, ready: true });
  });

  it("Should treat a fence of zero as a fence, not as absence", () => {
    // A run with no events yet publishes head_seq 0. Reading that as "unknown"
    // would leave exactly the runs a person is most likely watching unsubscribed.
    expect(loopStreamSeam(0)).toEqual({ afterSequence: 0, ready: true });
  });
});

describe("latchLoopStreamSeam", () => {
  it("Should keep the fence the subscription was opened on", () => {
    // Every lifecycle frame re-reads the newest window at a higher head. Following
    // it would tear down and re-open the stream on each frame — busiest runs
    // reconnecting most.
    expect(latchLoopStreamSeam(90, 140)).toBe(90);
  });

  it("Should latch a fence of zero rather than waiting for a truthy one", () => {
    expect(latchLoopStreamSeam(0, 40)).toBe(0);
    expect(latchLoopStreamSeam(null, 0)).toBe(0);
  });

  it("Should stay unknown until a head exists", () => {
    expect(latchLoopStreamSeam(null, null)).toBeNull();
  });
});

describe("loopTimelineSnapshotKey", () => {
  it("Should be printable text rather than lean on an invisible separator", () => {
    // A key joined with control characters makes the source file binary and hides
    // its own encoding from anyone reading it. Length prefixes are visible ASCII,
    // asserted positively so this file can never carry the bytes it forbids.
    const key = loopTimelineSnapshotKey("ws_alpha", "looprun-seam", "notable");

    expect(key).toBe("8:ws_alpha12:looprun-seam7:notable");
    expect(key).toMatch(/^[\w:-]+$/u);
  });

  it("Should keep two runs apart even when an id contains the separator", () => {
    // Any delimiter can also occur inside an id, and then two different runs
    // quietly share a snapshot — one inheriting the other's beats.
    expect(loopTimelineSnapshotKey("ws", "a:b", "notable")).not.toBe(
      loopTimelineSnapshotKey("ws", "a", "b:notable")
    );
    expect(loopTimelineSnapshotKey("ws-a", "b", "notable")).not.toBe(
      loopTimelineSnapshotKey("ws", "a-b", "notable")
    );
  });
});

describe("mergeTimelineEntries", () => {
  it("Should keep history a refetch slid out from under the reader", () => {
    // A refetch re-reads the newest window unpinned and re-derives every backward
    // cursor from it, so the loaded window shifts up by whatever appended and the
    // oldest pages fall off. Somebody who paged back to the run's start would
    // watch those beats disappear.
    const held = mergeTimelineEntries([], entries(10, 9, 8, 7, 6, 5, 4, 3, 2, 1));
    const merged = mergeTimelineEntries(held, entries(16, 15, 14, 13, 12, 11, 10, 9, 8, 7));

    expect(seqsOf(merged)).toEqual([16, 15, 14, 13, 12, 11, 10, 9, 8, 7, 6, 5, 4, 3, 2, 1]);
  });

  it("Should render each sequence exactly once, newest first", () => {
    const merged = mergeTimelineEntries(entries(5, 4, 3), entries(6, 5, 4));

    expect(seqsOf(merged)).toEqual([6, 5, 4, 3]);
    expect(new Set(seqsOf(merged)).size).toBe(merged.length);
  });

  it("Should prefer the daemon's newest authoring of a sequence", () => {
    // Titles are server-authored. A re-read that re-words one has to win, or the
    // page keeps showing a sentence the daemon no longer stands behind.
    const merged = mergeTimelineEntries(
      [entry(7, { title: "Step 7 finished" })],
      [entry(7, { title: "Step 7 finished after 2 attempts" })]
    );

    expect(merged[0].title).toBe("Step 7 finished after 2 attempts");
  });

  it("Should not print a coalesced run twice when it grows", () => {
    // A coalesced heartbeat run republishes the same window under a higher `seq`
    // with the same `first_seq`. Keeping both would print those heartbeats twice.
    const held = [entry(10, { first_seq: 5 }), entry(4)];
    const merged = mergeTimelineEntries(held, [entry(14, { first_seq: 5 })]);

    expect(seqsOf(merged)).toEqual([14, 4]);
  });

  it("Should return the same list when a refetch changed nothing", () => {
    // Identity is load-bearing: the hook adjusts state during render, so an
    // unchanged merge has to come back as the very same array. These are fresh
    // objects carrying identical facts — exactly what a refetch hands back — and
    // calling that "changed" would set state on every render.
    const held = mergeTimelineEntries([], entries(3, 2, 1));

    expect(mergeTimelineEntries(held, entries(3, 2, 1))).toBe(held);
  });
});

describe("advanceTimelineSnapshot", () => {
  it("Should latch the fence at the first head and grow the story across refetches", () => {
    const first = advanceTimelineSnapshot(emptyTimelineSnapshot(KEY), KEY, [
      page(entries(10, 9, 8, 7, 6), 10, "cursor-older"),
      page(entries(5, 4, 3, 2, 1), 10),
    ]);
    expect(first.fence).toBe(10);
    expect(seqsOf(first.entries)).toEqual([10, 9, 8, 7, 6, 5, 4, 3, 2, 1]);

    const afterFrames = advanceTimelineSnapshot(first, KEY, [
      page(entries(16, 15, 14, 13, 12), 16, "cursor-older-2"),
      page(entries(11, 10, 9, 8, 7), 16),
    ]);

    expect(afterFrames.fence).toBe(10);
    expect(seqsOf(afterFrames.entries)).toEqual([
      16, 15, 14, 13, 12, 11, 10, 9, 8, 7, 6, 5, 4, 3, 2, 1,
    ]);
  });

  it("Should start over for a different run rather than inherit its beats", () => {
    const loaded = advanceTimelineSnapshot(emptyTimelineSnapshot(KEY), KEY, [
      page(entries(3, 2, 1), 3),
    ]);
    const otherKey = loopTimelineSnapshotKey("ws_alpha", "looprun-other", "notable");

    const switched = advanceTimelineSnapshot(loaded, otherKey, [page(entries(9), 9)]);

    expect(seqsOf(switched.entries)).toEqual([9]);
    expect(switched.fence).toBe(9);
  });

  it("Should hold its identity when a refetch changed nothing", () => {
    const pages = [page(entries(2, 1), 2)];
    const loaded = advanceTimelineSnapshot(emptyTimelineSnapshot(KEY), KEY, pages);

    expect(advanceTimelineSnapshot(loaded, KEY, pages)).toBe(loaded);
  });

  it("Should stay fenceless for a run whose reads have not landed", () => {
    const empty = advanceTimelineSnapshot(emptyTimelineSnapshot(KEY), KEY, []);

    expect(empty.fence).toBeNull();
    expect(empty.entries).toEqual([]);
  });
});
