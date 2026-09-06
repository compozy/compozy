import { describe, expect, it } from "vitest";

import {
  FIND_SEED_MAX_CHARS,
  findCountLabel,
  findMatchSpeaker,
  findShortcutSeed,
  findSnippetSegments,
  isFindShortcut,
  isSequenceLoaded,
  navigationRefreshKey,
  normalizeTranscriptSearchQuery,
  reconcileFindMatches,
  stepFindMatch,
  TRAIL_HIT_HEIGHT_PX,
  TRAIL_MAX_GAP_PX,
  TRAIL_MIN_GAP_PX,
  TRAIL_TICK_WIDTH_HOVER_PX,
  TRAIL_TICK_WIDTH_PX,
  trailAnchorSequence,
  trailGap,
  trailKeyTarget,
  trailLayout,
  trailTickLabel,
  trailTickState,
  trailTickWidth,
} from "../session-navigation";
import { sessionFindLogic } from "../session-navigation-find-store";
import { caseFoldForSearch, findFoldedOccurrences } from "../session-find-text";
import {
  findMatchesInsideFolds,
  findMatchSource,
  indexTranscriptSequences,
  landingMessageId,
  visibleSequenceWindow,
} from "../session-navigation-transcript";
import type {
  NormalizedSessionTranscriptEntry,
  SessionTranscriptOutlineEntry,
  SessionTranscriptSearchMatch,
} from "../../types";

// Suite: session navigation derivations (UT-111 trail derivation + scale compression; find
// read model and interaction state for US-028).
// Invariant: trail layout depends only on the count and the pane height and every tick keeps
// its own hit target; the anchor is the last sent message at or above the viewport top; find
// steps wrap, a refreshed list keeps the active match by identity and counts appended hits
// without moving anything, and a jump into unloaded history is blocked from stepping until
// the page lands.
// Owning layer: session navigation lib. Canonical suite: this file.
// Boundary IN: search/outline payloads, host-measured ranges. Boundary OUT: find bar, trail.
function match(sequence: number, snippet = `hit ${sequence}`): SessionTranscriptSearchMatch {
  return { role: "assistant", sequence, snippet, turn_id: `turn-${sequence}` };
}

function entry(sequence: number, preview = `ask ${sequence}`): SessionTranscriptOutlineEntry {
  return {
    at: "2026-09-06T14:00:00Z",
    preview,
    reply_preview: `reply ${sequence}`,
    sequence,
    turn_id: `turn-${sequence}`,
  };
}

describe("find derivations", () => {
  it("Should glaze every case-insensitive literal hit in a snippet", () => {
    expect(findSnippetSegments("Run the Lifecycle tests; lifecycle first", "lifecycle")).toEqual([
      { match: false, text: "Run the " },
      { match: true, text: "Lifecycle" },
      { match: false, text: " tests; " },
      { match: true, text: "lifecycle" },
      { match: false, text: " first" },
    ]);
    expect(findSnippetSegments("no hit here", "zzz")).toEqual([
      { match: false, text: "no hit here" },
    ]);
    expect(findSnippetSegments("", "x")).toEqual([]);
    expect(findSnippetSegments("plain", "   ")).toEqual([{ match: false, text: "plain" }]);
  });

  it("Should name the speaker in the reader's words and label the count for the whole session", () => {
    expect(findMatchSpeaker("user", "Claude Code")).toBe("You");
    expect(findMatchSpeaker("assistant", "Claude Code")).toBe("Claude Code");
    expect(findMatchSpeaker("assistant", " ")).toBe("Agent");
    expect(findMatchSpeaker("tool", "x")).toBe("Tool");
    expect(findCountLabel(-1, 0, false)).toBe("No matches");
    expect(findCountLabel(-1, 1, false)).toBe("1 match");
    expect(findCountLabel(2, 12, false)).toBe("3 of 12");
    expect(findCountLabel(2, 200, true)).toBe("3 of 200+");
  });

  it("Should step through matches with wrap and land on the ends from nothing active", () => {
    const matches = [match(2), match(5), match(9)];
    expect(stepFindMatch(matches, null, 1)).toBe(2);
    expect(stepFindMatch(matches, null, -1)).toBe(9);
    expect(stepFindMatch(matches, 5, 1)).toBe(9);
    expect(stepFindMatch(matches, 9, 1)).toBe(2);
    expect(stepFindMatch(matches, 2, -1)).toBe(9);
    expect(stepFindMatch([], 2, 1)).toBeNull();
  });

  it("Should keep the active match by identity across a refresh and count appended hits (US-028.EC-2)", () => {
    const before = [match(2), match(5)];
    const after = [match(2), match(5), match(11), match(14)];
    expect(reconcileFindMatches(before, after, 5)).toEqual({ activeSequence: 5, appended: 2 });
    // The active match vanished (history rewritten): nothing is active, nothing is "new" on a first list.
    expect(reconcileFindMatches(before, [match(3)], 5)).toEqual({
      activeSequence: null,
      appended: 0,
    });
    expect(reconcileFindMatches([], after, null)).toEqual({ activeSequence: null, appended: 0 });
  });

  it("Should bound the literal query to the route's contract before it leaves", () => {
    expect(normalizeTranscriptSearchQuery("  lifecycle ")).toBe("lifecycle");
    expect(normalizeTranscriptSearchQuery("   ")).toBe("");
    expect(normalizeTranscriptSearchQuery("é".repeat(2_100))).toBe("");
    expect(isSequenceLoaded({ from: 10, to: 40 }, 10)).toBe(true);
    expect(isSequenceLoaded({ from: 10, to: 40 }, 9)).toBe(false);
    expect(isSequenceLoaded(null, 9)).toBe(false);
  });

  it("Should block stepping while a jump is loading older history and resolve it on landing", async () => {
    const store = sessionFindLogic.createStore();
    let release!: (reached: boolean) => void;
    const handlers = {
      isSequenceLoaded: (sequence: number) => sequence >= 100,
      jumpToSequence: () => undefined,
      loadOlderUntil: () =>
        new Promise<boolean>(resolve => {
          release = resolve;
        }),
    };
    store.trigger.matchesObserved({ matches: [match(5), match(120), match(130)] });
    store.trigger.stepped({ direction: 1, handlers });
    expect(store.getSnapshot().context).toMatchObject({
      activeSequence: 5,
      jump: { phase: "loading", sequence: 5 },
    });
    // No jumping on a stale set: the step is dropped until the page lands.
    store.trigger.stepped({ direction: 1, handlers });
    expect(store.getSnapshot().context.activeSequence).toBe(5);

    release(true);
    await Promise.resolve();
    await Promise.resolve();
    expect(store.getSnapshot().context.jump).toBeNull();
    store.trigger.stepped({ direction: 1, handlers });
    expect(store.getSnapshot().context).toMatchObject({
      activeSequence: 120,
      jump: { phase: "landing", sequence: 120 },
    });
  });
});

describe("trail derivations (UT-111)", () => {
  it("Should shrink the gap from 9px to 3px as the count grows, from the count alone", () => {
    expect(trailGap(1)).toBe(TRAIL_MAX_GAP_PX);
    expect(trailGap(12)).toBe(TRAIL_MAX_GAP_PX);
    expect(trailGap(24)).toBeLessThan(TRAIL_MAX_GAP_PX);
    expect(trailGap(24)).toBeGreaterThan(TRAIL_MIN_GAP_PX);
    expect(trailGap(48)).toBe(TRAIL_MIN_GAP_PX);
    expect(trailGap(400)).toBe(TRAIL_MIN_GAP_PX);
  });

  it("Should lay ticks out at the natural pitch while they fit and spread them proportionally past the pane share", () => {
    const few = trailLayout(6, 600);
    expect(few.compressed).toBe(false);
    expect(few.offsets).toEqual([0, 17, 34, 51, 68, 85]);
    expect(few.height).toBe(85 + TRAIL_HIT_HEIGHT_PX);

    const many = trailLayout(60, 220);
    expect(many.compressed).toBe(true);
    expect(many.height).toBe(176);
    expect(many.offsets).toHaveLength(60);
    expect(many.offsets[0]).toBe(0);
    expect(many.offsets[59]).toBe(176 - TRAIL_HIT_HEIGHT_PX);
    // Strictly increasing: every tick keeps its own slot even when compressed.
    for (let index = 1; index < many.offsets.length; index += 1) {
      expect(many.offsets[index]).toBeGreaterThanOrEqual(many.offsets[index - 1]!);
    }
    expect(trailLayout(0, 400)).toEqual({ compressed: false, height: 0, offsets: [], pitch: 0 });
    expect(trailLayout(1, 400).offsets).toEqual([0]);
  });

  it("Should widen ticks toward the pointer in a Gaussian window and leave colour to the state", () => {
    expect(trailTickWidth(40, null, 17)).toBe(TRAIL_TICK_WIDTH_PX);
    expect(trailTickWidth(40, 40, 17)).toBe(TRAIL_TICK_WIDTH_HOVER_PX);
    const near = trailTickWidth(57, 40, 17);
    const far = trailTickWidth(120, 40, 17);
    expect(near).toBeGreaterThan(far);
    expect(far).toBe(TRAIL_TICK_WIDTH_PX);
    // A dense rail widens the window (σ grows with the pitch, capped at 22).
    expect(trailTickWidth(40 + 20, 40, 3)).toBeGreaterThan(TRAIL_TICK_WIDTH_PX);
  });

  it("Should mark the anchor as the last sent message at or above the viewport top", () => {
    const entries = [entry(3), entry(20), entry(41), entry(90)];
    expect(trailAnchorSequence(entries, 45)).toBe(41);
    expect(trailAnchorSequence(entries, 3)).toBe(3);
    expect(trailAnchorSequence(entries, 1)).toBeNull();
    expect(trailAnchorSequence(entries, null)).toBeNull();
    expect(trailTickState(entry(41), 41, { from: 30, to: 50 })).toBe("anchor");
    expect(trailTickState(entry(20), 41, { from: 15, to: 50 })).toBe("in-view");
    expect(trailTickState(entry(90), 41, { from: 15, to: 50 })).toBe("rest");
  });

  it("Should rove with arrows, Home and End and name ticks by ordinal and first line", () => {
    expect(trailKeyTarget("ArrowDown", 0, 4)).toBe(1);
    expect(trailKeyTarget("ArrowUp", 0, 4)).toBe(0);
    expect(trailKeyTarget("End", 0, 4)).toBe(3);
    expect(trailKeyTarget("Home", 3, 4)).toBe(0);
    expect(trailKeyTarget("Enter", 1, 4)).toBeNull();
    expect(trailKeyTarget("ArrowDown", 0, 0)).toBeNull();
    expect(trailTickLabel(entry(3, "\n Ship it\nwith tests"), 2)).toBe("Message 2: Ship it");
    expect(trailTickLabel(entry(3, "  "), 2)).toBe("Message 2");
  });
});

// Suite: navigation host derivations (task_08 host integration).
// Invariant: the loaded pages index to a sequence range/row map; a match's source is the part
// and field the daemon named (`part_index`/`field`) read from the loaded projected parts — never
// guessed from the snippet — and decides fold placement and which body must open; a jump lands
// on the entry or the nearest loaded row above it; the refresh key moves on fences/count/settling
// only; ⌘F/Ctrl+F seeds one line; matching folds per code point like the daemon.
// Owning layer: session navigation lib. Canonical suite: this file.
describe("navigation host derivations", () => {
  const entries: NormalizedSessionTranscriptEntry[] = [
    {
      message: {
        id: "m-10",
        parts: [{ text: "Refactor the lifecycle tests", turn_id: "turn-a", type: "text" }],
        role: "user",
      } as unknown as NormalizedSessionTranscriptEntry["message"],
      sequence: 10,
      start_sequence: 10,
    },
    {
      message: {
        id: "m-12",
        parts: [
          { text: "Check the lifecycle channel.", turn_id: "turn-a", type: "reasoning" },
          {
            output: { content: "go test ./lifecycle" },
            timestamp: "2026-09-06T12:00:03Z",
            toolCallId: "call-bash",
            turn_id: "turn-a",
            type: "tool-Bash",
          },
          {
            text: "All green.",
            timestamp: "2026-09-06T12:00:08Z",
            turn_id: "turn-a",
            type: "text",
          },
        ],
        role: "assistant",
      } as unknown as NormalizedSessionTranscriptEntry["message"],
      sequence: 12,
      start_sequence: 12,
    },
    {
      message: { id: "m-20", parts: [], role: "system" },
      sequence: 20,
      start_sequence: 20,
    },
  ];

  it("Should index loaded entries by sequence and message with the turn, parts and time", () => {
    const index = indexTranscriptSequences(entries);
    expect(index.range).toEqual({ from: 10, to: 20 });
    expect(index.sequenceOfMessage.get("m-12")).toBe(12);
    expect(index.bySequence.get(12)).toMatchObject({
      at: Date.parse("2026-09-06T12:00:03Z"),
      messageId: "m-12",
      turnId: "turn-a",
    });
    expect(index.bySequence.get(12)?.parts).toHaveLength(3);
    // No part names a turn: the message id stands in, as the timeline does.
    expect(index.bySequence.get(20)?.turnId).toBe("m-20");
    expect(indexTranscriptSequences([]).range).toBeNull();
  });

  it("Should locate a match in the part and field the daemon named and decide what must open", () => {
    const index = indexTranscriptSequences(entries);
    const agent = index.bySequence.get(12);
    expect(findMatchSource({ field: "output", part_index: 1 }, agent)).toEqual({
      field: "output",
      insideFold: true,
      kind: "tool",
      opensBody: true,
      partIndex: 1,
      toolCallId: "call-bash",
    });
    expect(findMatchSource({ field: "tool_name", part_index: 1 }, agent)).toMatchObject({
      insideFold: true,
      opensBody: false,
    });
    expect(findMatchSource({ field: "text", part_index: 0 }, agent)).toMatchObject({
      insideFold: true,
      kind: "reasoning",
      opensBody: true,
      toolCallId: null,
    });
    // The terminal answer stays visible below the fold.
    expect(findMatchSource({ field: "text", part_index: 2 }, agent)).toMatchObject({
      insideFold: false,
      kind: "text",
      opensBody: false,
    });
    // The operator's own ask never folds.
    expect(
      findMatchSource({ field: "text", part_index: 0 }, index.bySequence.get(10))
    ).toMatchObject({ insideFold: false, kind: "text" });
    // No source (older daemon), an index past the loaded parts, or an unloaded entry: nothing is claimed.
    expect(findMatchSource({}, agent)).toBeNull();
    expect(findMatchSource({ field: "text", part_index: null }, agent)).toBeNull();
    expect(findMatchSource({ field: "output", part_index: 7 }, agent)).toBeNull();
    expect(findMatchSource({ field: "output", part_index: 1 }, undefined)).toBeNull();
  });

  it("Should count matches behind each fold by the daemon's turn id from located sources only", () => {
    const index = indexTranscriptSequences(entries);
    const work = {
      field: "output",
      part_index: 1,
      role: "assistant",
      sequence: 12,
      snippet: "…go test…",
      turn_id: "turn-a",
    };
    const thought = {
      field: "text",
      part_index: 0,
      role: "assistant",
      sequence: 12,
      snippet: "lifecycle channel",
      turn_id: "turn-a",
    };
    const prose = {
      field: "text",
      part_index: 2,
      role: "assistant",
      sequence: 12,
      snippet: "All green.",
      turn_id: "turn-a",
    };
    const legacy = {
      role: "assistant",
      sequence: 12,
      snippet: "go test ./lifecycle",
      turn_id: "turn-a",
    };
    const ask = {
      field: "text",
      part_index: 0,
      role: "user",
      sequence: 10,
      snippet: "lifecycle",
      turn_id: "turn-a",
    };
    expect(findMatchesInsideFolds([work, thought, prose, legacy, ask], index)).toEqual(
      new Map([["turn-a", 2]])
    );
  });

  it("Should land on the entry's row or the nearest loaded row above an unknown cursor", () => {
    const index = indexTranscriptSequences(entries);
    expect(landingMessageId(index, 12)).toBe("m-12");
    expect(landingMessageId(index, 15)).toBe("m-12");
    expect(landingMessageId(index, 5)).toBeNull();
    expect(visibleSequenceWindow(index, "m-10", "m-20")).toEqual({
      range: { from: 10, to: 20 },
      top: 10,
    });
    expect(visibleSequenceWindow(index, null, "m-20")).toEqual({ range: null, top: null });
    expect(visibleSequenceWindow(index, "m-12", "unknown")).toEqual({
      range: { from: 12, to: 12 },
      top: 12,
    });
  });

  it("Should move the refresh key on fences, entry count and settling only", () => {
    const base = { entryCount: 3, epoch: 1, generation: 2, streamTick: 0, streaming: false };
    const settled = navigationRefreshKey(base);
    expect(navigationRefreshKey(base)).toBe(settled);
    expect(navigationRefreshKey({ ...base, generation: 3 })).not.toBe(settled);
    expect(navigationRefreshKey({ ...base, entryCount: 4 })).not.toBe(settled);
    const live = navigationRefreshKey({ ...base, streaming: true });
    expect(live).not.toBe(settled);
    expect(navigationRefreshKey({ ...base, streaming: true })).toBe(live);
    expect(navigationRefreshKey({ ...base, streamTick: 1, streaming: true })).not.toBe(live);
  });

  it("Should recognise the find shortcut and seed one bounded line from the selection", () => {
    const keys = { altKey: false, ctrlKey: false, metaKey: false, shiftKey: false };
    expect(isFindShortcut({ ...keys, key: "f", metaKey: true })).toBe(true);
    expect(isFindShortcut({ ...keys, key: "F", ctrlKey: true })).toBe(true);
    expect(isFindShortcut({ ...keys, key: "f" })).toBe(false);
    expect(isFindShortcut({ ...keys, key: "f", metaKey: true, shiftKey: true })).toBe(false);
    expect(isFindShortcut({ ...keys, key: "f", ctrlKey: true, metaKey: true })).toBe(false);
    expect(findShortcutSeed("  lifecycle tests \nsecond line")).toBe("lifecycle tests");
    expect(findShortcutSeed(null)).toBe("");
    expect(findShortcutSeed("x".repeat(FIND_SEED_MAX_CHARS + 1))).toBe("");
  });

  it("Should fold per code point like the daemon and cut occurrences in the original text", () => {
    // U+0130 lower-cases to two code points in JavaScript; the daemon keeps one.
    const dotted = "İstanbul lifecycle";
    expect(caseFoldForSearch(dotted).folded).toBe("istanbul lifecycle");
    expect(findFoldedOccurrences(dotted, "LIFECYCLE")).toEqual([{ end: 18, start: 9 }]);
    expect(findFoldedOccurrences(dotted, "istanbul")).toEqual([{ end: 8, start: 0 }]);
    // A final sigma is not a plain sigma for the daemon either: no context-sensitive folding.
    expect(findFoldedOccurrences("ΟΔΥΣΣΕΥΣ", "σ")).toEqual([
      { end: 4, start: 3 },
      { end: 5, start: 4 },
      { end: 8, start: 7 },
    ]);
    expect(findFoldedOccurrences("Οδυσσεύς", "σ")).toEqual([
      { end: 4, start: 3 },
      { end: 5, start: 4 },
    ]);
    // Astral code points keep UTF-16 widths straight.
    expect(findFoldedOccurrences("🚀 Lifecycle", "lifecycle")).toEqual([{ end: 12, start: 3 }]);
    expect(findFoldedOccurrences("abc", "")).toEqual([]);
    expect(findSnippetSegments("İstanbul: lifecycle", "LIFECYCLE")).toEqual([
      { match: false, text: "İstanbul: " },
      { match: true, text: "lifecycle" },
    ]);
  });
});
