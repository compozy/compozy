import { describe, expect, it } from "vitest";

import {
  filterPaletteItems,
  LOOP_PALETTE,
  loopPaletteItems,
  paletteKindKey,
  uniqueNodeId,
} from "../loop-palette";

describe("loop palette", () => {
  it("Should group Action / Control / Source with the reserved action kinds + tool entries", () => {
    const groups = LOOP_PALETTE.map(group => group.label);
    expect(groups).toEqual(["Action", "Control", "Source"]);
    const actionKinds = LOOP_PALETTE[0].items.map(item => item.kindLabel);
    expect(actionKinds).toContain("run-agent");
    expect(actionKinds).toContain("run-loop");
    expect(actionKinds).toContain("transform");
    // The Channel post shortcut is a pre-filled compozy__network_send, not a bespoke kind.
    expect(actionKinds).toContain("compozy__network_send");
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

  it("Should add ask and route to Control without dropping a kind that was already there", () => {
    const control = LOOP_PALETTE.find(group => group.label === "Control")!;
    const kinds = control.items.map(item => item.kindLabel);
    expect(kinds).toContain("ask");
    expect(kinds).toContain("route");

    expect(kinds).toEqual(
      expect.arrayContaining(["fan-out", "collect", "branch", "gate", "sub-loop", "wait"])
    );

    expect(
      LOOP_PALETTE.find(group => group.label === "Action")!.items.map(i => i.kindLabel)
    ).toEqual(expect.arrayContaining(["run-agent", "goal", "run-loop", "transform"]));
  });

  it("Should seed ask and route so a dropped node is valid-shaped from the first render", () => {
    const control = LOOP_PALETTE.find(group => group.label === "Control")!;

    expect(control.items.find(item => item.kindLabel === "route")!.buildRaw("triage")).toEqual({
      id: "triage",
      class: "control",
      kind: "route",
      routes: [],
      default: "",
    });

    expect(control.items.find(item => item.kindLabel === "ask")!.buildRaw("confirm")).toEqual({
      id: "confirm",
      class: "control",
      kind: "ask",
      params: { prompt: "", expect: { type: "object" } },
    });
  });
});

describe("filterPaletteItems", () => {
  const items = loopPaletteItems();

  it("Should return the whole palette for an empty query", () => {
    expect(filterPaletteItems(items, "")).toHaveLength(items.length);
    expect(filterPaletteItems(items, "   ")).toHaveLength(items.length);
  });

  it("Should match on the label, the kind, and the extra keywords alike", () => {
    expect(filterPaletteItems(items, "watch events").map(item => item.kindLabel)).toEqual([
      "watch-events",
    ]);
    expect(filterPaletteItems(items, "compozy__network_send").map(item => item.label)).toEqual([
      "Channel post",
    ]);

    expect(filterPaletteItems(items, "question").map(item => item.kindLabel)).toEqual(["ask"]);
    expect(filterPaletteItems(items, "switch").map(item => item.kindLabel)).toEqual(["route"]);
  });

  it("Should ignore case and surrounding whitespace when matching", () => {
    expect(filterPaletteItems(items, "  ROUTE  ").map(item => item.kindLabel)).toContain("route");
  });

  it("Should return nothing for a query the palette cannot satisfy", () => {
    expect(filterPaletteItems(items, "deploy to production")).toEqual([]);
  });
});

describe("paletteKindKey", () => {
  it("Should resolve the tool placeholder to the generic action glyph key", () => {
    expect(paletteKindKey("tool…")).toBe("");
    expect(paletteKindKey("run-agent")).toBe("run-agent");
    expect(paletteKindKey("route")).toBe("route");
  });
});
