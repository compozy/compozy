import { describe, expect, it } from "vitest";

import { definitionToGraph } from "../codec";
import { loopDetailByName } from "../../mocks/fixtures";
import { annotationsToPositions, layoutEditorGraph } from "../loop-editor-layout";

const def = loopDetailByName.get("software-delivery")!.definition;

describe("loop editor layout", () => {
  it("Should index saved annotations by node id, dropping non-finite coordinates", () => {
    const positions = annotationsToPositions([
      { node_id: "a", x: 10, y: 20 },
      { node_id: "b", x: Number.NaN, y: 5 },
    ]);
    expect(positions.get("a")).toEqual({ x: 10, y: 20 });
    expect(positions.has("b")).toBe(false);
  });

  it("Should auto-layout every node to a finite position when no annotations exist", () => {
    const { nodes, edges } = definitionToGraph(def);
    const laid = layoutEditorGraph(nodes, edges, []);
    expect(laid).toHaveLength(nodes.length);
    for (const node of laid) {
      expect(Number.isFinite(node.position.x)).toBe(true);
      expect(Number.isFinite(node.position.y)).toBe(true);
    }
    // A left-to-right dagre pass separates the linear spine's first two nodes.
    const slug = laid.find(node => node.id === "slug")!;
    const load = laid.find(node => node.id === "load_tasks")!;
    expect(load.position.x).toBeGreaterThan(slug.position.x);
  });

  it("Should let a saved annotation override the computed dagre position", () => {
    const { nodes, edges } = definitionToGraph(def);
    const laid = layoutEditorGraph(nodes, edges, [{ node_id: "review", x: 999, y: 111 }]);
    expect(laid.find(node => node.id === "review")?.position).toEqual({ x: 999, y: 111 });
  });
});
