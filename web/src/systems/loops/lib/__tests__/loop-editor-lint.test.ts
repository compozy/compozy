import { describe, expect, it } from "vitest";

import { definitionToGraph } from "../codec";
import { loopDetailByName } from "../../mocks/fixtures";
import {
  applyLintToNodes,
  buildLintState,
  classifyInvariant,
  emptyLintState,
} from "../loop-editor-lint";
import type { ValidateLoopResult } from "../../types";

const def = loopDetailByName.get("software-delivery")!.definition;

describe("loop editor lint", () => {
  it("Should treat a passing verdict as clean with every invariant chip passing", () => {
    const state = buildLintState({ valid: true, errors: [] });
    expect(state.hasBlockingErrors).toBe(false);
    expect(state.errorCount).toBe(0);
    expect(Object.values(state.invariants).every(status => status === "pass")).toBe(true);
    expect(emptyLintState().hasBlockingErrors).toBe(false);
  });

  it("Should mark the pre-validation state neutral and a real verdict validated", () => {
    // The pre-validation state must not claim a pass the daemon has not confirmed.
    expect(emptyLintState().validated).toBe(false);
    expect(buildLintState(null).validated).toBe(false);
    expect(buildLintState({ valid: true, errors: [] }).validated).toBe(true);
    expect(
      buildLintState({
        valid: false,
        errors: [{ code: "cycle_detected", message: "", severity: "error" }],
      }).validated
    ).toBe(true);
  });

  it("Should map 422 per-node errors onto the correct node ids (web-unit-7)", () => {
    const result: ValidateLoopResult = {
      valid: false,
      errors: [
        {
          node_id: "implement",
          code: "fan_out_ceiling_exceeded",
          message: "max_fan_out (80) exceeds the daemon ceiling of 64.",
          severity: "error",
        },
        {
          node_id: "execute_task",
          code: "unknown_reference",
          message: "nodes.load_tasks.output.missing is not declared.",
          severity: "error",
        },
      ],
    };
    const state = buildLintState(result);
    expect(state.byNode.get("implement")).toHaveLength(1);
    expect(state.byNode.get("execute_task")?.[0].code).toBe("unknown_reference");
    expect(state.byNode.has("review")).toBe(false);
    expect(state.errorCount).toBe(2);
    expect(state.hasBlockingErrors).toBe(true);
  });

  it("Should flip only the invariant chips a blocking error attributes to", () => {
    const state = buildLintState({
      valid: false,
      errors: [
        { node_id: "implement", code: "fan_out_ceiling_exceeded", message: "", severity: "error" },
        { node_id: "a", code: "cycle_detected", message: "", severity: "error" },
      ],
    });
    expect(state.invariants.fan_out).toBe("fail");
    expect(state.invariants.acyclicity).toBe("fail");
    expect(state.invariants.reachability).toBe("pass");
    expect(state.invariants.termination).toBe("pass");
  });

  it("Should collect whole-graph issues that carry no node_id as general issues", () => {
    const state = buildLintState({
      valid: false,
      errors: [{ code: "no_terminal_state", message: "no terminal reachable", severity: "error" }],
    });
    expect(state.general).toHaveLength(1);
    expect(state.byNode.size).toBe(0);
    expect(state.invariants.termination).toBe("fail");
  });

  it("Should not let a warning block Publish", () => {
    const state = buildLintState({
      valid: true,
      errors: [
        { node_id: "x", code: "tool_missing_output_schema", message: "", severity: "warning" },
      ],
    });
    expect(state.hasBlockingErrors).toBe(false);
    expect(state.errorCount).toBe(0);
    expect(state.byNode.get("x")).toHaveLength(1);
  });

  it("Should stamp data.hasError onto the nodes named by the per-node map", () => {
    const { nodes } = definitionToGraph(def);
    const state = buildLintState({
      valid: false,
      errors: [
        { node_id: "implement", code: "fan_out_ceiling_exceeded", message: "", severity: "error" },
      ],
    });
    const marked = applyLintToNodes(nodes, state.byNode);
    expect(marked.find(node => node.id === "implement")?.data.hasError).toBe(true);
    expect(marked.find(node => node.id === "review")?.data.hasError).toBe(false);
  });

  it("Should classify codes to invariants by family and return null when unattributable", () => {
    expect(classifyInvariant("fan_out_ceiling_exceeded")).toBe("fan_out");
    expect(classifyInvariant("graph_has_cycle")).toBe("acyclicity");
    expect(classifyInvariant("node_unreachable")).toBe("reachability");
    expect(classifyInvariant("unknown_reference")).toBe("references");
    // `item_outside_fanout` is a reference-scope error, not a fan-out-bounds failure —
    // its `fanout` substring must not misattribute it to the Fan-out chip.
    expect(classifyInvariant("item_outside_fanout")).toBe("references");
    expect(classifyInvariant("some_new_daemon_code")).toBeNull();
  });
});
