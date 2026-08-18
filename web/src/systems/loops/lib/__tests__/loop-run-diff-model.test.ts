import { describe, expect, it } from "vitest";

import {
  emptyDiffFixture,
  generationDiffFixture,
  runDiffFixture,
} from "../../mocks/fixture-graph-eng-diff";
import { projectLoopDiff } from "../loop-run-diff-model";
import { LOOP_DIFF_CHANGES } from "../loop-request-vocabulary";
import type { LoopDiff } from "../../types";

function diffOf(overrides: Partial<LoopDiff>): LoopDiff {
  return {
    kind: "generation",
    base: { run_id: "looprun_a", generation: 1, status: "done" },
    against: { run_id: "looprun_a", generation: 2, status: "done" },
    inputs: [],
    nodes: [],
    terminal: null,
    ...overrides,
  } as LoopDiff;
}

describe("projectLoopDiff", () => {
  it("Should group rows by change kind in the declared order and drop empty groups", () => {
    const full = projectLoopDiff(generationDiffFixture);
    expect(full.groups.map(group => group.change)).toEqual([...LOOP_DIFF_CHANGES]);
    expect(full.groups.map(group => group.label)).toEqual([
      "Changed",
      "Rerun",
      "Skipped",
      "Carried",
      "Verdict",
    ]);

    const partial = projectLoopDiff(runDiffFixture);
    expect(partial.groups.map(group => group.change)).toEqual(["changed", "skipped"]);
    expect(partial.groups.every(group => group.rows.length > 0)).toBe(true);
  });

  it("Should keep a daemon-summarized value summarized instead of pretending to hold it", () => {
    const view = projectLoopDiff(generationDiffFixture);
    const carried = view.groups.find(group => group.change === "carried")!.rows[0];
    expect(carried.base).toEqual({
      text: "",
      summary: "47.0 KB · sha256:9f21ab",
      isSummarized: true,
      isAbsent: false,
    });

    const changed = view.groups.find(group => group.change === "changed")!.rows[0];
    expect(changed.base).toMatchObject({ text: "standard", isSummarized: false, isAbsent: false });
    expect(changed.against).toMatchObject({ text: "hotfix" });
    expect(changed.cause).toBe("condition_matched");
  });

  it("Should project a side the daemon never sent as absent rather than as empty text", () => {
    const skipped = projectLoopDiff(generationDiffFixture).groups.find(
      group => group.change === "skipped"
    )!.rows[0];
    expect(skipped.base).toEqual({ text: "", summary: "", isSummarized: false, isAbsent: true });
    expect(skipped.against.isAbsent).toBe(true);
    expect(skipped.cause).toBe("route_not_taken");
  });

  it("Should render a structured inline value as JSON instead of coercing it to a label", () => {
    const inputs = projectLoopDiff(runDiffFixture).inputs;
    const services = inputs.find(input => input.key === "services")!;
    expect(services.base.text).toBe('["api","web","worker"]');
    expect(services.changed).toBe(false);
    expect(inputs.find(input => input.key === "severity")?.changed).toBe(true);
  });

  it("Should distinguish an explicit null from an absent diff value", () => {
    const [input] = projectLoopDiff(
      diffOf({ inputs: [{ key: "value", base: { inline: null }, against: {} }] })
    ).inputs;
    expect(input.base).toMatchObject({ text: "null", isAbsent: false });
    expect(input.against.isAbsent).toBe(true);
    expect(input.changed).toBe(true);
  });

  it("Should call a diff empty only when neither a node nor an input differs", () => {
    expect(projectLoopDiff(emptyDiffFixture).isEmpty).toBe(true);
    expect(projectLoopDiff(generationDiffFixture).isEmpty).toBe(false);

    const inputsOnly = projectLoopDiff(
      diffOf({ inputs: [{ key: "severity", base: { inline: "p1" }, against: { inline: "p0" } }] })
    );
    expect(inputsOnly.groups).toEqual([]);
    expect(inputsOnly.isEmpty).toBe(false);

    expect(
      projectLoopDiff(
        diffOf({ inputs: [{ key: "severity", base: { inline: "p1" }, against: { inline: "p1" } }] })
      ).isEmpty
    ).toBe(true);
  });

  it("Should name the live side only while one of the compared runs is still executing", () => {
    expect(projectLoopDiff(emptyDiffFixture).liveSide).toBeNull();
    expect(projectLoopDiff(generationDiffFixture).liveSide).toBe("base");
    expect(
      projectLoopDiff(
        diffOf({
          base: { run_id: "looprun_a", generation: 1, status: "done" },
          against: { run_id: "looprun_b", generation: 1, status: "running" },
        })
      ).liveSide
    ).toBe("against");
  });

  it("Should degrade an unrecognized change word to `changed` rather than throwing", () => {
    const view = projectLoopDiff(
      diffOf({
        nodes: [{ node_id: "mystery", change: "teleported", base: { inline: "a" } }],
      } as Partial<LoopDiff>)
    );
    expect(view.groups).toHaveLength(1);
    expect(view.groups[0].change).toBe("changed");
    expect(view.groups[0].rows[0]).toMatchObject({ nodeId: "mystery", changeLabel: "Changed" });
  });

  it("Should carry the compared identities, divergence flag, and terminal words verbatim", () => {
    const view = projectLoopDiff(runDiffFixture);
    expect(view.kind).toBe("run");
    expect(view.isRunCompare).toBe(true);
    expect(view.baseLabel).toBe("looprun_release_train · generation 3");
    expect(view.againstLabel).toBe("looprun_release_train_fork · generation 2");
    expect(view.hasDefinitionDivergence).toBe(true);
    expect(view.terminalBase).toBe("done");
    expect(view.terminalAgainst).toBe("running");

    const generation = projectLoopDiff(generationDiffFixture);
    expect(generation.isRunCompare).toBe(false);
    expect(generation.hasDefinitionDivergence).toBe(false);
    expect(generation.terminalBase).toBe("running");
  });

  it("Should keep a fan-out lane identifiable on the row it belongs to", () => {
    const rerun = projectLoopDiff(generationDiffFixture).groups.find(
      group => group.change === "rerun"
    )!.rows[0];
    expect(rerun).toMatchObject({ nodeId: "apply-migration", itemIndex: 1 });

    const changed = projectLoopDiff(generationDiffFixture).groups[0].rows[0];
    expect(changed.itemIndex).toBeNull();
  });
});
