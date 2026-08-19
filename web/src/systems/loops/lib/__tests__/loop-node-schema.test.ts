import { describe, expect, it } from "vitest";

import type { EditorNode, RawLoopNode } from "../codec";
import { setNodeFields } from "../loop-editor-draft";
import { environmentHint, environmentModeEdits } from "../loop-node-environment-fields";
import { readReview, reviewEdits } from "../loop-node-review-fields";
import { readRoutes, routeDefaultEdit, routesEdit } from "../loop-node-route-fields";
import { buildNodeFields, type FieldSpec } from "../loop-node-schema";
import type { LoopWaitMode } from "../loop-node-schema-types";
import { readStrategy, strategyEdits } from "../loop-node-strategy-fields";
import { waitMode, waitModeEdits } from "../loop-node-wait-fields";

function keys(fields: FieldSpec[]): string[] {
  return fields.map(field => ("key" in field ? field.key : "hint"));
}

/** Flattens folds so a lifecycle field can be asserted by key wherever it is nested. */
function flatten(fields: FieldSpec[]): FieldSpec[] {
  return fields.flatMap(field =>
    field.type === "fold" ? [field, ...flatten(field.fields)] : [field]
  );
}

function fieldByKey(fields: FieldSpec[], key: string): FieldSpec | undefined {
  return flatten(fields).find(field => "key" in field && field.key === key);
}

function hasKey(fields: FieldSpec[], key: string): boolean {
  return fieldByKey(fields, key) !== undefined;
}

function editorNode(raw: RawLoopNode): EditorNode {
  return {
    id: raw.id,
    type: "loopNode",
    position: { x: 0, y: 0 },
    data: { raw, nodeClass: null, kind: raw.kind, hasError: false },
  };
}

/** Applies a wait-mode switch exactly the way the inspector does, and returns the new params. */
function switchWaitMode(raw: RawLoopNode, mode: LoopWaitMode): RawLoopNode {
  const next = setNodeFields([editorNode(raw)], raw.id, waitModeEdits(raw, mode));
  return next[0].data.raw;
}

describe("loop node schema", () => {
  it("Should render run-agent with required agent + prompt and the overrides fold", () => {
    const raw: RawLoopNode = { id: "execute", class: "action", kind: "run-agent", params: {} };
    const fields = buildNodeFields(raw);
    const agent = fields.find(f => "key" in f && f.key === "agent");
    const prompt = fields.find(f => "key" in f && f.key === "prompt");
    expect(agent).toMatchObject({ required: true, reference: true });
    expect(prompt).toMatchObject({ type: "textarea", required: true, reference: true });
    expect(keys(fields)).not.toContain("action_ref");
    expect(keys(fields)).toContain("overrides");
  });

  it("Should render fan-out max_fan_out as an uncapped positive author bound", () => {
    const raw: RawLoopNode = { id: "implement", class: "control", kind: "fan-out" };
    const fanOut = buildNodeFields(raw).find(f => "key" in f && f.key === "max_fan_out");
    expect(fanOut).toMatchObject({ type: "number" });
    expect(fanOut).not.toHaveProperty("ceiling");
    const fields = buildNodeFields(raw);
    const strategyFields = flatten(fields).filter(field => field.key === "strategy");
    expect(strategyFields).toHaveLength(1);
    expect(strategyFields[0]).toMatchObject({ type: "strategy" });
    expect(fieldByKey(fields, "bind_as")).toMatchObject({ path: ["bind_as"] });
    expect(fieldByKey(fields, "index_as")).toMatchObject({ path: ["index_as"] });
  });

  it("Should render a gate with a criteria editor, verdict policy, and editable routing", () => {
    const raw: RawLoopNode = { id: "review", class: "control", kind: "gate" };
    const fields = buildNodeFields(raw);
    expect(fields.some(f => "type" in f && f.type === "criteria")).toBe(true);
    const policy = fields.find(f => "key" in f && f.key === "verdict_policy");
    expect(policy).toMatchObject({
      type: "select",
      options: ["revise_until_clean", "fixed_passes"],
    });
    const onPass = fields.find(f => "key" in f && f.key === "on_pass");
    const onFail = fields.find(f => "key" in f && f.key === "on_fail");
    expect(onPass).toMatchObject({
      type: "text",
      path: ["on_result", "pass"],
      json: true,
      hint: expect.stringContaining('{"route":"node_id"}'),
    });
    expect(onFail).toMatchObject({
      type: "text",
      path: ["on_result", "fail"],
      json: true,
      hint: expect.stringContaining("removed branch action"),
    });
  });

  it("Should mark branch.condition as a CEL reference field and give run-loop editable inputs", () => {
    const branch = buildNodeFields({ id: "b", class: "control", kind: "branch" }).find(
      f => "key" in f && f.key === "condition"
    );
    expect(branch).toMatchObject({ reference: true, cel: true });
    const runLoop = buildNodeFields({ id: "child", class: "action", kind: "run-loop" });
    expect(runLoop.some(f => "key" in f && f.key === "inputs" && "json" in f && f.json)).toBe(true);
  });

  /**
   * `params.cwd` is retired end to end (B-009): the daemon rejects it at runtime
   * validation, so the inspector must offer exactly one environment control and
   * no directory field beside it (Business Rule 22).
   */
  it("Should offer one Environment descriptor and no retired working-dir field on agent nodes", () => {
    for (const kind of ["run-agent", "goal"]) {
      const fields = flatten(buildNodeFields({ id: "x", class: "action", kind }));
      expect(hasKey(fields, "cwd")).toBe(false);
      expect(fieldByKey(fields, "environment")).toMatchObject({
        type: "environment",
        basePath: ["params", "environment"],
        mode: null,
        inheritLabel: "Inherit",
      });
    }
  });

  it("Should carry the selected environment mode on the descriptor without companion FieldSpecs", () => {
    const worktreeNode = flatten(
      buildNodeFields({
        id: "x",
        class: "action",
        kind: "run-agent",
        params: { environment: { mode: "worktree", worktree_ref: "payments-retry" } },
      })
    );
    expect(fieldByKey(worktreeNode, "environment")).toMatchObject({
      type: "environment",
      mode: "worktree",
    });
    expect(hasKey(worktreeNode, "environment_worktree_ref")).toBe(false);
    expect(hasKey(worktreeNode, "environment_directory")).toBe(false);

    const directoryNode = flatten(
      buildNodeFields({
        id: "x",
        class: "action",
        kind: "run-agent",
        params: { environment: { mode: "directory", directory: "packages/api" } },
      })
    );
    expect(fieldByKey(directoryNode, "environment")).toMatchObject({ mode: "directory" });
    expect(hasKey(directoryNode, "environment_directory")).toBe(false);
    expect(hasKey(directoryNode, "environment_worktree_ref")).toBe(false);

    const perRunNode = flatten(
      buildNodeFields({
        id: "x",
        class: "action",
        kind: "run-agent",
        params: { environment: { mode: "per_run" } },
      })
    );
    expect(fieldByKey(perRunNode, "environment")).toMatchObject({ mode: "per_run" });
    expect(hasKey(perRunNode, "environment_worktree_ref")).toBe(false);
    expect(hasKey(perRunNode, "environment_directory")).toBe(false);
  });

  it("Should name each environment mode's inspector hint without inventing a loop default", () => {
    expect(environmentHint("root")).toBe(
      "Runs at the workspace root. Part of the session binding key."
    );
    expect(environmentHint("worktree")).toBeUndefined();
    expect(environmentHint("worktree", { mode: "root" })).toBe(
      "Overrides the loop default (Workspace root) for this node."
    );
    expect(environmentHint("per_run")).toBe(
      "Name and branch are generated at run start — run/<task-slug>."
    );
    expect(environmentHint("directory")).toBe(
      "Resolved when the run starts. Type {{ to autocomplete references."
    );
    expect(environmentHint(null)).toBeUndefined();
  });

  it("Should drop retired cwd when an environment mode is chosen", () => {
    const raw: RawLoopNode = {
      id: "x",
      class: "action",
      kind: "run-agent",
      params: { cwd: "packages/api" },
    };
    const next = setNodeFields([editorNode(raw)], raw.id, environmentModeEdits(raw, "root"));
    const params = next[0].data.raw.params as Record<string, unknown>;
    expect(params.cwd).toBeUndefined();
    expect(params.environment).toEqual({ mode: "root" });
  });

  it("Should keep the Environment descriptor off nodes that cannot start agents", () => {
    for (const node of [
      { id: "b", class: "control", kind: "branch" },
      { id: "c", class: "action", kind: "run-loop" },
    ]) {
      expect(hasKey(flatten(buildNodeFields(node)), "environment")).toBe(false);
    }
  });

  it("Should render any non-reserved action kind as a generic ToolID form with editable kind", () => {
    const raw: RawLoopNode = {
      id: "post",
      class: "action",
      kind: "compozy__network_send",
      params: {},
    };
    const fields = buildNodeFields(raw);
    const kind = fields.find(f => "key" in f && f.key === "kind");
    expect(kind).toMatchObject({ type: "text", path: ["kind"], required: true });
    expect(fields.some(f => "key" in f && f.key === "params" && "json" in f && f.json)).toBe(true);
  });

  it("Should mark structured fields as JSON so an edit never coerces them to strings", () => {
    const raw: RawLoopNode = { id: "execute", class: "action", kind: "run-agent", params: {} };
    const schema = buildNodeFields(raw).find(f => "key" in f && f.key === "output_schema");
    expect(schema).toMatchObject({ json: true });
  });

  it("Should render a file-import parse select over exactly json and text", () => {
    const raw: RawLoopNode = { id: "load", class: "source", kind: "file-import", pattern: "" };
    const parse = buildNodeFields(raw).find(f => "key" in f && f.key === "parse");
    expect(parse).toMatchObject({ type: "select", path: ["parse"], options: ["json", "text"] });
  });

  it("Should render a watch-events source with a subscription list editor", () => {
    const raw: RawLoopNode = { id: "on_events", class: "source", kind: "watch-events", events: [] };
    const fields = buildNodeFields(raw);
    const events = fields.find(f => "key" in f && f.key === "events");
    // The subscription list is a bespoke editor (kind select fed by the supported matrix
    // + CEL filter per entry), not a raw JSON textarea.
    expect(events).toMatchObject({ type: "events", path: ["events"] });
    expect(keys(fields)).toContain("produces");
  });

  it("Should render Goal as a first-class action with its closed operational contract", () => {
    const raw: RawLoopNode = {
      id: "goal",
      class: "action",
      kind: "goal",
      params: {},
    };
    const fields = buildNodeFields(raw);
    expect(fields.find(field => "key" in field && field.key === "judge")).toMatchObject({
      type: "criteria",
      path: ["params", "judge"],
      allowedTypes: ["command", "agent-judge", "extension"],
    });
    expect(fields.find(field => "key" in field && field.key === "max_turns")).toMatchObject({
      type: "number",
      path: ["params", "max_turns"],
    });
    expect(fields.find(field => "key" in field && field.key === "on_exhausted")).toMatchObject({
      type: "select",
      path: ["params", "on_exhausted"],
      options: ["halt", "escalate"],
    });
    expect(fields.find(field => "key" in field && field.key === "session_mode")).toMatchObject({
      path: ["session", "mode"],
      options: ["continuous"],
    });
    expect(fields.find(field => "key" in field && field.key === "retry")).toMatchObject({
      type: "fold",
      fields: expect.arrayContaining([
        expect.objectContaining({ key: "max_attempts", path: ["retry", "max_attempts"] }),
      ]),
    });
  });

  it("Should render an input node's input_ref as an editable select over the declared inputs", () => {
    const raw: RawLoopNode = { id: "slug", class: "source", kind: "input", input_ref: "" };
    const fields = buildNodeFields(raw, {
      inputs: { slug: { type: "string" }, max: { type: "number" } },
    });
    const ref = fields.find(f => "key" in f && f.key === "input_ref");
    expect(ref).toMatchObject({ type: "select", path: ["input_ref"], options: ["slug", "max"] });
    // With no declared inputs it falls back to an editable text field, never a read-only static.
    const noInputs = buildNodeFields(raw, { inputs: {} }).find(
      f => "key" in f && f.key === "input_ref"
    );
    expect(noInputs).toMatchObject({ type: "text", path: ["input_ref"] });
  });

  it("Should degrade an unknown class/kind to id + kind static", () => {
    const raw: RawLoopNode = { id: "mystery", class: "unknown", kind: "weird" };
    const fields = buildNodeFields(raw);
    expect(keys(fields)).toEqual(["id", "kind"]);
  });

  it("WT-005: Should author every Spec 1 reliability key at its exact DSL path on an action", () => {
    const fields = buildNodeFields({ id: "execute", class: "action", kind: "run-agent" });
    expect(fieldByKey(fields, "deadline")).toMatchObject({ path: ["deadline"] });
    expect(fieldByKey(fields, "max_attempts")).toMatchObject({
      path: ["retry", "max_attempts"],
    });
    expect(fieldByKey(fields, "backoff_base")).toMatchObject({
      path: ["retry", "backoff", "base"],
    });
    expect(fieldByKey(fields, "backoff_max")).toMatchObject({ path: ["retry", "backoff", "max"] });
    expect(fieldByKey(fields, "non_retryable")).toMatchObject({
      path: ["retry", "non_retryable"],
      json: true,
    });
    expect(fieldByKey(fields, "result_failure_field")).toMatchObject({
      path: ["result_contract", "failure_field"],
    });
    expect(fieldByKey(fields, "on_error_route")).toMatchObject({ path: ["on_error", "route"] });
    expect(fieldByKey(fields, "on_error_allow_fail")).toMatchObject({
      type: "switch",
      path: ["on_error", "allow_fail"],
    });
    expect(fieldByKey(fields, "on_error_effects")).toMatchObject({
      type: "effects",
      path: ["on_error", "effects"],
    });
    // `retry.max` is retired (`retry_max_unsupported`) — the editor must never author it.
    expect(JSON.stringify(fields)).not.toContain('"retry","max"');
  });

  it("WT-005: Should author the six node triggers as effect lists at their envelope keys", () => {
    const fields = buildNodeFields({ id: "execute", class: "action", kind: "run-agent" });
    for (const trigger of [
      "on_retry",
      "on_success",
      "on_pause",
      "on_timeout",
      "on_cancel",
      "on_quarantine",
    ]) {
      expect(fieldByKey(fields, trigger)).toMatchObject({ type: "effects", path: [trigger] });
    }
  });

  it("WT-005: Should scope lifecycle keys to the classes whose runtime executes them", () => {
    // A control node reacts and routes, but produces no task run: no retry, no deadline, and
    // no result_contract (its linter branch has no declared output schema to resolve against).
    const gate = buildNodeFields({ id: "review", class: "control", kind: "gate" });
    expect(hasKey(gate, "on_error_route")).toBe(true);
    expect(hasKey(gate, "on_pause")).toBe(true);
    expect(hasKey(gate, "deadline")).toBe(false);
    expect(hasKey(gate, "max_attempts")).toBe(false);
    expect(hasKey(gate, "result_failure_field")).toBe(false);
    // A gate owns the approval expiry ladder.
    expect(fieldByKey(gate, "gate_expires_after")).toMatchObject({ path: ["expires", "after"] });

    // A goal owns its own attempt budget; deadline/backoff/non_retryable are rejected there.
    const goal = buildNodeFields({ id: "goal", class: "action", kind: "goal", params: {} });
    expect(hasKey(goal, "on_error_route")).toBe(true);
    expect(hasKey(goal, "result_failure_field")).toBe(true);
    expect(hasKey(goal, "deadline")).toBe(false);
    expect(hasKey(goal, "backoff_base")).toBe(false);
    expect(hasKey(goal, "non_retryable")).toBe(false);

    // Source kinds produce no attempt at all — the envelope stays off entirely.
    const input = buildNodeFields({ id: "slug", class: "source", kind: "input" }, { inputs: {} });
    expect(hasKey(input, "on_error_route")).toBe(false);
    expect(hasKey(input, "on_retry")).toBe(false);
  });

  it("WT-005: Should expose on_parent_close only on run-loop, with the closed enum", () => {
    const runLoop = buildNodeFields({ id: "child", class: "action", kind: "run-loop" });
    expect(fieldByKey(runLoop, "on_parent_close")).toMatchObject({
      type: "select",
      path: ["on_parent_close"],
      options: ["terminate", "cancel", "abandon"],
    });
    const runAgent = buildNodeFields({ id: "x", class: "action", kind: "run-agent" });
    expect(hasKey(runAgent, "on_parent_close")).toBe(false);
  });

  it("WT-005: Should author the wait inspector at its params paths", () => {
    const raw: RawLoopNode = {
      id: "await_ack",
      class: "control",
      kind: "wait",
      params: { for: "24h" },
    };
    const fields = buildNodeFields(raw);
    expect(fieldByKey(fields, "wait_mode")).toMatchObject({ type: "wait-mode", mode: "for" });
    expect(fieldByKey(fields, "wait_for")).toMatchObject({ path: ["params", "for"] });
    expect(fieldByKey(fields, "wait_expect")).toMatchObject({
      path: ["params", "expect"],
      json: true,
    });
    expect(fieldByKey(fields, "wait_ahead_arrival")).toMatchObject({
      path: ["params", "ahead_arrival"],
      options: ["consume_on_entry", "reject"],
    });
    expect(fieldByKey(fields, "wait_expires_after")).toMatchObject({
      path: ["params", "expires", "after"],
    });
    expect(fieldByKey(fields, "wait_expires_escalate")).toMatchObject({
      type: "effects",
      path: ["params", "expires", "escalate"],
    });
  });

  it("WT-005: Should leave exactly one wait discriminator through for → until → event → for", () => {
    // `wait_shape_invalid` fires on a second discriminator OR any stray params key, so every
    // switch must land as ONE transition that clears both losers.
    let raw: RawLoopNode = {
      id: "await_ack",
      class: "control",
      kind: "wait",
      params: { for: "24h" },
    };
    for (const mode of ["until", "event", "for"] as const) {
      raw = switchWaitMode(raw, mode);
      const params = raw.params as Record<string, unknown>;
      const declared = (["for", "until", "event"] as const).filter(
        key => params[key] !== undefined
      );
      expect(declared).toEqual([mode]);
      expect(waitMode(raw)).toBe(mode);
    }
    // The event mode seeds a valid subscription rather than an empty object.
    const eventRaw = switchWaitMode(raw, "event");
    expect((eventRaw.params as Record<string, unknown>).event).toMatchObject({
      kind: expect.any(String),
    });
  });
});

function fieldByType(fields: FieldSpec[], type: FieldSpec["type"]): FieldSpec | undefined {
  return flatten(fields).find(field => field.type === type);
}

function applyEdits(raw: RawLoopNode, edits: Parameters<typeof setNodeFields>[2]): RawLoopNode {
  return setNodeFields([editorNode(raw)], raw.id, edits)[0].data.raw;
}

describe("graph completion node schema", () => {
  it("Should render a route node with forward-only targets and a fail-closed eval policy", () => {
    const raw: RawLoopNode = {
      id: "triage",
      class: "control",
      kind: "route",
      routes: [{ when: 'inputs.severity == "p0"', to: "hotfix" }],
      default: "backlog",
    };
    const fields = buildNodeFields(raw, undefined, {
      forwardNodeIds: ["hotfix", "standard", "backlog"],
    });
    expect(fieldByKey(fields, "routes")).toMatchObject({
      type: "routes",
      path: ["routes"],
      defaultPath: ["default"],
      routes: [{ when: 'inputs.severity == "p0"', to: "hotfix" }],
      defaultRoute: "backlog",
      targets: ["hotfix", "standard", "backlog"],
    });

    expect(fieldByKey(fields, "on_eval_error")).toMatchObject({
      type: "static",
      value: "fail",
      badge: "fail-closed",
    });
  });

  it("Should render an ask node's prompt, context, answer shape, responders, and expiry", () => {
    const fields = buildNodeFields({ id: "confirm", class: "control", kind: "ask", params: {} });
    expect(fieldByKey(fields, "ask_prompt")).toMatchObject({
      type: "textarea",
      path: ["params", "prompt"],
      required: true,
    });
    expect(fieldByKey(fields, "ask_context")).toMatchObject({
      path: ["params", "context"],
      json: true,
    });

    expect(fieldByKey(fields, "ask_expect")).toMatchObject({
      path: ["params", "expect"],
      json: true,
      required: true,
    });
    expect(fieldByKey(fields, "ask_responders_agents")).toMatchObject({
      type: "switch",
      path: ["params", "responders", "agents"],
    });
    expect(fieldByKey(fields, "ask_expires_after")).toMatchObject({
      path: ["params", "expires", "after"],
    });
  });

  it("Should give a fan-out the strategy composite rather than a bare enum", () => {
    const fields = buildNodeFields({
      id: "rollout",
      class: "control",
      kind: "fan-out",
      strategy: { kind: "best_effort", threshold: "66%", missing: "acceptable" },
    });
    expect(fieldByType(fields, "strategy")).toMatchObject({
      key: "strategy",
      basePath: ["strategy"],
      kind: "best_effort",
      threshold: "66%",
      missingAcceptable: true,
    });

    expect(
      fieldByType(buildNodeFields({ id: "j", class: "control", kind: "collect" }), "strategy")
    ).toBeUndefined();
  });

  it("Should give every action node the review composite", () => {
    for (const kind of ["run-agent", "goal", "run-loop", "transform", "compozy__network_send"]) {
      const fields = buildNodeFields({ id: "x", class: "action", kind, params: {} });
      expect(fieldByType(fields, "review")).toMatchObject({
        key: "review",
        basePath: ["review"],
        enabled: false,
      });
    }

    for (const node of [
      { id: "g", class: "control", kind: "gate" },
      { id: "s", class: "source", kind: "watch-source" },
    ]) {
      expect(fieldByType(buildNodeFields(node), "review")).toBeUndefined();
    }
  });

  it("Should offer branch and gate the inheritable eval-error select", () => {
    for (const kind of ["branch", "gate"]) {
      expect(
        fieldByKey(buildNodeFields({ id: "n", class: "control", kind }), "on_eval_error")
      ).toMatchObject({
        type: "select",
        path: ["on_eval_error"],
        options: ["fail", "exit"],
        clearOption: { value: "", label: "Inherit the loop policy" },
      });
    }
  });
});

describe("strategyEdits", () => {
  it("Should collapse wait_all, fail_fast, and race to the shorthand the DSL emits", () => {
    for (const kind of ["wait_all", "fail_fast", "race"] as const) {
      expect(strategyEdits({ kind, threshold: "", missingAcceptable: false })).toEqual([
        { path: ["strategy"], value: kind },
      ]);
    }
  });

  it("Should keep best_effort in its object form with the threshold the author wrote", () => {
    expect(
      strategyEdits({ kind: "best_effort", threshold: "66%", missingAcceptable: false })
    ).toEqual([{ path: ["strategy"], value: { kind: "best_effort", threshold: "66%" } }]);

    expect(strategyEdits({ kind: "best_effort", threshold: "3", missingAcceptable: true })).toEqual(
      [
        {
          path: ["strategy"],
          value: { kind: "best_effort", threshold: { count: 3 }, missing: "acceptable" },
        },
      ]
    );
  });

  it("Should clear the key entirely when the author declares no strategy", () => {
    expect(strategyEdits({ kind: null, threshold: "66%", missingAcceptable: true })).toEqual([
      { path: ["strategy"], value: undefined },
    ]);
  });

  it("Should round-trip a written strategy back through the descriptor", () => {
    const raw: RawLoopNode = { id: "rollout", class: "control", kind: "fan-out" };
    const declared = applyEdits(
      raw,
      strategyEdits({ kind: "best_effort", threshold: "3", missingAcceptable: true })
    );
    expect(readStrategy(declared)).toMatchObject({
      kind: "best_effort",
      threshold: "3",
      missingAcceptable: true,
    });

    const cleared = applyEdits(
      declared,
      strategyEdits({ kind: null, threshold: "", missingAcceptable: false })
    );
    expect(cleared.strategy).toBeUndefined();
    expect(readStrategy(cleared)).toMatchObject({ kind: null, threshold: "" });
  });
});

describe("reviewEdits", () => {
  it("Should write the whole review block in one transition", () => {
    const raw: RawLoopNode = { id: "apply", class: "action", kind: "run-agent" };
    const next = applyEdits(
      raw,
      reviewEdits({
        enabled: true,
        decisions: ["approve", "reject"],
        when: 'inputs.severity == "p0"',
        prompt: "apply proposes a migrate",
        agentsAllowed: true,
        onRejectRoute: "backlog",
        expiresAfter: "24h",
      })
    );
    expect(next.review).toEqual({
      decisions: ["approve", "reject"],
      when: 'inputs.severity == "p0"',
      prompt: "apply proposes a migrate",
      responders: { agents: "allow" },
      on_reject: { route: "backlog" },
      expires: { after: "24h" },
    });
    expect(readReview(next)).toMatchObject({ enabled: true, agentsAllowed: true });
  });

  it("Should omit every clause the author left blank instead of writing an empty one", () => {
    const written = applyEdits(
      { id: "apply", class: "action", kind: "run-agent" },
      reviewEdits({
        enabled: true,
        decisions: [],
        when: "  ",
        prompt: "",
        agentsAllowed: false,
        onRejectRoute: "",
        expiresAfter: "",
      })
    );

    expect(written.review).toEqual({});
  });

  it("Should clear the key when the review is switched off, never leave a half shape", () => {
    const enabled = applyEdits(
      { id: "apply", class: "action", kind: "run-agent" },
      reviewEdits({
        enabled: true,
        decisions: ["approve"],
        when: "",
        prompt: "",
        agentsAllowed: false,
        onRejectRoute: "",
        expiresAfter: "",
      })
    );
    const disabled = applyEdits(
      enabled,
      reviewEdits({
        enabled: false,
        decisions: ["approve"],
        when: "",
        prompt: "",
        agentsAllowed: false,
        onRejectRoute: "",
        expiresAfter: "",
      })
    );
    expect(disabled.review).toBeUndefined();
    expect(readReview(disabled).enabled).toBe(false);
  });
});

describe("routesEdit and routeDefaultEdit", () => {
  it("Should replace the whole ordered list, keeping declaration order as the tiebreak", () => {
    const raw: RawLoopNode = { id: "triage", class: "control", kind: "route", routes: [] };
    const next = applyEdits(
      raw,
      routesEdit([
        { when: 'inputs.severity == "p0"', to: "hotfix" },
        { when: 'inputs.severity == "p1"', to: "standard" },
      ])
    );
    expect(next.routes).toEqual([
      { when: 'inputs.severity == "p0"', to: "hotfix" },
      { when: 'inputs.severity == "p1"', to: "standard" },
    ]);
    expect(readRoutes(next).map(route => route.to)).toEqual(["hotfix", "standard"]);
  });

  it("Should write only the route keys the DSL declares", () => {
    const next = applyEdits(
      { id: "triage", class: "control", kind: "route" },
      routesEdit([{ when: "a", to: "x", stale: true } as unknown as { when: string; to: string }])
    );
    expect(next.routes).toEqual([{ when: "a", to: "x" }]);
  });

  it("Should clear the default key when the author empties the picker", () => {
    const raw: RawLoopNode = { id: "triage", class: "control", kind: "route", default: "backlog" };
    expect(applyEdits(raw, routeDefaultEdit("hotfix")).default).toBe("hotfix");
    expect(applyEdits(raw, routeDefaultEdit("")).default).toBeUndefined();
  });
});
