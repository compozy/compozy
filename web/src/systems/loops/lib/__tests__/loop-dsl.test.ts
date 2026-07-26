import { describe, expect, it } from "vitest";

import { loopDetailByName } from "../../mocks/fixtures";
import type { LoopValidationIssue } from "../../types";
import { buildDslView } from "../loop-dsl";

const def = loopDetailByName.get("software-delivery")!.definition as unknown as Record<
  string,
  unknown
>;

describe("loop dsl view", () => {
  it("Should render the agh.loop/v1 document with top-level blocks and the graph nodes", () => {
    const lines = buildDslView(def, new Map());
    const text = lines.map(line => line.text).join("\n");
    expect(text).toContain("apiVersion: agh.loop/v1");
    expect(text).toContain("graph:");
    expect(text).toContain("nodes:");
    expect(text).toContain("id: implement");
    expect(lines.every(line => line.highlight === false)).toBe(true);
  });

  it("Should highlight a flagged node's block and mark its offending field line", () => {
    const byNode = new Map<string, LoopValidationIssue[]>([
      [
        "implement",
        [
          {
            node_id: "implement",
            code: "fan_out_ceiling_exceeded",
            message: "",
            severity: "error",
          },
        ],
      ],
    ]);
    const lines = buildDslView(def, byNode);
    const highlighted = lines.filter(line => line.highlight);
    expect(highlighted.length).toBeGreaterThan(0);
    expect(
      highlighted.every(line => line.text.includes("implement") || line.text.trim().length > 0)
    ).toBe(true);
    const offending = lines.filter(line => line.offending);
    expect(offending.some(line => line.text.includes("max_fan_out"))).toBe(true);
  });

  it("Should render empty object/array fields as flow leaves, never [object Object]", () => {
    // Palette seeds (Channel post / Call tool / Gate / Transform) emit empty objects.
    const seeded = {
      apiVersion: "agh.loop/v1",
      graph: {
        nodes: [
          { id: "post", class: "action", kind: "agh__network_send", params: {} },
          { id: "gate", class: "control", kind: "gate", criteria: [], on_result: {} },
          { id: "shape", class: "action", kind: "transform", params: { map: {} } },
        ],
        edges: [],
      },
    };
    const text = buildDslView(seeded, new Map())
      .map(line => line.text)
      .join("\n");
    expect(text).not.toContain("[object Object]");
    expect(text).toContain("params: {}");
    expect(text).toContain("on_result: {}");
    expect(text).toContain("criteria: []");
    expect(text).toContain("map: {}");
  });

  it("Should render multiline strings as block scalars without embedding physical newlines", () => {
    const seeded = {
      apiVersion: "agh.loop/v1",
      graph: {
        nodes: [
          {
            id: "prompt",
            class: "action",
            kind: "run-agent",
            params: {
              prompt: "first line\nsecond line",
              examples: ["alpha\nbeta"],
            },
          },
        ],
        edges: [],
      },
    };
    const lines = buildDslView(seeded, new Map());
    const text = lines.map(line => line.text).join("\n");
    expect(lines.every(line => !line.text.includes("\n"))).toBe(true);
    expect(text).toContain("prompt: |\n          first line\n          second line");
    expect(text).toContain("- |\n            alpha\n            beta");
  });
});
