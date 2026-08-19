import { describe, expect, it } from "vitest";

import { definitionToGraph, type EditorEdge, type EditorNode } from "../codec";
import { loopDetailByName } from "../../mocks/fixtures";
import {
  annotationsToPositions,
  EDITOR_NODE_HEIGHT,
  EDITOR_ROUTE_ROW_HEIGHT,
  editorNodeHeight,
  layoutEditorGraph,
} from "../loop-editor-layout";

const def = loopDetailByName.get("quality-gate-demo")!.definition;

function routeNode(routeCount: number): EditorNode {
  const routes = Array.from({ length: routeCount }, (_, index) => ({
    when: `c${index}`,
    to: `t${index}`,
  }));
  return {
    id: "triage",
    type: "loopNode",
    position: { x: 0, y: 0 },
    data: {
      raw: { id: "triage", class: "control", kind: "route", routes, default: "backlog" },
      nodeClass: "control",
      kind: "route",
      hasError: false,
    },
  };
}

function plainNode(id: string): EditorNode {
  return {
    id,
    type: "loopNode",
    position: { x: 0, y: 0 },
    data: {
      raw: { id, class: "action", kind: "transform" },
      nodeClass: "action",
      kind: "transform",
      hasError: false,
    },
  };
}

function edge(source: string, target: string): EditorEdge {
  return {
    id: `${source}__${target}`,
    source,
    target,
    data: { raw: { from: source, to: target } },
  };
}

function centreGap(nodes: EditorNode[], a: string, b: string): number {
  const centre = (id: string) => {
    const node = nodes.find(entry => entry.id === id)!;
    return node.position.y + editorNodeHeight(node) / 2;
  };
  return Math.abs(centre(a) - centre(b));
}

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

  it("Should grow a route card one row per declared route plus one for the default", () => {
    expect(editorNodeHeight(plainNode("standard"))).toBe(EDITOR_NODE_HEIGHT);

    expect(editorNodeHeight(routeNode(0))).toBe(EDITOR_NODE_HEIGHT + EDITOR_ROUTE_ROW_HEIGHT);
    expect(editorNodeHeight(routeNode(3))).toBe(EDITOR_NODE_HEIGHT + 4 * EDITOR_ROUTE_ROW_HEIGHT);

    const malformed = routeNode(0);
    malformed.data.raw.routes = "not-a-list";
    expect(editorNodeHeight(malformed)).toBe(EDITOR_NODE_HEIGHT + EDITOR_ROUTE_ROW_HEIGHT);
  });

  it("Should lay a tall route card out without overlapping the sibling beside it", () => {
    const nodes = [plainNode("source"), routeNode(4), plainNode("standard")];
    const edges = [edge("source", "triage"), edge("source", "standard")];
    const laid = layoutEditorGraph(nodes, edges, []);

    const boxes = laid
      .filter(node => node.id !== "source")
      .map(node => ({ top: node.position.y, bottom: node.position.y + editorNodeHeight(node) }))
      .sort((left, right) => left.top - right.top);
    expect(boxes[0].bottom).toBeLessThanOrEqual(boxes[1].top);
  });

  it("Should push a sibling further away as the route card grows taller", () => {
    const edges = [edge("source", "triage"), edge("source", "standard")];
    const shortGap = centreGap(
      layoutEditorGraph([plainNode("source"), routeNode(0), plainNode("standard")], edges, []),
      "triage",
      "standard"
    );
    const tallGap = centreGap(
      layoutEditorGraph([plainNode("source"), routeNode(5), plainNode("standard")], edges, []),
      "triage",
      "standard"
    );

    expect(tallGap).toBeGreaterThan(shortGap);
  });
});
