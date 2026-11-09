import { describe, expect, it } from "vitest";
import plugin from "../test-file-location.mjs";

describe("deterministic unit test conventions", () => {
  const rule = plugin.rules["deterministic-tests"];
  function messages(callee, args, filename = "example.test.ts", options = []) {
    const reports = [];
    rule.create({ filename, options, report: report => reports.push(report) }).CallExpression?.({
      type: "CallExpression",
      callee,
      arguments: args,
    });
    return reports;
  }
  it("rejects numeric sleeps while allowing indefinite I/O stubs", () => {
    const callee = { type: "Identifier", name: "delay" };
    expect(messages(callee, [{ value: 50 }])).toHaveLength(1);
    expect(messages(callee, [{ value: "infinite" }])).toHaveLength(0);
  });
  it("rejects unconditional unit skips without blanket banning process integration contracts", () => {
    const callee = {
      type: "MemberExpression",
      object: { name: "test" },
      property: { name: "skip" },
    };
    expect(messages(callee, [])).toHaveLength(1);
    expect(messages(callee, [], "process.spec.ts")).toHaveLength(0);
  });
  it("rejects disabled deadlines in positional and options forms", () => {
    const callee = { type: "Identifier", name: "it" };
    for (const deadline of [
      { type: "Literal", value: 0 },
      { type: "UnaryExpression", operator: "-", argument: { type: "Literal", value: 1 } },
      { type: "Identifier", name: "Infinity" },
    ]) {
      expect(messages(callee, [{ value: "name" }, {}, deadline])).toHaveLength(1);
      expect(
        messages(callee, [
          { value: "name" },
          {
            type: "ObjectExpression",
            properties: [{ key: { name: "timeout" }, value: deadline }],
          },
          {},
        ])
      ).toHaveLength(1);
    }
    expect(
      messages(callee, [
        { value: "name" },
        {
          type: "ObjectExpression",
          properties: [{ key: { name: "timeout" }, value: { type: "Literal", value: 1 } }],
        },
        {},
      ])
    ).toHaveLength(0);
  });
  it("rejects timeout increases and retries while preserving stricter deadlines", () => {
    const callee = { type: "Identifier", name: "it" };
    expect(messages(callee, [{ value: "name" }, {}, { value: 1000 }])).toHaveLength(0);
    expect(messages(callee, [{ value: "name" }, {}, { value: 30000 }])).toHaveLength(1);
    expect(
      messages(callee, [{ value: "name" }, {}, { value: 10000 }], "web.test.ts", [
        { defaultTimeout: 20000 },
      ])
    ).toHaveLength(0);
    expect(
      messages(callee, [{ type: "ObjectExpression", properties: [{ key: { name: "retry" } }] }])
    ).toHaveLength(1);
    expect(messages(callee, [{ value: "name" }, {}])).toHaveLength(0);
  });
});
