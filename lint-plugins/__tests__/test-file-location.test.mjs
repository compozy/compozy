import { describe, expect, it } from "vitest";
import plugin from "../test-file-location.mjs";

describe("deterministic unit test conventions", () => {
  const rule = plugin.rules["deterministic-tests"];
  function messages(callee, args, filename = "example.test.ts", options = [], sourceCode) {
    const reports = [];
    rule
      .create({ filename, options, sourceCode, report: report => reports.push(report) })
      .CallExpression?.({
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
  it("resolves static keys and constant conditional skips without guessing dynamic values", () => {
    const base = { type: "Identifier", name: "test" };
    const member = (property, computed = true) => ({
      type: "MemberExpression",
      object: base,
      property,
      computed,
    });
    const literal = value => ({ type: "Literal", value });
    const template = value => ({
      type: "TemplateLiteral",
      expressions: [],
      quasis: [{ value: { cooked: value } }],
    });
    const mode = { type: "Identifier", name: "mode" };
    const variable = {
      defs: [{ parent: { kind: "const" }, node: { id: mode, init: literal("skip") } }],
    };
    const sourceCode = { getScope: () => ({ set: new Map([["mode", variable]]) }) };
    expect(messages(member(mode), [], undefined, [], sourceCode)).toHaveLength(1);
    variable.defs[0].parent.kind = "let";
    expect(messages(member(mode), [], undefined, [], sourceCode)).toHaveLength(0);
    variable.defs[0].parent.kind = "const";
    variable.defs[0].node.init = mode;
    expect(messages(member(mode), [], undefined, [], sourceCode)).toHaveLength(0);
    expect(messages(member(template("skip")), [])).toHaveLength(1);
    for (const [name, value, count] of [
      ["skipIf", true, 1],
      ["runIf", false, 1],
      ["skipIf", false, 0],
      ["runIf", true, 0],
      ["skipIf", undefined, 0],
    ]) {
      const factory = {
        type: "CallExpression",
        callee: member({ name }, false),
        arguments: [literal(value)],
      };
      expect(messages(factory, [{ value: "case" }, {}])).toHaveLength(count);
    }
    const options = key => [
      { value: "case" },
      { type: "ObjectExpression", properties: [{ computed: true, key, value: literal(1000) }] },
      {},
    ];
    expect(messages(base, options(template("timeout")))).toHaveLength(0);
    expect(messages(base, options(template("retry")))).toHaveLength(1);
  });
  it("checks inherited suite options and statically negated skip conditions", () => {
    const suite = { type: "Identifier", name: "describe" };
    const options = (name, value) => [
      { value: "suite" },
      { type: "ObjectExpression", properties: [{ key: { name }, value: { value } }] },
      {},
    ];
    expect(messages(suite, options("retry", 2))).toHaveLength(1);
    expect(messages(suite, options("timeout", 6000))).toHaveLength(1);
    expect(messages(suite, options("timeout", 1000))).toHaveLength(0);
    const factory = (name, argument) => ({
      type: "CallExpression",
      callee: {
        type: "MemberExpression",
        object: { type: "Identifier", name: "it" },
        property: { name },
      },
      arguments: [{ type: "UnaryExpression", operator: "!", argument }],
    });
    expect(
      messages(factory("skipIf", { type: "Literal", value: false }), [{ value: "case" }, {}])
    ).toHaveLength(1);
    expect(
      messages(factory("runIf", { type: "Literal", value: true }), [{ value: "case" }, {}])
    ).toHaveLength(1);
    expect(
      messages(factory("skipIf", { type: "Literal", value: true }), [{ value: "case" }, {}])
    ).toHaveLength(0);
    expect(
      messages(factory("skipIf", { type: "Identifier", name: "dynamic" }), [{ value: "case" }, {}])
    ).toHaveLength(0);
  });
  it("distinguishes global undefined from shadowed and unresolved skip conditions", () => {
    const identifier = { type: "Identifier", name: "undefined" };
    const callee = {
      type: "CallExpression",
      callee: {
        type: "MemberExpression",
        object: { type: "Identifier", name: "it" },
        property: { name: "skipIf" },
      },
      arguments: [{ type: "UnaryExpression", operator: "!", argument: identifier }],
    };
    const variable = { defs: [] };
    const sourceCode = { getScope: () => ({ set: new Map([["undefined", variable]]) }) };
    expect(messages(callee, [])).toHaveLength(1);
    expect(messages(callee, [], undefined, [], sourceCode)).toHaveLength(1);
    variable.defs = [
      {
        parent: { kind: "const" },
        node: { id: identifier, init: { type: "Literal", value: true } },
      },
    ];
    expect(messages(callee, [], undefined, [], sourceCode)).toHaveLength(0);
    variable.defs = [{ type: "Parameter" }];
    expect(messages(callee, [], undefined, [], sourceCode)).toHaveLength(0);
  });
  it("checks suite aliases and renamed Vitest imports through lexical bindings", () => {
    const variable = {
      defs: [
        {
          type: "ImportBinding",
          parent: { source: { value: "vitest" } },
          node: { imported: { name: "describe" } },
        },
      ],
    };
    const sourceCode = { getScope: () => ({ set: new Map([["group", variable]]) }) };
    for (const name of ["describe", "suite", "group"]) {
      const root = { type: "Identifier", name };
      const options = [
        { type: "ObjectExpression", properties: [{ key: { name: "retry" }, value: { value: 2 } }] },
      ];
      expect(messages(root, options, undefined, [], sourceCode)).toHaveLength(1);
      const skip = { type: "MemberExpression", object: root, property: { name: "skip" } };
      expect(messages(skip, [], undefined, [], sourceCode)).toHaveLength(1);
      const conditional = {
        type: "CallExpression",
        callee: { ...skip, property: { name: "skipIf" } },
        arguments: [
          { type: "UnaryExpression", operator: "!", argument: { type: "Literal", value: false } },
        ],
      };
      expect(messages(conditional, [], undefined, [], sourceCode)).toHaveLength(1);
    }
    variable.defs = [{ type: "Parameter" }];
    const skip = {
      type: "MemberExpression",
      object: { type: "Identifier", name: "group" },
      property: { name: "skip" },
    };
    expect(messages(skip, [], undefined, [], sourceCode)).toHaveLength(0);
  });
});
