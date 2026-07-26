import { describe, expect, it } from "vitest";

import type { RawLoopNode } from "../codec";
import { buildNodeFields, type FieldSpec } from "../loop-node-schema";

function keys(fields: FieldSpec[]): string[] {
  return fields.map(field => ("key" in field ? field.key : "hint"));
}

describe("loop node schema", () => {
  it("Should render run-agent with required agent + prompt and the overrides fold", () => {
    const raw: RawLoopNode = { id: "execute", class: "action", kind: "run-agent", params: {} };
    const fields = buildNodeFields(raw);
    const agent = fields.find(f => "key" in f && f.key === "agent");
    const prompt = fields.find(f => "key" in f && f.key === "prompt");
    expect(agent).toMatchObject({ required: true, reference: true });
    expect(prompt).toMatchObject({ type: "textarea", required: true, reference: true });
    // No `action_ref` and no closed kind select (round-5 delete targets, ADR-021).
    expect(keys(fields)).not.toContain("action_ref");
    expect(keys(fields)).toContain("overrides");
  });

  it("Should render fan-out max_fan_out with the daemon ceiling", () => {
    const raw: RawLoopNode = { id: "implement", class: "control", kind: "fan-out" };
    const fanOut = buildNodeFields(raw).find(f => "key" in f && f.key === "max_fan_out");
    expect(fanOut).toMatchObject({ type: "number", ceiling: 64 });
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
    // Routing is editable in-inspector (not a read-only summary) — R-004 remediation.
    const onPass = fields.find(f => "key" in f && f.key === "on_pass");
    const onFail = fields.find(f => "key" in f && f.key === "on_fail");
    expect(onPass).toMatchObject({ type: "text", path: ["on_result", "pass"] });
    expect(onFail).toMatchObject({ type: "text", path: ["on_result", "fail"] });
  });

  it("Should mark branch.condition as a CEL reference field and give run-loop editable inputs", () => {
    const branch = buildNodeFields({ id: "b", class: "control", kind: "branch" }).find(
      f => "key" in f && f.key === "condition"
    );
    expect(branch).toMatchObject({ reference: true, cel: true });
    const runLoop = buildNodeFields({ id: "child", class: "action", kind: "run-loop" });
    expect(runLoop.some(f => "key" in f && f.key === "inputs" && "json" in f && f.json)).toBe(true);
    const cwd = buildNodeFields({ id: "x", class: "action", kind: "run-agent" }).find(
      f => "key" in f && f.key === "cwd"
    );
    expect(cwd).toMatchObject({ reference: true });
  });

  it("Should render any non-reserved action kind as a generic ToolID form with editable kind", () => {
    const raw: RawLoopNode = { id: "post", class: "action", kind: "agh__network_send", params: {} };
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
    // The Markdown-task parse value left the core: file-import parses only the two
    // structured formats now (ADR-002 — the extension owns Compozy task import).
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
});
