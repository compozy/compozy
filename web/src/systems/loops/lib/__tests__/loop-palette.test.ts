import { describe, expect, it } from "vitest";

import { LOOP_PALETTE, uniqueNodeId } from "../loop-palette";

describe("loop palette", () => {
  it("Should group Action / Control / Source with the reserved action kinds + tool entries", () => {
    const groups = LOOP_PALETTE.map(group => group.label);
    expect(groups).toEqual(["Action", "Control", "Source"]);
    const actionKinds = LOOP_PALETTE[0].items.map(item => item.kindLabel);
    expect(actionKinds).toContain("run-agent");
    expect(actionKinds).toContain("run-loop");
    expect(actionKinds).toContain("transform");
    // The Channel post shortcut is a pre-filled agh__network_send, not a bespoke kind.
    expect(actionKinds).toContain("agh__network_send");
  });

  it("Should offer both source watch kinds with valid-shaped seeds", () => {
    const source = LOOP_PALETTE.find(group => group.label === "Source")!;
    const sourceKinds = source.items.map(item => item.kindLabel);
    expect(sourceKinds).toContain("watch-source");
    expect(sourceKinds).toContain("watch-events");
    const watchEvents = source.items.find(item => item.kindLabel === "watch-events")!.buildRaw("w");
    expect(watchEvents).toEqual({ id: "w", class: "source", kind: "watch-events", events: [] });
  });

  it("Should seed a valid-shaped raw node for each palette item", () => {
    for (const group of LOOP_PALETTE) {
      for (const item of group.items) {
        const raw = item.buildRaw("n1");
        expect(raw.id).toBe("n1");
        expect(raw.class).toBe(item.nodeClass);
      }
    }
    const fanOut = LOOP_PALETTE[1].items.find(item => item.kindLabel === "fan-out")!.buildRaw("f");
    expect(fanOut).toMatchObject({ kind: "fan-out", batch_size: 1, max_parallel: 1 });
    const goal = LOOP_PALETTE[0].items.find(item => item.kindLabel === "goal")!.buildRaw("goal");
    expect(goal).toEqual({
      id: "goal",
      class: "action",
      kind: "goal",
      params: {
        agent: "",
        objective: "",
        judge: [{ id: "criterion_1", type: "command", check: "" }],
        max_turns: 20,
        on_exhausted: "halt",
      },
      session: { mode: "continuous" },
      retry: { max_attempts: 2, on_failure: "fresh_session" },
    });
  });

  it("Should generate a unique snake_case node id, suffixing on collision", () => {
    const existing = new Set(["run_agent", "run_agent_2"]);
    expect(uniqueNodeId("run_agent", new Set())).toBe("run_agent");
    expect(uniqueNodeId("run_agent", existing)).toBe("run_agent_3");
    expect(uniqueNodeId("Bad Id!", new Set())).toBe("bad_id_");
  });
});
