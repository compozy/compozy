import { describe, expect, it } from "vitest";

import {
  buildGenerationHistory,
  formatLoopScore,
  loopGenerationOriginLabel,
  loopRunBestLabel,
} from "../loop-generation-presentation";
import type { LoopRosterNode, LoopRunGeneration } from "../../types";

function generation(overrides: Partial<LoopRunGeneration> = {}): LoopRunGeneration {
  return {
    generation: 1,
    origin: "initial",
    parent_generation: 0,
    outputs: [],
    route_causes: [],
    verdicts: [],
    ...overrides,
  } as LoopRunGeneration;
}

function verdict(
  outcome: LoopRunGeneration["verdicts"][number]["outcome"],
  score?: number
): LoopRunGeneration["verdicts"][number] {
  return {
    blocking_issues: [],
    criteria: [],
    gate_id: "quality",
    item_index: 0,
    outcome,
    ...(score === undefined ? {} : { score }),
  } as LoopRunGeneration["verdicts"][number];
}

function node(overrides: Partial<LoopRosterNode> = {}): LoopRosterNode {
  return {
    generation: 1,
    node_id: "implementar",
    item_index: 0,
    state: "succeeded",
    attempt: 1,
    attempts: [],
    started_at: "2026-08-19T18:40:00Z",
    ended_at: "2026-08-19T18:41:00Z",
    usage: { tokens: 10_000 },
    ...overrides,
  } as LoopRosterNode;
}

describe("loopGenerationOriginLabel", () => {
  it("Should name where a round came from in words", () => {
    expect(loopGenerationOriginLabel("initial", 0)).toBe("Initial generation");
    expect(loopGenerationOriginLabel("ratchet_restore", 2)).toBe("Restored from gen 2");
  });
});

describe("formatLoopScore and loopRunBestLabel", () => {
  it("Should format a persisted score and leave an absent one absent", () => {
    expect(formatLoopScore(0.9)).toBe("0.90");
    expect(formatLoopScore(undefined)).toBeNull();
  });

  it("Should read the daemon's best generation without recomputing one", () => {
    expect(loopRunBestLabel({ best_generation: 2, best_score: 0.95 })).toBe("Gen 2 · 0.95");
    expect(loopRunBestLabel({ best_generation: null, best_score: null })).toBeNull();
  });
});

describe("buildGenerationHistory", () => {
  it("Should never print a raw verdict enum", () => {
    // `invalid_output` is a wire value. Spacing it into "invalid output" is still
    // the enum, and it is not a sentence anybody wrote (UT-044).
    const [row] = buildGenerationHistory({
      generations: [generation({ verdicts: [verdict("invalid_output")] })],
      nodes: [],
    });

    expect(row.outcomeLabel).toBe("the output did not match its schema");
    expect(row.outcomeLabel).not.toContain("invalid_output");
    expect(row.tone).toBe("danger");
  });

  it("Should read a fully approved round as accepted and a mixed one as partly", () => {
    const [accepted] = buildGenerationHistory({
      generations: [generation({ verdicts: [verdict("approved"), verdict("approved")] })],
      nodes: [],
    });
    expect(accepted.outcomeLabel).toBe("accepted");

    const [mixed] = buildGenerationHistory({
      generations: [generation({ verdicts: [verdict("approved"), verdict("rejected")] })],
      nodes: [],
    });
    expect(mixed.outcomeLabel).toBe("partly accepted — 1 of 2");
  });

  it("Should give a loop with no scoring no score at all", () => {
    // A column of dashes invites the reader to wonder what they are missing.
    const [unscored] = buildGenerationHistory({
      generations: [generation({ verdicts: [verdict("approved")] })],
      nodes: [],
    });
    expect(unscored.scoreLabel).toBeNull();

    const [scored] = buildGenerationHistory({
      generations: [generation({ verdicts: [verdict("approved", 0.87)] })],
      nodes: [],
    });
    expect(scored.scoreLabel).toBe("0.87");
  });

  it("Should sum the round's usage from its own steps and label the cost an estimate", () => {
    // The run total answers "what has this cost"; only the roster answers "what
    // did this round cost".
    const [row] = buildGenerationHistory({
      generations: [generation()],
      nodes: [
        node({ usage: { tokens: 20_000 } }),
        node({ item_index: 1, usage: { tokens: 8_000 } }),
      ],
    });

    expect(row.tokens).toBe(28_000);
    expect(row.costLabel).toBe("~$0.14");
  });

  it("Should call a round interrupted when the run ended holding an unsettled step", () => {
    // A crash or a forced cancellation leaves a round mid-flight. Dressing that as settled is
    // the most misleading thing this view could do (US-013.EC-1).
    //
    // The settled sibling is the point: it carries a real `ended_at`, so a view
    // that reports "the round's last end" would print 18:41 and read as if the
    // round finished there. It did not — the run died holding the other step
    // open, and an interrupted round has no ending to show.
    const [row] = buildGenerationHistory({
      generations: [generation()],
      nodes: [
        node({ ended_at: "2026-08-19T18:41:00Z" }),
        node({ item_index: 1, state: "running", ended_at: null }),
      ],
      runStatus: "failed",
    });

    expect(row.progressState).toBe("interrupted");
    expect(row.endedAt).toBeNull();
  });

  it("Should call the same round in-progress while the run is still live", () => {
    const [row] = buildGenerationHistory({
      generations: [generation()],
      nodes: [node({ state: "running", ended_at: null })],
      runStatus: "running",
    });

    expect(row.progressState).toBe("in-progress");
    expect(row.endedAt).toBeNull();
  });

  it("Should call a round partial when some steps settled and others failed", () => {
    const [row] = buildGenerationHistory({
      generations: [generation()],
      nodes: [node(), node({ item_index: 1, node_id: "revisar", state: "failed" })],
      runStatus: "failed",
    });

    expect(row.progressState).toBe("partial");
  });

  it("Should end a settled round at its last step rather than at a scheduled retry", () => {
    const [row] = buildGenerationHistory({
      generations: [generation()],
      nodes: [node(), node({ item_index: 1, ended_at: "2026-08-19T18:45:00Z" })],
      runStatus: "done",
    });

    expect(row.progressState).toBe("settled");
    expect(row.endedAt).toBe("2026-08-19T18:45:00Z");
  });

  it("Should list newest round first", () => {
    const rows = buildGenerationHistory({
      generations: [generation({ generation: 1 }), generation({ generation: 3 })],
      nodes: [],
    });

    expect(rows.map(row => row.generation)).toEqual([3, 1]);
  });
});
