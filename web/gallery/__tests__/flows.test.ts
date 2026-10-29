// @vitest-environment node
import { mkdirSync, mkdtempSync, realpathSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { afterEach, describe, expect, it } from "vitest";

import type { FlowEntry, ResolvedEdge } from "../flow";
import {
  bfsOrder,
  coverage,
  deadEnds,
  duplicateEdges,
  membership,
  unreachable,
} from "../flow-graph";
import { findPrototypes } from "../node/discovery";
import { findFlows } from "../node/flows";
import type { SpecCatalog } from "../node/specs";

const edge = (from: string, to: string, on = "Open"): ResolvedEdge => ({
  from,
  to,
  on,
  kind: "navigate",
});

describe("flow graph rules", () => {
  const edges = [edge("a", "b"), edge("a", "c"), edge("b", "d"), edge("c", "a")];

  it("orders nodes breadth-first from the start, following edges in file order", () => {
    expect(bfsOrder(["a"], edges)).toEqual(["a", "b", "c", "d"]);
    expect(unreachable(["a", "b", "c", "d", "e"], bfsOrder(["a"], edges))).toEqual(["e"]);
  });

  it("reports dead ends unless the node declares end, and repeated edges", () => {
    const nodes = ["a", "b", "c", "d"].map(id => ({ id, end: id === "d" }));
    expect(deadEnds(nodes, edges)).toEqual([]);
    expect(deadEnds(nodes, edges.slice(0, 2))).toEqual(["b", "c"]);
    expect(duplicateEdges([edge("a", "b"), edge("a", "b", "Other"), edge("a", "b")])).toEqual([
      edge("a", "b"),
    ]);
  });

  it("maps prototype states to flow steps and warns about states no flow reaches", () => {
    const flow = {
      id: "journey",
      specs: ["kanban"],
      nodes: [
        { kind: "prototype", id: "board", proto: "board", state: "ready", step: 1 },
        { kind: "external", id: "gh", step: 2 },
      ],
    } as unknown as FlowEntry;
    expect(membership([flow]).get("board:ready")).toEqual([
      { flow: "journey", node: "board", step: 1, steps: 2 },
    ]);
    expect(
      coverage(
        [flow],
        [
          { slug: "board", spec: "kanban", states: ["ready", "empty"] },
          { slug: "other", spec: "elsewhere", states: ["ready"] },
          { slug: "loose", states: ["ready"] },
        ]
      )
    ).toEqual([{ code: "uncovered_state", spec: "kanban", slug: "board", state: "empty" }]);
  });
});

const roots: string[] = [];

afterEach(() => {
  for (const root of roots.splice(0)) rmSync(root, { recursive: true, force: true });
});

function project() {
  const root = realpathSync(mkdtempSync(join(tmpdir(), "compozy-flows-")));
  roots.push(root);
  const web = join(root, "web");
  const write = (file: string, body: string) => {
    const path = join(root, file);
    mkdirSync(join(path, ".."), { recursive: true });
    writeFileSync(path, body);
  };
  write(
    "web/src/systems/session/board.proto.tsx",
    `export const meta = definePrototype({ title: "Board", spec: "kanban", states: [{ id: "ready", label: "Ready" }, { id: "empty", label: "Empty" }] });\nexport default function Board() { return null; }`
  );
  return { root, web, write };
}

const catalog: SpecCatalog = { available: true, specs: [{ id: "kanban", title: "Kanban" }] };

function discover(app: ReturnType<typeof project>, specs = catalog) {
  const prototypes = findPrototypes(app.web, app.root, specs).entries;
  return findFlows(app.web, app.root, prototypes, specs);
}

const flowFile = (body: string) =>
  `import { defineFlow } from "#gallery/flow";\nexport const flow = defineFlow(${body});\nthrow new Error("must not execute");`;

const validFlow = `{
  title: "Journey",
  spec: "kanban",
  start: ["board"],
  nodes: {
    board: { proto: "board", state: "ready" },
    gh: { external: "github", url: "https://github.com/compozy/compozy/pull/1", end: true },
  },
  edges: [["board", "gh", { on: "Open PR", kind: "external" }]],
}`;

describe("flow discovery", () => {
  it("resolves a literal flow without executing it, in journey order", () => {
    const app = project();
    app.write("web/gallery/flows/journey.flow.ts", flowFile(validFlow));
    const result = discover(app);
    expect(result.problems).toEqual([]);
    expect(result.flows).toHaveLength(1);
    const [flow] = result.flows;
    expect(flow).toMatchObject({
      id: "journey",
      file: "web/gallery/flows/journey.flow.ts",
      spec: "kanban",
      specs: ["kanban"],
      edges: [{ from: "board", to: "gh", on: "Open PR", kind: "external" }],
    });
    expect(flow?.nodes.map(node => [node.id, node.step, node.start, node.end])).toEqual([
      ["board", 1, true, false],
      ["gh", 2, false, true],
    ]);
    expect(flow?.nodes[1]).toMatchObject({ kind: "external", label: "GitHub" });
    expect(result.warnings).toEqual([
      { code: "uncovered_state", spec: "kanban", slug: "board", state: "empty" },
    ]);
  });

  it.each([
    [
      "unknown prototype and state",
      `{ title: "J", start: ["a"], nodes: { a: { proto: "nope", state: "x" }, b: { proto: "board", state: "gone", end: true } }, edges: [["a", "b", { on: "Go" }]] }`,
      ["unknown_prototype", "unknown_state"],
    ],
    [
      "unreachable nodes and dead ends",
      `{ title: "J", start: ["a"], nodes: { a: { proto: "board", state: "ready" }, b: { external: "docs", end: true } }, edges: [] }`,
      ["unreachable", "dead_end"],
    ],
    [
      "duplicate edges and undeclared nodes",
      `{ title: "J", start: ["a"], nodes: { a: { proto: "board", state: "ready" } }, edges: [["a", "a", { on: "Go" }], ["a", "a", { on: "Go" }], ["a", "z", { on: "Go" }]] }`,
      ["unknown_node", "duplicate_edge"],
    ],
    [
      "an end node with an outgoing edge and an external edge inside the app",
      `{ title: "J", start: ["a"], nodes: { a: { proto: "board", state: "ready", end: true }, b: { proto: "board", state: "empty", end: true } }, edges: [["a", "b", { on: "Go", kind: "external" }]] }`,
      ["end_with_edges", "external_edge"],
    ],
    [
      "unknown external kinds, missing labels and unsafe URLs",
      `{ title: "J", start: ["a"], nodes: { a: { external: "slack", end: true }, b: { external: "other", end: true }, c: { external: "github", url: "http://x.dev", end: true } }, edges: [["a", "b", { on: "Go" }], ["a", "c", { on: "Go" }]] }`,
      ["invalid_flow", "invalid_flow", "invalid_flow"],
    ],
    [
      "an unknown owner spec",
      `{ title: "J", spec: "nope", start: ["a"], nodes: { a: { proto: "board", state: "ready", end: true } }, edges: [] }`,
      ["unknown_spec"],
    ],
    [
      "repeated nodes",
      `{ title: "J", start: ["a"], nodes: { a: { proto: "board", state: "ready", end: true }, a: { proto: "board", state: "empty", end: true } }, edges: [] }`,
      ["duplicate_node"],
    ],
    [
      "computed values",
      `{ title: "J" + "x", start: ["a"], nodes: {}, edges: [] }`,
      ["missing_flow"],
    ],
  ])("reports %s", (_, body, expected) => {
    const app = project();
    app.write("web/gallery/flows/journey.flow.ts", flowFile(body));
    const result = discover(app);
    expect(result.flows).toEqual([]);
    expect(result.problems.map(problem => problem.code)).toEqual(expected);
  });

  it("skips spec validation when the spec catalog is absent", () => {
    const app = project();
    app.write(
      "web/gallery/flows/journey.flow.ts",
      flowFile(validFlow.replace('"kanban"', '"nope"'))
    );
    expect(discover(app, { available: false, specs: [] }).problems).toEqual([]);
  });

  it("reports flows outside web/gallery/flows and invalid ids", () => {
    const app = project();
    app.write("web/src/systems/session/journey.flow.ts", flowFile(validFlow));
    app.write("web/gallery/flows/Bad_Id.flow.ts", flowFile(validFlow));
    expect(discover(app).problems.map(problem => [problem.code, problem.file])).toEqual([
      ["invalid_flow_id", "web/gallery/flows/Bad_Id.flow.ts"],
      ["flow_outside_root", "web/src/systems/session/journey.flow.ts"],
    ]);
  });
});
