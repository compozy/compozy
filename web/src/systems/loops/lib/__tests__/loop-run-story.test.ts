import { describe, expect, it } from "vitest";

import type { LoopTimelineEntry } from "../../types";
// Straight from the module that owns it; the retired story projection only
// re-exported it.
import { humanizeLoopNodeId } from "../loop-run-story-rows";
import { buildStoryBeats } from "../loop-run-story-beats";

describe("humanizeLoopNodeId", () => {
  it("Should collapse mixed separators, trim edges, and space snake and kebab ids", () => {
    expect(humanizeLoopNodeId("fix_batches")).toBe("fix batches");
    expect(humanizeLoopNodeId("archive-event")).toBe("archive event");
    // Consecutive and mixed separators collapse to a single space.
    expect(humanizeLoopNodeId("fix__batches")).toBe("fix batches");
    expect(humanizeLoopNodeId("resolve-_review__threads")).toBe("resolve review threads");
    // Leading and trailing separators are trimmed, not rendered as gaps.
    expect(humanizeLoopNodeId("_watch_pr_")).toBe("watch pr");
  });
});

// The durable story: the paged timeline that survives a reload, where the daemon
// has already written each title and the client's job is register and arithmetic.
// The live SSE narration this file used to cover left with the cockpit that read
// it — nothing renders a client-built story any more.
describe("buildStoryBeats", () => {
  function entry(overrides: Partial<LoopTimelineEntry> & Pick<LoopTimelineEntry, "seq" | "kind">) {
    return {
      at: "2026-08-19T18:43:38Z",
      title: "",
      ...overrides,
    } as LoopTimelineEntry;
  }

  // UT-044
  it("Should render the served meaning title and never leak the event kind", () => {
    const [beat] = buildStoryBeats([
      entry({
        seq: 84,
        kind: "node_failed",
        node_id: "revisor-perf",
        generation: 1,
        attempt: 3,
        title: "step revisor-perf failed",
      }),
    ]);

    expect(beat.title).toBe("step revisor-perf failed");
    expect(beat.title).not.toContain("node_failed");
    expect(beat.tone).toBe("danger");
    expect(beat.icon).toBe("node-failed");
    // The attempt rides along as metadata; it is not a beat of its own.
    expect(beat.attemptLabel).toBe("attempt 3");
    expect(beat.count).toBe(1);
  });

  it("Should write a plain sentence rather than print the kind when a title is missing", () => {
    const [beat] = buildStoryBeats([entry({ seq: 12, kind: "node_quarantined", title: "" })]);
    // Vague but honest beats mechanical and precise: an operator reading
    // "node_quarantined" has been handed the wire, not the story.
    expect(beat.title).toBe("A step was quarantined");
    expect(beat.title).not.toContain("_");
  });

  it("Should fold a coalesced heartbeat run into one beat carrying its count", () => {
    const [beat] = buildStoryBeats([
      entry({ seq: 220, first_seq: 79, kind: "token_tick", title: "progress heartbeats" }),
    ]);

    // 79..220 inclusive. The count is the span, so resuming after seq 220 skips
    // every folded event instead of replaying 142 of them.
    expect(beat.count).toBe(142);
    expect(beat.firstSeq).toBe(79);
    expect(beat.seq).toBe(220);
  });

  it("Should order beats newest first and keep one beat per sequence", () => {
    const beats = buildStoryBeats([
      entry({ seq: 84, kind: "node_succeeded", title: "step revisor-estilo succeeded" }),
      entry({ seq: 90, kind: "gate_verdict", title: "gate approved" }),
      // The seam between the durable page and the live stream overlaps by
      // design; the same sequence arriving twice must still be one beat.
      entry({ seq: 90, kind: "gate_verdict", title: "gate approved" }),
    ]);

    expect(beats.map(beat => beat.seq)).toEqual([90, 84]);
  });

  it("Should carry a fork beat in the informational register", () => {
    const [beat] = buildStoryBeats([
      entry({ seq: 1, kind: "run_forked", title: "forked from round 1 of an earlier run" }),
    ]);
    expect(beat.icon).toBe("forked");
    expect(beat.tone).toBe("info");
  });
});
