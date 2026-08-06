import { describe, expect, it } from "vitest";

import { definitionToGraph } from "../codec";
import { loopDetailByName } from "../../mocks/fixtures";
import {
  getAtPath,
  isNodeIdPath,
  renameNodeId,
  setAtPath,
  setNodeField,
  setNodeFields,
  unsetAtPath,
} from "../loop-editor-draft";

const def = loopDetailByName.get("quality-gate-demo")!.definition;

describe("loop editor draft", () => {
  it("Should read a nested value at a raw path", () => {
    const source = { params: { agent: "impl", nested: { deep: 3 } } };
    expect(getAtPath(source, ["params", "agent"])).toBe("impl");
    expect(getAtPath(source, ["params", "nested", "deep"])).toBe(3);
    expect(getAtPath(source, ["params", "missing"])).toBeUndefined();
  });

  it("Should immutably set a value, creating intermediate containers", () => {
    const source = { a: 1 };
    const next = setAtPath(source, ["b", "c"], 5);
    expect(next).toEqual({ a: 1, b: { c: 5 } });
    expect(source).toEqual({ a: 1 });
  });

  it("Should delete the leaf key when setting undefined instead of pinning null", () => {
    const source = { params: { model: "opus", agent: "impl" } };
    const next = setAtPath(source, ["params", "model"], undefined);
    expect(next).toEqual({ params: { agent: "impl" } });
  });

  it("Should not synthesize an empty parent when clearing a value under a missing parent", () => {
    // Clearing `retry.max` on a node with no `retry` must not add `retry: {}`.
    const source = { id: "x", kind: "run-agent" };
    const next = setAtPath(source, ["retry", "max"], undefined);
    expect(next).toEqual({ id: "x", kind: "run-agent" });
    expect(next).not.toHaveProperty("retry");
  });

  it("Should apply a single field edit to one node, leaving siblings untouched", () => {
    const { nodes } = definitionToGraph(def);
    const edited = setNodeField(nodes, "implement", ["max_fan_out"], 32);
    expect(edited.find(node => node.id === "implement")?.data.raw.max_fan_out).toBe(32);
    // The original node array is not mutated.
    expect(nodes.find(node => node.id === "implement")?.data.raw.max_fan_out).toBe(64);
    // A sibling node is the same reference (only the edited node is replaced).
    const slugBefore = nodes.find(node => node.id === "slug");
    const slugAfter = edited.find(node => node.id === "slug");
    expect(slugAfter).toBe(slugBefore);
  });

  it("WT-005: Should drop a container the clear emptied, but keep one with siblings", () => {
    // A leftover `result_contract: {}` is a guaranteed `result_contract_invalid` and an
    // `expires: {}` a `wait_shape_invalid`, so clearing the last key must prune its parent.
    const emptied = unsetAtPath({ id: "x", result_contract: { failure_field: "err" } }, [
      "result_contract",
      "failure_field",
    ]);
    expect(emptied).toEqual({ id: "x" });
    expect(emptied).not.toHaveProperty("result_contract");

    const kept = unsetAtPath(
      { id: "x", result_contract: { failure_field: "err", message_field: "err.message" } },
      ["result_contract", "failure_field"]
    );
    expect(kept).toEqual({ id: "x", result_contract: { message_field: "err.message" } });
  });

  it("WT-005: Should prune every ancestor a nested clear emptied, along that path only", () => {
    const next = unsetAtPath({ id: "x", timeout: "30m", retry: { backoff: { base: "30s" } } }, [
      "retry",
      "backoff",
      "base",
    ]);
    expect(next).toEqual({ id: "x", timeout: "30m" });

    const partial = unsetAtPath({ id: "x", retry: { max_attempts: 3, backoff: { base: "30s" } } }, [
      "retry",
      "backoff",
      "base",
    ]);
    expect(partial).toEqual({ id: "x", retry: { max_attempts: 3 } });
  });

  it("WT-005: Should clear a whole effect list path so no zero-count list is published", () => {
    const next = unsetAtPath({ id: "x", on_retry: [{ emit: { kind: "task_retrying" } }] }, [
      "on_retry",
    ]);
    expect(next).toEqual({ id: "x" });
    // Clearing an absent path is a no-op that returns the same reference.
    const source = { id: "x" };
    expect(unsetAtPath(source, ["on_retry"])).toBe(source);
  });

  it("WT-005: Should apply multi-key edits as one transition with no invalid intermediate", () => {
    const { nodes } = definitionToGraph(def);
    const seeded = setNodeField(nodes, "implement", ["params"], { for: "1h" });
    const edited = setNodeFields(seeded, "implement", [
      { path: ["params", "for"], value: undefined },
      { path: ["params", "until"], value: "2026-09-01T00:00:00Z" },
      { path: ["params", "event"], value: undefined },
    ]);
    // Exactly one discriminator survives, and the losers left no empty containers behind.
    expect(edited.find(node => node.id === "implement")?.data.raw.params).toEqual({
      until: "2026-09-01T00:00:00Z",
    });
    // Only the edited node is replaced; siblings keep their identity.
    const slugBefore = seeded.find(node => node.id === "slug");
    expect(edited.find(node => node.id === "slug")).toBe(slugBefore);
  });

  it("Should recognize the node-id path so a rename can cascade", () => {
    expect(isNodeIdPath(["id"])).toBe(true);
    expect(isNodeIdPath(["params", "agent"])).toBe(false);
  });

  it("Should rename a node id across the node, edges, and raw from/to references", () => {
    const { nodes, edges } = definitionToGraph(def);
    const renamed = renameNodeId(nodes, edges, "implement", "fan_tasks");
    expect(renamed.nodes.find(node => node.id === "fan_tasks")?.data.raw.id).toBe("fan_tasks");
    expect(renamed.nodes.some(node => node.id === "implement")).toBe(false);
    const inbound = renamed.edges.find(edge => edge.target === "fan_tasks");
    expect(inbound?.data?.raw.to).toBe("fan_tasks");
    const outbound = renamed.edges.find(edge => edge.source === "fan_tasks");
    expect(outbound?.data?.raw.from).toBe("fan_tasks");
  });
});
