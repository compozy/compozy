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
      object: { type: "Identifier", name: "test" },
      property: { name: "skip" },
    };
    expect(messages(callee, [])).toHaveLength(1);
    expect(messages(callee, [], "/repo/web/e2e/process.spec.ts")).toHaveLength(0);
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
  it("checks modified and parameterized test declarations without checking their data factories", () => {
    const member = (object, name) => ({
      type: "MemberExpression",
      object,
      property: { name },
    });
    for (const name of ["it", "test"]) {
      const base = { type: "Identifier", name };
      const concurrent = member(base, "concurrent");
      const each = { type: "CallExpression", callee: member(base, "each") };
      for (const callee of [
        concurrent,
        each,
        { type: "CallExpression", callee: member(concurrent, "each") },
        { type: "TaggedTemplateExpression", tag: member(base, "each") },
      ]) {
        expect(messages(callee, [{ value: "case" }, {}, { value: 0 }])).toHaveLength(1);
        for (const key of ["timeout", "retry"]) {
          expect(
            messages(callee, [
              { value: "case" },
              {
                type: "ObjectExpression",
                properties: [{ key: { value: key }, value: { value: 0 } }],
              },
              {},
            ])
          ).toHaveLength(1);
        }
        expect(messages(callee, [{ value: "case" }, {}, { value: 1000 }])).toHaveLength(0);
      }
      expect(messages(member(base, "each"), [{}, {}, { value: 0 }])).toHaveLength(0);
    }
    expect(
      messages(member({ type: "Identifier", name: "other" }, "concurrent"), [{}, {}, { value: 0 }])
    ).toHaveLength(0);
  });
  it("checks unit specs, chained skips and spread options while exempting E2E specs", () => {
    const base = { type: "Identifier", name: "test" };
    const concurrent = { type: "MemberExpression", object: base, property: { name: "concurrent" } };
    const skip = { type: "MemberExpression", object: concurrent, property: { name: "skip" } };
    expect(messages(skip, [], "/repo/src/example.spec.ts")).toHaveLength(1);
    expect(messages(skip, [], "/repo/desktop/e2e/example.spec.ts")).toHaveLength(0);
    const option = value => ({
      type: "ObjectExpression",
      properties: [{ type: "SpreadElement", argument: value }],
    });
    expect(
      messages(base, [{ value: "name" }, option({ type: "Identifier", name: "shared" }), {}])
    ).toHaveLength(1);
    for (const key of ["retry", "timeout"]) {
      expect(
        messages(base, [
          { value: "name" },
          option({
            type: "ObjectExpression",
            properties: [{ key: { name: key }, value: { value: 0 } }],
          }),
          {},
        ])
      ).toHaveLength(1);
    }
    expect(
      messages(base, [
        { value: "name" },
        option({
          type: "ObjectExpression",
          properties: [{ key: { name: "timeout" }, value: { value: 1000 } }],
        }),
        {},
      ])
    ).toHaveLength(0);
  });
  it("checks computed skip members and rejects unresolved option keys", () => {
    const base = { type: "Identifier", name: "test" };
    const member = value => ({
      type: "MemberExpression",
      computed: true,
      object: base,
      property: { type: "Literal", value },
    });
    expect(messages(member("skip"), [])).toHaveLength(1);
    expect(messages(member("only"), [])).toHaveLength(0);
    expect(
      messages({ ...member("skip"), property: { type: "Identifier", name: "skip" } }, [])
    ).toHaveLength(0);
    const options = (key, value) => [
      { value: "name" },
      { type: "ObjectExpression", properties: [{ computed: true, key, value: { value } }] },
      {},
    ];
    expect(messages(base, options({ type: "Identifier", name: "k" }, 3))).toHaveLength(1);
    expect(messages(base, options({ type: "Literal", value: "retry" }, 3))).toHaveLength(1);
    expect(messages(base, options({ type: "Literal", value: "timeout" }, 0))).toHaveLength(1);
    expect(messages(base, options({ type: "Literal", value: "timeout" }, 1000))).toHaveLength(0);
  });
});
