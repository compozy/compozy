import { describe, expect, it } from "vitest";

import { editorEdgeId, type EditorEdge, type EditorNode } from "../codec";
import { copyEditorSelection, pasteEditorClipboard } from "../loop-editor-clipboard";

function node(id: string, position = { x: 100, y: 200 }): EditorNode {
  return {
    id,
    type: "loopNode",
    position,
    data: {
      raw: { id, class: "action", kind: "run-agent", params: {} },
      nodeClass: "action",
      kind: "run-agent",
      hasError: false,
    },
  };
}

function edge(source: string, target: string, index = 0): EditorEdge {
  return {
    id: editorEdgeId(source, target, index),
    source,
    target,
    data: { raw: { from: source, to: target } },
  };
}

describe("copyEditorSelection", () => {
  it("Should take only the edges whose endpoints are both inside the selection", () => {
    const nodes = [node("a"), node("b"), node("c")];
    const edges = [edge("a", "b", 0), edge("b", "c", 1), edge("c", "a", 2)];

    const clipboard = copyEditorSelection(nodes, edges, ["a", "b"]);
    expect(clipboard.nodes.map(entry => entry.id)).toEqual(["a", "b"]);

    expect(clipboard.edges.map(entry => entry.id)).toEqual([edges[0].id]);
  });

  it("Should detach the copied nodes from the live draft objects", () => {
    const nodes = [node("a")];
    const clipboard = copyEditorSelection(nodes, [], ["a"]);
    expect(clipboard.nodes[0]).not.toBe(nodes[0]);
    expect(clipboard.nodes[0]).toEqual(nodes[0]);
  });

  it("Should copy nothing when the selection names no node on the canvas", () => {
    const clipboard = copyEditorSelection([node("a")], [edge("a", "a")], ["ghost"]);
    expect(clipboard).toEqual({ nodes: [], edges: [] });
  });
});

describe("pasteEditorClipboard", () => {
  it("Should re-identify every pasted node without colliding with an existing id", () => {
    const nodes = [node("a"), node("b"), node("c")];
    const clipboard = copyEditorSelection(nodes, [], ["a", "b"]);

    const result = pasteEditorClipboard(nodes, [], clipboard)!;
    const pasted = result.nodes.slice(nodes.length);
    expect(pasted.map(entry => entry.id)).toEqual(["a_2", "b_2"]);

    expect(pasted.map(entry => entry.data.raw.id)).toEqual(["a_2", "b_2"]);
    expect(pasted.every(entry => entry.selected === false)).toBe(true);
  });

  it("Should rewrite internal edge endpoints onto the newly generated ids", () => {
    const nodes = [node("a"), node("b")];
    const edges = [edge("a", "b", 0)];
    const clipboard = copyEditorSelection(nodes, edges, ["a", "b"]);

    const result = pasteEditorClipboard(nodes, edges, clipboard)!;
    const pastedEdge = result.edges.at(-1)!;
    expect(pastedEdge).toMatchObject({ source: "a_2", target: "b_2" });
    expect(pastedEdge.data?.raw).toEqual({ from: "a_2", to: "b_2" });

    expect(pastedEdge.id).not.toBe(edges[0].id);
  });

  it("Should drop an edge whose endpoint never made it into the clipboard", () => {
    const nodes = [node("a"), node("b")];
    const dangling = { nodes: [node("a")], edges: [edge("a", "b", 0)] };

    const result = pasteEditorClipboard(nodes, [], dangling)!;
    expect(result.nodes.map(entry => entry.id)).toEqual(["a", "b", "a_2"]);
    expect(result.edges).toEqual([]);
  });

  it("Should offset the pasted fragment so it never lands exactly on its source", () => {
    const nodes = [node("a", { x: 100, y: 200 })];
    const clipboard = copyEditorSelection(nodes, [], ["a"]);

    const result = pasteEditorClipboard(nodes, [], clipboard)!;
    expect(result.nodes[1].position).toEqual({ x: 132, y: 232 });

    expect(result.nodes[0].position).toEqual({ x: 100, y: 200 });
  });

  it("Should append the fragment so the published order is preserved", () => {
    const nodes = [node("a"), node("b"), node("c")];
    const edges = [edge("a", "b", 0), edge("b", "c", 1)];
    const clipboard = copyEditorSelection(nodes, edges, ["b", "c"]);

    const result = pasteEditorClipboard(nodes, edges, clipboard)!;
    expect(result.nodes.map(entry => entry.id)).toEqual(["a", "b", "c", "b_2", "c_2"]);
    expect(result.edges.slice(0, edges.length)).toEqual(edges);
    expect(result.edges).toHaveLength(edges.length + 1);
  });

  it("Should focus the first pasted node so the inspector follows the paste", () => {
    const nodes = [node("a"), node("b")];
    const clipboard = copyEditorSelection(nodes, [], ["a", "b"]);
    expect(pasteEditorClipboard(nodes, [], clipboard)?.selectedNodeId).toBe("a_2");
  });

  it("Should return null for an empty clipboard instead of an empty paste", () => {
    expect(pasteEditorClipboard([node("a")], [], { nodes: [], edges: [] })).toBeNull();
  });
});
