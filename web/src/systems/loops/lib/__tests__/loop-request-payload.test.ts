// Suite: Loop request payload fields
// Invariant: every schema-derived control serializes the exact JSON value represented by that control.
// Boundary IN: request-field projection, input parsing, and payload assembly.
// Boundary OUT: rendered controls and daemon-side JSON Schema validation.

import { describe, expect, it } from "vitest";

import {
  checkLoopRequestFields,
  loopRequestFieldLabel,
  loopRequestFields,
  loopRequestFieldSeed,
  type LoopRequestField,
} from "../loop-request-payload";

function field(fields: readonly LoopRequestField[], name: string): LoopRequestField {
  const found = fields.find(candidate => candidate.name === name);
  if (!found) throw new Error(`missing field ${name}`);
  return found;
}

function optionToken(fields: readonly LoopRequestField[], name: string, index = 0): string {
  const control = field(fields, name).control;
  if (control.kind !== "select") throw new Error(`${name} is not a select`);
  const option = control.options[index];
  if (!option) throw new Error(`${name} has no option ${index}`);
  return option.token;
}

describe("loop request payload fields", () => {
  it("Should preserve every JSON enum value with or without a declared type", () => {
    const fields = loopRequestFields({
      type: "object",
      required: ["decision", "count", "enabled", "nothing", "list", "record"],
      properties: {
        decision: { enum: ["approve", "discard"] },
        count: { type: "integer", enum: [2, 4] },
        enabled: { enum: [true, false] },
        nothing: { enum: [null] },
        list: { enum: [["one", "two"]] },
        record: { enum: [{ mode: "safe", retries: 2 }] },
      },
    });

    expect(
      checkLoopRequestFields(fields, {
        decision: optionToken(fields, "decision"),
        count: optionToken(fields, "count", 1),
        enabled: optionToken(fields, "enabled", 1),
        nothing: optionToken(fields, "nothing"),
        list: optionToken(fields, "list"),
        record: optionToken(fields, "record"),
      })
    ).toEqual({
      ok: true,
      errors: {},
      payload: {
        decision: "approve",
        count: 4,
        enabled: false,
        nothing: null,
        list: ["one", "two"],
        record: { mode: "safe", retries: 2 },
      },
    });
  });

  it("Should use primitive controls only for unambiguous primitive schemas", () => {
    const fields = loopRequestFields({
      type: "object",
      required: ["name", "ratio", "count", "enabled", "items", "metadata", "unknown"],
      properties: {
        name: { type: "string", minLength: 3 },
        ratio: { type: "number", minimum: 0 },
        count: { type: "integer" },
        enabled: { type: "boolean" },
        items: { type: "array", items: { type: "string" } },
        metadata: { type: "object" },
        unknown: {},
        union: { type: ["string", "null"] },
        choice: { oneOf: [{ type: "string" }, { type: "number" }] },
      },
    });

    expect(fields.map(candidate => [candidate.name, candidate.control.kind])).toEqual([
      ["name", "text"],
      ["ratio", "number"],
      ["count", "integer"],
      ["enabled", "boolean"],
      ["items", "json"],
      ["metadata", "json"],
      ["unknown", "json"],
      ["union", "json"],
      ["choice", "json"],
    ]);
    expect(
      checkLoopRequestFields(fields, {
        name: "ok",
        ratio: "1.5",
        count: "2",
        enabled: "true",
        items: '["a"]',
        metadata: '{"mode":"safe"}',
        unknown: "null",
        union: '"value"',
        choice: "3",
      })
    ).toEqual({
      ok: true,
      errors: {},
      payload: {
        name: "ok",
        ratio: 1.5,
        count: 2,
        enabled: true,
        items: ["a"],
        metadata: { mode: "safe" },
        unknown: null,
        union: "value",
        choice: 3,
      },
    });
  });

  it("Should project nested entity annotations while keeping enum precedence", () => {
    const fields = loopRequestFields({
      type: "object",
      required: ["assignment", "decision"],
      properties: {
        assignment: {
          type: "object",
          required: ["reviewer"],
          properties: {
            reviewer: { type: "string", "x-compozy-kind": "agent" },
          },
        },
        decision: {
          type: "string",
          enum: ["approve", "reject"],
          "x-compozy-kind": "agent",
        },
      },
    });

    expect(field(fields, "assignment.reviewer")).toMatchObject({
      name: "assignment.reviewer",
      required: true,
    });
    expect(field(fields, "assignment.reviewer").control).toEqual({
      kind: "entity",
      entityKind: "agent",
    });
    expect(field(fields, "decision").control.kind).toBe("select");
    expect(
      checkLoopRequestFields(fields, {
        "assignment.reviewer": "reviewer",
        decision: optionToken(fields, "decision", 1),
      })
    ).toEqual({
      ok: true,
      errors: {},
      payload: { assignment: { reviewer: "reviewer" }, decision: "reject" },
    });
    expect(loopRequestFieldSeed(fields, { assignment: { reviewer: "reviewer" } })).toMatchObject({
      "assignment.reviewer": "reviewer",
    });
  });

  it("Should reject values a control cannot serialize and leave schema constraints to the daemon", () => {
    const fields = loopRequestFields({
      type: "object",
      required: ["decision", "count", "enabled", "metadata"],
      properties: {
        decision: { enum: ["approve"] },
        count: { type: "integer", minimum: 10 },
        enabled: { type: "boolean" },
        metadata: { type: "object" },
      },
    });

    expect(
      checkLoopRequestFields(fields, {
        decision: "not-an-option",
        count: "2.5",
        enabled: "yes",
        metadata: "not-json",
      })
    ).toEqual({
      ok: false,
      errors: {
        decision: "Decision must be an offered value.",
        count: "Count must be a whole number.",
        enabled: "Enabled must be Yes or No.",
        metadata: "Metadata must be JSON.",
      },
    });

    expect(
      checkLoopRequestFields(fields, {
        decision: optionToken(fields, "decision"),
        count: "2",
        enabled: "false",
        metadata: "{}",
      })
    ).toEqual({
      ok: true,
      errors: {},
      payload: { decision: "approve", count: 2, enabled: false, metadata: {} },
    });
  });

  it("Should humanize field names once for labels and error copy alike", () => {
    expect(loopRequestFieldLabel({ name: "regions" })).toBe("Regions");
    expect(loopRequestFieldLabel({ name: "migration_url" })).toBe("Migration url");
    expect(loopRequestFieldLabel({ name: "dryRun" })).toBe("Dry run");
    expect(loopRequestFieldLabel({ name: "retry-count" })).toBe("Retry count");
    expect(loopRequestFieldLabel({ name: "assignment.reviewer" })).toBe("Reviewer");
  });

  it("Should seed controls from exact typed values without confusing null or empty string with unset", () => {
    const fields = loopRequestFields({
      type: "object",
      properties: {
        empty: { enum: [""] },
        nothing: { enum: [null] },
        record: { enum: [{ first: 1, second: 2 }] },
        omitted: { type: "string" },
      },
    });

    expect(
      loopRequestFieldSeed(fields, {
        empty: "",
        nothing: null,
        record: { second: 2, first: 1 },
      })
    ).toEqual({
      empty: optionToken(fields, "empty"),
      nothing: optionToken(fields, "nothing"),
      record: optionToken(fields, "record"),
      omitted: "",
    });
  });
});
