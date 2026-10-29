/**
 * Finds every journey flow (`web/gallery/flows/<id>.flow.ts`) by reading the syntax tree only, and
 * resolves it against the discovered prototypes: nodes name existing prototype states or known
 * external places, edges name declared nodes, every node is reachable from `start` and every node
 * without an outgoing edge declares `end: true`. Problems fail `proto check` and the build;
 * coverage warnings (states no flow reaches) never do.
 */
import { existsSync, readdirSync, readFileSync } from "node:fs";
import { join, relative } from "node:path";
import ts from "typescript";

import { bfsOrder, coverage, deadEnds, duplicateEdges, unreachable } from "../flow-graph.ts";
import {
  edgeKinds,
  externalLabels,
  type CoverageWarning,
  type EdgeKind,
  type ExternalService,
  type FlowEntry,
  type FlowProblem,
  type FlowProblemCode,
  type ResolvedEdge,
  type ResolvedNode,
} from "../flow.ts";
import type { PrototypeEntry } from "./discovery.ts";
import { LiteralError, readLiteral } from "./literal.ts";
import type { SpecCatalog } from "./specs.ts";

export type {
  CoverageWarning,
  FlowEntry,
  FlowProblem,
  FlowProblemCode,
  ResolvedEdge,
  ResolvedNode,
} from "../flow.ts";

const flowSuffix = ".flow.ts";
const ignored = new Set(["node_modules", "dist", "dist-gallery", "comments", "coverage"]);
const edgeFields = new Set(["on", "kind"]);
const flowFields = new Set(["title", "spec", "description", "start", "nodes", "edges"]);
const knownEdgeKinds = new Set<EdgeKind>(edgeKinds);

export const validFlowId = (id: string) => /^[a-z0-9]+(-[a-z0-9]+)*$/.test(id) && id.length <= 120;
const text = (value: unknown): value is string =>
  typeof value === "string" && value.trim().length > 0;
const object = (value: unknown): value is Record<string, unknown> =>
  value !== null && typeof value === "object" && !Array.isArray(value);
const onlyKeys = (value: Record<string, unknown>, keys: readonly string[]) => {
  const allowed = new Set(keys);
  return Object.keys(value).every(key => allowed.has(key));
};

type Report = (code: FlowProblemCode, message: string, node?: string) => void;

function files(root: string): string[] {
  if (!existsSync(root)) return [];
  return readdirSync(root, { withFileTypes: true }).flatMap(entry => {
    if (entry.name.startsWith(".") || ignored.has(entry.name)) return [];
    const path = join(root, entry.name);
    if (entry.isDirectory()) return files(path);
    return entry.isFile() && entry.name.endsWith(flowSuffix) ? [path] : [];
  });
}

/** The literal argument of `export const flow = defineFlow({...})`. */
function declaration(path: string): unknown {
  const source = ts.createSourceFile(
    path,
    readFileSync(path, "utf8"),
    ts.ScriptTarget.Latest,
    true
  );
  for (const statement of source.statements) {
    if (
      !ts.isVariableStatement(statement) ||
      !statement.modifiers?.some(modifier => modifier.kind === ts.SyntaxKind.ExportKeyword)
    ) {
      continue;
    }
    for (const item of statement.declarationList.declarations) {
      const value = item.initializer;
      if (
        !ts.isIdentifier(item.name) ||
        item.name.text !== "flow" ||
        !value ||
        !ts.isCallExpression(value) ||
        !ts.isIdentifier(value.expression) ||
        value.expression.text !== "defineFlow" ||
        value.arguments.length !== 1 ||
        !value.arguments[0]
      ) {
        continue;
      }
      return readLiteral(value.arguments[0]);
    }
  }
  throw new LiteralError("not_literal");
}

/** Absolute HTTPS URLs without credentials; anything else is refused. */
function externalUrl(value: string): string | null {
  if (/\s|\\/.test(value) || !value.startsWith("https://")) return null;
  try {
    const url = new URL(value);
    return url.protocol === "https:" && !url.username && !url.password ? url.href : null;
  } catch {
    return null;
  }
}

function resolvePrototypeNode(
  id: string,
  value: Record<string, unknown>,
  position: { id: string; end: boolean; start: boolean; step: number },
  prototypes: readonly PrototypeEntry[],
  report: Report
): ResolvedNode | null {
  if (!text(value.proto) || !text(value.state) || !onlyKeys(value, ["proto", "state", "end"])) {
    report("invalid_flow", `node "${id}": expected proto, state and optional end`, id);
    return null;
  }
  const meta = prototypes.find(entry => entry.slug === value.proto)?.meta;
  if (!meta) {
    report(
      "unknown_prototype",
      `node "${id}" uses prototype "${value.proto}" that does not exist`,
      id
    );
    return null;
  }
  const state = meta.states.find(option => option.id === value.state);
  if (!state) {
    report(
      "unknown_state",
      `node "${id}" uses state "${value.state}" that ${value.proto} does not declare (${meta.states.map(option => option.id).join(", ")})`,
      id
    );
    return null;
  }
  return {
    ...position,
    kind: "prototype",
    proto: value.proto,
    state: state.id,
    stateLabel: state.label,
    title: meta.title,
    route: meta.route ?? null,
    spec: meta.spec ?? null,
  };
}

function resolveExternalNode(
  id: string,
  value: Record<string, unknown>,
  position: { id: string; end: boolean; start: boolean; step: number },
  report: Report
): ResolvedNode | null {
  if (
    !text(value.external) ||
    !Object.hasOwn(externalLabels, value.external) ||
    !onlyKeys(value, ["external", "label", "url", "end"]) ||
    (value.label !== undefined && !text(value.label)) ||
    (value.external === "other" && !text(value.label))
  ) {
    report(
      "invalid_flow",
      `node "${id}": external must be ${Object.keys(externalLabels).join(", ")}; "other" needs a label`,
      id
    );
    return null;
  }
  const url = typeof value.url === "string" ? externalUrl(value.url) : null;
  if (value.url !== undefined && !url) {
    report("invalid_flow", `node "${id}": url must be an absolute https:// URL`, id);
    return null;
  }
  const external = value.external as ExternalService;
  return {
    ...position,
    kind: "external",
    external,
    label: typeof value.label === "string" ? value.label : externalLabels[external],
    url,
  };
}

function resolveNode(
  id: string,
  value: unknown,
  start: readonly string[],
  prototypes: readonly PrototypeEntry[],
  report: Report
): ResolvedNode | null {
  if (!object(value) || !validFlowId(id) || "proto" in value === "external" in value) {
    report("invalid_node", `node "${id}" must set either proto and state, or external`, id);
    return null;
  }
  if (value.end !== undefined && typeof value.end !== "boolean") {
    report("invalid_flow", `node "${id}": end must be a boolean`, id);
    return null;
  }
  const position = { id, end: value.end === true, start: start.includes(id), step: 0 };
  return "proto" in value
    ? resolvePrototypeNode(id, value, position, prototypes, report)
    : resolveExternalNode(id, value, position, report);
}

function resolveEdges(
  id: string,
  raw: readonly unknown[],
  declared: ReadonlySet<string>,
  report: Report
) {
  const edges: ResolvedEdge[] = [];
  for (const edge of raw) {
    if (
      !Array.isArray(edge) ||
      edge.length !== 3 ||
      !text(edge[0]) ||
      !text(edge[1]) ||
      !object(edge[2]) ||
      !text(edge[2].on) ||
      Object.keys(edge[2]).some(key => !edgeFields.has(key)) ||
      (edge[2].kind !== undefined && !knownEdgeKinds.has(edge[2].kind as EdgeKind))
    ) {
      report("invalid_flow", `${id}: edges must be [from, to, { on, kind? }] with a known kind`);
      continue;
    }
    const [from, to, options] = edge as [string, string, { on: string; kind?: EdgeKind }];
    for (const node of new Set([from, to])) {
      if (!declared.has(node)) {
        report(
          "unknown_node",
          `edge ${from} → ${to} names node "${node}" that the flow does not declare`,
          node
        );
      }
    }
    edges.push({ from, to, on: options.on, kind: options.kind ?? "navigate" });
  }
  return edges;
}

type FlowShape = {
  title: string;
  spec?: string;
  description?: string;
  start: string[];
  nodes: Record<string, unknown>;
  edges: unknown[];
};

function isFlowShape(value: unknown): value is FlowShape {
  return (
    object(value) &&
    text(value.title) &&
    (value.spec === undefined || text(value.spec)) &&
    (value.description === undefined || text(value.description)) &&
    Array.isArray(value.start) &&
    value.start.length > 0 &&
    value.start.every(text) &&
    object(value.nodes) &&
    Object.keys(value.nodes).length > 0 &&
    Array.isArray(value.edges) &&
    Object.keys(value).every(key => flowFields.has(key))
  );
}

function resolveFlow(
  id: string,
  file: string,
  value: unknown,
  context: { prototypes: readonly PrototypeEntry[]; catalog: SpecCatalog },
  report: Report
): FlowEntry | null {
  if (!isFlowShape(value)) {
    report(
      "invalid_flow",
      `${id}: expected title, optional spec/description, non-empty start/nodes and edges`
    );
    return null;
  }
  const { catalog } = context;
  if (value.spec && catalog.available && !catalog.specs.some(spec => spec.id === value.spec)) {
    report(
      "unknown_spec",
      `${id}: unknown spec "${value.spec}" (no .compozy/tasks/${value.spec}/)`
    );
  }
  if (new Set(value.start).size !== value.start.length) {
    report("invalid_flow", `${id}: start must not repeat node ids`);
  }
  const nodeIds = Object.keys(value.nodes);
  const declared = new Set(nodeIds);
  for (const node of value.start) {
    if (!declared.has(node)) {
      report("unknown_node", `start names node "${node}" that the flow does not declare`, node);
    }
  }
  const nodes = Object.entries(value.nodes).flatMap(([node, data]) => {
    const resolved = resolveNode(node, data, value.start, context.prototypes, report);
    return resolved ? [resolved] : [];
  });
  const edges = resolveEdges(id, value.edges, declared, report);
  for (const edge of duplicateEdges(edges)) {
    report("duplicate_edge", `edge ${edge.from} → ${edge.to} "${edge.on}" is declared twice`);
  }
  const order = bfsOrder(value.start, edges);
  for (const node of unreachable(nodeIds, order)) {
    report("unreachable", `node "${node}" is unreachable from start`, node);
  }
  for (const node of deadEnds(nodes, edges)) {
    report("dead_end", `node "${node}" is a dead end (no outgoing edge and not end)`, node);
  }
  const sorted = order.flatMap((node, index) => {
    const found = nodes.find(item => item.id === node);
    return found ? [{ ...found, step: index + 1 }] : [];
  });
  const owner = value.spec ?? null;
  const crossed = sorted.flatMap(node =>
    node.kind === "prototype" && node.spec ? [node.spec] : []
  );
  return {
    id,
    file,
    title: value.title,
    spec: owner,
    description: value.description ?? null,
    start: value.start,
    nodes: sorted,
    edges,
    specs: [...new Set([...(owner ? [owner] : []), ...crossed])],
  };
}

function parseFlow(path: string, id: string, report: Report): unknown {
  try {
    return declaration(path);
  } catch (error) {
    if (error instanceof LiteralError && error.reason === "duplicate_key") {
      const node = error.path.length === 1 && error.path[0] === "nodes" ? error.key : undefined;
      if (node) report("duplicate_node", `node "${node}" is declared twice`, node);
      else report("invalid_flow", `${id}: property "${error.key}" is declared twice`);
    } else {
      report("missing_flow", `${id}: missing literal "export const flow = defineFlow({...})"`);
    }
    return undefined;
  }
}

/** Flows by owner spec (catalog order, no spec last), then title. */
function sortFlows(flows: FlowEntry[], catalog: SpecCatalog) {
  const order = catalog.specs.map(spec => spec.id);
  const rank = (spec: string | null) => {
    const index = spec ? order.indexOf(spec) : -1;
    return index === -1 ? order.length + (spec ? 0 : 1) : index;
  };
  return flows.sort(
    (a, b) =>
      rank(a.spec) - rank(b.spec) ||
      a.title.localeCompare(b.title, "en", { numeric: true }) ||
      a.id.localeCompare(b.id)
  );
}

/**
 * Scans `web/src` and `web/gallery` for `*.flow.ts`: only `web/gallery/flows/<id>.flow.ts` is a
 * valid place, so a flow written elsewhere is reported instead of silently ignored.
 */
export function findFlows(
  webRoot: string,
  repoRoot: string,
  prototypes: readonly PrototypeEntry[],
  catalog: SpecCatalog
): { flows: FlowEntry[]; problems: FlowProblem[]; warnings: CoverageWarning[] } {
  const flows: FlowEntry[] = [];
  const problems: FlowProblem[] = [];
  const paths = [...files(join(webRoot, "src")), ...files(join(webRoot, "gallery"))].sort();
  for (const path of paths) {
    const local = relative(webRoot, path);
    const file = relative(repoRoot, path);
    const id = (path.split("/").at(-1) ?? "").slice(0, -flowSuffix.length);
    const before = problems.length;
    const report: Report = (code, message, node) =>
      problems.push({ code, flow: id, file, ...(node ? { node } : {}), message });
    if (!/^gallery\/flows\/[^/]+\.flow\.ts$/.test(local)) {
      report("flow_outside_root", `${file}: flows live in web/gallery/flows/<id>.flow.ts`);
      continue;
    }
    if (!validFlowId(id)) report("invalid_flow_id", `Invalid flow id "${id}" (${file})`);
    const value = parseFlow(path, id, report);
    if (value === undefined) continue;
    const flow = resolveFlow(id, file, value, { prototypes, catalog }, report);
    if (flow && problems.length === before) flows.push(flow);
  }
  return {
    flows: sortFlows(flows, catalog),
    problems,
    warnings: coverage(
      flows,
      prototypes.map(entry => ({
        slug: entry.slug,
        spec: entry.meta?.spec,
        states: entry.meta?.states.map(state => state.id) ?? [],
      }))
    ),
  };
}
