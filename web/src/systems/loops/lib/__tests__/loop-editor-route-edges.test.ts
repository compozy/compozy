import { describe, expect, it, vi } from "vitest";

import { editorEdgeId, type EditorEdge, type EditorNode } from "../codec";
import {
  buildDisplayEdges,
  forwardNodeIds,
  LOOP_EDITOR_EDGE_TYPE,
  reconcileRouteEdges,
  removeRouteTargets,
  ROUTE_DEFAULT_HANDLE_ID,
  routeCardRows,
  routeHandleId,
  updateRouteTarget,
} from "../loop-editor-route-edges";

function actionNode(id: string): EditorNode {
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

function routeNode(id: string, routes: { when: string; to: string }[], fallback = ""): EditorNode {
  return {
    id,
    type: "loopNode",
    position: { x: 0, y: 0 },
    data: {
      raw: { id, class: "control", kind: "route", routes, default: fallback },
      nodeClass: "control",
      kind: "route",
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

const TRIAGE_ROUTES = [
  { when: 'inputs.severity == "p0"', to: "hotfix" },
  { when: 'inputs.severity == "p1"', to: "standard" },
];

function displayOptions() {
  return { readOnly: false, onDelete: vi.fn() };
}

describe("buildDisplayEdges", () => {
  it("Should give an edge to a declared route target that route's handle and condition", () => {
    const nodes = [routeNode("triage", TRIAGE_ROUTES, "backlog"), actionNode("standard")];
    const [display] = buildDisplayEdges([edge("triage", "standard", 1)], nodes, displayOptions());
    expect(display.sourceHandle).toBe(routeHandleId(1));
    expect(display.data?.routeLabel).toBe('inputs.severity == "p1"');
  });

  it("Should give an edge to the default target the default handle and the literal word", () => {
    const nodes = [routeNode("triage", TRIAGE_ROUTES, "backlog"), actionNode("backlog")];
    const [display] = buildDisplayEdges([edge("triage", "backlog")], nodes, displayOptions());
    expect(display.sourceHandle).toBe(ROUTE_DEFAULT_HANDLE_ID);
    expect(display.data?.routeLabel).toBe("default");
  });

  it("Should keep an unexplained edge plain instead of inventing a handle for it", () => {
    const nodes = [routeNode("triage", TRIAGE_ROUTES, "backlog"), actionNode("orphan")];
    const [display] = buildDisplayEdges([edge("triage", "orphan")], nodes, displayOptions());
    expect(display.sourceHandle).toBeUndefined();
    expect(display.data?.routeLabel).toBeUndefined();

    const seeded = [routeNode("triage", [], ""), actionNode("anything")];
    const [seededDisplay] = buildDisplayEdges(
      [edge("triage", "anything")],
      seeded,
      displayOptions()
    );
    expect(seededDisplay.sourceHandle).toBeUndefined();
  });

  it("Should leave an edge from a non-route node untouched by route display", () => {
    const nodes = [actionNode("standard"), actionNode("rollout")];
    const [display] = buildDisplayEdges([edge("standard", "rollout")], nodes, displayOptions());
    expect(display.sourceHandle).toBeUndefined();
    expect(display.data?.routeLabel).toBeUndefined();
  });

  it("Should give every edge the custom type plus the delete affordance context", () => {
    const onDelete = vi.fn();
    const nodes = [routeNode("triage", TRIAGE_ROUTES, "backlog"), actionNode("hotfix")];
    const edges = [edge("triage", "hotfix"), edge("hotfix", "triage", 1)];

    const display = buildDisplayEdges(edges, nodes, { readOnly: true, onDelete });
    expect(display.every(entry => entry.type === LOOP_EDITOR_EDGE_TYPE)).toBe(true);
    expect(display.every(entry => entry.data?.readOnly === true)).toBe(true);
    expect(display.every(entry => entry.data?.onDelete === onDelete)).toBe(true);
  });

  it("Should synthesize the raw endpoints for an edge the operator just drew", () => {
    const drawn = { id: "drawn", source: "a", target: "b" } as EditorEdge;
    const [display] = buildDisplayEdges([drawn], [actionNode("a")], displayOptions());
    expect(display.data?.raw).toEqual({ from: "a", to: "b" });
  });
});

describe("routeCardRows", () => {
  it("Should pair every condition with its handle and target, defaulting last", () => {
    expect(routeCardRows(routeNode("triage", TRIAGE_ROUTES, "backlog"))).toEqual([
      { handle: "route:0", label: 'inputs.severity == "p0"', to: "hotfix" },
      { handle: "route:1", label: 'inputs.severity == "p1"', to: "standard" },
      { handle: ROUTE_DEFAULT_HANDLE_ID, label: "default", to: "backlog" },
    ]);
  });

  it("Should say a route has no condition rather than rendering an empty row", () => {
    const rows = routeCardRows(routeNode("triage", [{ when: "", to: "hotfix" }], ""));
    expect(rows[0].label).toBe("(no condition)");
    expect(rows[1]).toEqual({ handle: ROUTE_DEFAULT_HANDLE_ID, label: "default", to: "" });
  });

  it("Should give a node that is not a route no rows", () => {
    expect(routeCardRows(actionNode("standard"))).toEqual([]);
  });
});

describe("route graph coherence", () => {
  it("Should rebuild outgoing edges from the declared route targets", () => {
    const nodes = [
      routeNode("triage", TRIAGE_ROUTES, "backlog"),
      actionNode("hotfix"),
      actionNode("standard"),
      actionNode("backlog"),
      actionNode("stale"),
    ];
    const result = reconcileRouteEdges(nodes, [edge("triage", "stale")], "triage");
    expect(result.map(entry => entry.target)).toEqual(["hotfix", "standard", "backlog"]);
  });

  it("Should update the declaration addressed by a route handle", () => {
    const nodes = [routeNode("triage", TRIAGE_ROUTES, "backlog")];
    const updated = updateRouteTarget(nodes, "triage", routeHandleId(1), "canary");
    expect(routeCardRows(updated[0])[1]?.to).toBe("canary");
    const fallback = updateRouteTarget(updated, "triage", ROUTE_DEFAULT_HANDLE_ID, "manual");
    expect(routeCardRows(fallback[0]).at(-1)?.to).toBe("manual");
  });

  it("Should remove deleted targets from conditions and fallback", () => {
    const nodes = [routeNode("triage", TRIAGE_ROUTES, "backlog")];
    const updated = removeRouteTargets(nodes, new Set(["standard", "backlog"]));
    expect(routeCardRows(updated[0])).toEqual([
      { handle: "route:0", label: 'inputs.severity == "p0"', to: "hotfix" },
      { handle: ROUTE_DEFAULT_HANDLE_ID, label: "default", to: "" },
    ]);
  });
});

describe("forwardNodeIds", () => {
  it("Should offer only nodes downstream of the picker's node", () => {
    const nodes = ["upstream", "here", "next", "last"].map(actionNode);
    const edges = [edge("upstream", "here", 0), edge("here", "next", 1), edge("next", "last", 2)];
    const forward = forwardNodeIds(nodes, edges, "here");
    expect(forward).toEqual(["next", "last"]);
    expect(forward).not.toContain("here");
    expect(forward).not.toContain("upstream");
  });

  it("Should include an unconnected node the author has not wired yet", () => {
    const nodes = ["here", "next", "dropped"].map(actionNode);
    const edges = [edge("here", "next", 0)];
    expect(forwardNodeIds(nodes, edges, "here")).toEqual(["next", "dropped"]);
  });

  it("Should degrade a node caught in a cycle to an empty picker instead of guessing", () => {
    const nodes = ["a", "b"].map(actionNode);
    const edges = [edge("a", "b", 0), edge("b", "a", 1)];

    expect(forwardNodeIds(nodes, edges, "a")).toEqual(["b"]);
    expect(forwardNodeIds(nodes, edges, "a")).not.toContain("a");
  });
});
