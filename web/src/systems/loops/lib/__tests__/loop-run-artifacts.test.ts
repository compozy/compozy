import { describe, expect, it } from "vitest";

import type { LoopBriefing } from "../../types";
import { buildRunOutcome } from "../loop-run-artifacts";

function briefing(overrides: Partial<LoopBriefing> = {}): LoopBriefing {
  return {
    run_id: "looprun-77aa01b2c3d4e5f6",
    status: "done",
    tone: "ok",
    headline: "The run finished and wrote post-final.md",
    blockers: [],
    artifacts: [],
    progress: { round: 2, steps_done: 6, steps_total: 6 },
    usage: { tokens: 214_500 },
    ...overrides,
  } as LoopBriefing;
}

describe("buildRunOutcome", () => {
  // UT-043: retention removes bytes, not the fact that the run produced
  // something. Dropping the row would quietly rewrite what the run did.
  it("Should keep a pruned artifact and say plainly that its content is gone", () => {
    const model = buildRunOutcome(
      briefing({
        artifacts: [
          { name: "post-final.md", output: "saida", availability: "pruned", ref: "blob-2f81" },
        ],
      })
    );

    expect(model.artifacts).toHaveLength(1);
    const artifact = model.artifacts[0];
    expect(artifact.name).toBe("post-final.md");
    expect(artifact.output).toBe("saida");
    expect(artifact.note).toBe("Content no longer stored");
    // Nothing is behind it any more, so it offers no link to follow. A dead
    // link would be worse than none.
    expect(artifact.ref).toBeNull();
    expect(model.producedNothing).toBe(false);
  });

  it("Should label a partial artifact as partial and keep its link", () => {
    const model = buildRunOutcome(
      briefing({
        artifacts: [
          { name: "draft.md", output: "saida", availability: "partial", ref: "blob-9c02" },
        ],
      })
    );

    expect(model.artifacts[0].availability).toBe("partial");
    expect(model.artifacts[0].note).toContain("Partial");
    expect(model.artifacts[0].toneForNote).toBe("warning");
    expect(model.artifacts[0].ref).toBe("blob-9c02");
  });

  it("Should carry an available artifact without a note", () => {
    const model = buildRunOutcome(
      briefing({
        artifacts: [
          { name: "post-final.md", output: "saida", availability: "available", ref: "blob-2f81" },
        ],
      })
    );
    expect(model.artifacts[0].note).toBeNull();
    expect(model.artifacts[0].toneForNote).toBeNull();
  });

  it("Should record who ended a canceled run and when", () => {
    const model = buildRunOutcome(
      briefing({
        status: "canceled",
        outcome: {
          status: "canceled",
          cause: "operator stopped the run",
          at: "2026-08-19T18:12:00Z",
          actor_kind: "user",
          actor_ref: "pedro",
        },
      })
    );

    expect(model.outcome?.label).toBe("Canceled");
    expect(model.outcome?.actorLabel).toBe("pedro");
    expect(model.outcome?.at).toBe("2026-08-19T18:12:00Z");
    // A canceled run that produced nothing says so; it never implies outputs.
    expect(model.producedNothing).toBe(true);
  });

  it("Should not claim a live run produced nothing", () => {
    const model = buildRunOutcome(briefing({ status: "running", outcome: null }));
    expect(model.outcome).toBeNull();
    // "Produced nothing" is a terminal statement. A running run has simply not
    // produced anything *yet*, which is a different sentence entirely.
    expect(model.producedNothing).toBe(false);
  });
});
