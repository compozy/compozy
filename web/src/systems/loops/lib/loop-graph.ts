import type { LoopDefinition, LoopDefinitionGraph } from "../types";

/** The closed node-class vocabulary from the OpenAPI contract; color never encodes class. */
export type LoopNodeClass = LoopDefinitionGraph["nodes"][number]["class"];

const LOOP_NODE_CLASSES = [
  "action",
  "control",
  "source",
] as const satisfies readonly LoopNodeClass[];
const LOOP_NODE_CLASS_SET = new Set<LoopNodeClass>(LOOP_NODE_CLASSES);

/**
 * A read-only projection of one graph node for the Loop-detail body view.
 *
 * The daemon serializes a full `dsl.Node` (id / class / kind + the fan-out knobs,
 * gate verdict policy, etc.), but the generated OpenAPI types the graph as
 * `Record<string, never>[]` because the Go `Node` uses an inline `Extra` map the
 * codegen can't introspect. This projection is the hand-authored counterpart to
 * the generated OpenAPI graph shape and is read through runtime guards, so a
 * malformed node degrades gracefully instead of throwing.
 */
export interface LoopGraphNode {
  id: string;
  nodeClass: LoopNodeClass | null;
  kind: string;
  /** Fan-out knobs (present on `control` `fan-out` nodes). */
  batchSize?: number;
  maxParallel?: number;
  maxFanOut?: number;
  /** Gate metadata (present on `control` `gate` nodes). */
  isGate: boolean;
  /** The watch spec map (`poll` / `settle` / …) when this node watches a source. */
  watch?: Record<string, unknown>;
  /** Declared watch-event subscription count (event-driven watch nodes). */
  eventsCount: number;
  /**
   * Authored `retry.max_attempts`. Only present when the definition declares it —
   * the run payload never carries an attempt ceiling, so "attempt 2 of 3" may be
   * phrased only for a node whose author wrote the 3.
   */
  retryMaxAttempts?: number;
  /** Authored `timeout` — the ceiling on a single attempt (Go duration string). */
  timeout?: string;
  /** Authored `deadline` — the ceiling on the whole node across attempts. */
  deadline?: string;
  /** Authored `expires.after` on a wait node — the point the wait gives up. */
  waitExpiresAfter?: string;

  strategy?: LoopGraphStrategy;

  bindAs?: string;
  indexAs?: string;

  routes: LoopGraphRoute[];
  defaultRoute?: string;

  onEvalError?: string;

  askPrompt?: string;
  hasAskExpect: boolean;

  review?: LoopGraphReview;
}

export interface LoopGraphStrategy {
  kind: string;

  threshold?: string;

  missing?: string;
}

export interface LoopGraphRoute {
  when: string;
  to: string;
}

export interface LoopGraphReview {
  decisions: string[];
  prompt?: string;
  onRejectRoute?: string;
  expiresAfter?: string;
}

export interface LoopGraphEdge {
  from: string;
  to: string;
}

export interface LoopGraph {
  nodes: LoopGraphNode[];
  edges: LoopGraphEdge[];
}

function asRecord(value: unknown): Record<string, unknown> | null {
  return typeof value === "object" && value !== null ? (value as Record<string, unknown>) : null;
}

function asString(value: unknown): string {
  return typeof value === "string" ? value : "";
}

function asOptionalNumber(value: unknown): number | undefined {
  return typeof value === "number" && Number.isFinite(value) ? value : undefined;
}

function asOptionalString(value: unknown): string | undefined {
  if (typeof value !== "string") return undefined;
  const trimmed = value.trim();
  return trimmed === "" ? undefined : trimmed;
}

export function toLoopNodeClass(value: unknown): LoopNodeClass | null {
  return typeof value === "string" && LOOP_NODE_CLASS_SET.has(value as LoopNodeClass)
    ? (value as LoopNodeClass)
    : null;
}

function projectStrategy(raw: unknown): LoopGraphStrategy | undefined {
  if (typeof raw === "string") {
    const kind = raw.trim();
    return kind === "" ? undefined : { kind };
  }
  const record = asRecord(raw);
  if (!record) return undefined;
  const kind = asString(record.kind);
  if (kind === "") return undefined;
  const count = asOptionalNumber(asRecord(record.threshold)?.count);
  const threshold =
    asOptionalString(record.threshold) ?? (count === undefined ? undefined : String(count));
  return { kind, threshold, missing: asOptionalString(record.missing) };
}

function projectRoutes(raw: unknown): LoopGraphRoute[] {
  if (!Array.isArray(raw)) return [];
  return raw
    .map(entry => {
      const record = asRecord(entry);
      if (!record) return null;
      const to = asString(record.to);
      if (to === "") return null;
      return { when: asString(record.when), to };
    })
    .filter((route): route is LoopGraphRoute => route !== null);
}

function projectReview(raw: unknown): LoopGraphReview | undefined {
  const record = asRecord(raw);
  if (!record) return undefined;
  const decisions = Array.isArray(record.decisions)
    ? record.decisions.filter((entry): entry is string => typeof entry === "string")
    : [];
  return {
    decisions,
    prompt: asOptionalString(record.prompt),
    onRejectRoute: asOptionalString(asRecord(record.on_reject)?.route),
    expiresAfter: asOptionalString(asRecord(record.expires)?.after),
  };
}

function projectNode(raw: unknown): LoopGraphNode | null {
  const record = asRecord(raw);
  if (!record) return null;
  const id = asString(record.id);
  if (id === "") return null;
  const kind = asString(record.kind);
  const params = asRecord(record.params);
  return {
    id,
    nodeClass: toLoopNodeClass(record.class),
    kind,
    batchSize: asOptionalNumber(record.batch_size),
    maxParallel: asOptionalNumber(record.max_parallel),
    maxFanOut: asOptionalNumber(record.max_fan_out),
    isGate: kind === "gate" || Boolean(record.criteria) || Boolean(record.verdict_policy),
    watch: asRecord(record.watch) ?? undefined,
    eventsCount: Array.isArray(record.events) ? record.events.length : 0,
    retryMaxAttempts: asOptionalNumber(asRecord(record.retry)?.max_attempts),
    timeout: asOptionalString(record.timeout),
    deadline: asOptionalString(record.deadline),
    waitExpiresAfter: asOptionalString(asRecord(params?.expires)?.after),
    strategy: projectStrategy(record.strategy),
    bindAs: asOptionalString(record.bind_as),
    indexAs: asOptionalString(record.index_as),
    routes: projectRoutes(record.routes),
    defaultRoute: asOptionalString(record.default),
    onEvalError: asOptionalString(record.on_eval_error),
    askPrompt: asOptionalString(params?.prompt),
    hasAskExpect: asRecord(params?.expect) !== null,
    review: projectReview(record.review),
  };
}

/** The authored attempt ceiling for one node, or undefined when none is declared. */
export function nodeRetryMaxAttempts(graph: LoopGraph | null, nodeId: string): number | undefined {
  return graph?.nodes.find(node => node.id === nodeId)?.retryMaxAttempts;
}

function projectEdge(raw: unknown): LoopGraphEdge | null {
  const record = asRecord(raw);
  if (!record) return null;
  const from = asString(record.from);
  const to = asString(record.to);
  if (from === "" || to === "") return null;
  return { from, to };
}

/**
 * Projects a Loop definition's graph into the typed read-only shape the detail
 * body view renders. Unreadable nodes/edges are dropped rather than surfaced as
 * empty rows.
 */
export function readLoopGraph(definition: Pick<LoopDefinition, "graph">): LoopGraph {
  const graph = asRecord(definition.graph);
  const rawNodes = Array.isArray(graph?.nodes) ? graph.nodes : [];
  const rawEdges = Array.isArray(graph?.edges) ? graph.edges : [];
  return {
    nodes: rawNodes.map(projectNode).filter((node): node is LoopGraphNode => node !== null),
    edges: rawEdges.map(projectEdge).filter((edge): edge is LoopGraphEdge => edge !== null),
  };
}

/** The node-class label rendered as neutral mono text (never color-coded). */
export function nodeClassLabel(node: LoopGraphNode): string {
  if (node.nodeClass === "control" && node.kind === "fan-out") return "control · fan-out";
  if (node.nodeClass === "control" && node.kind === "route") return "control · route";
  if (node.nodeClass === "control" && node.kind === "ask") return "control · ask";
  if (node.nodeClass === "control" && node.isGate) return "control · gate";
  return node.nodeClass ?? "node";
}

/** The node this graph parks on between ticks; null when the loop has no watch source. */
export function findWatchNode(graph: LoopGraph): LoopGraphNode | null {
  return (
    graph.nodes.find(node => node.watch !== undefined || node.eventsCount > 0) ??
    graph.nodes.find(node => node.kind === "watch-source") ??
    null
  );
}

/** Ids of `goal` nodes — the only nodes whose story rows expose a Turns disclosure. */
export function goalNodeIds(graph: LoopGraph): Set<string> {
  const ids = new Set<string>();
  for (const node of graph.nodes) {
    if (node.kind === "goal") ids.add(node.id);
  }
  return ids;
}

/**
 * Node ids in topological order (Kahn), used to phrase "what happens next" from
 * the pinned graph. Nodes caught in a cycle (invalid graphs degrade, never throw)
 * append in declaration order.
 */
export function topoOrder(graph: LoopGraph): string[] {
  const indegree = new Map<string, number>(graph.nodes.map(node => [node.id, 0]));
  const downstream = new Map<string, string[]>();
  for (const edge of graph.edges) {
    if (!indegree.has(edge.from) || !indegree.has(edge.to)) continue;
    indegree.set(edge.to, (indegree.get(edge.to) ?? 0) + 1);
    downstream.set(edge.from, [...(downstream.get(edge.from) ?? []), edge.to]);
  }
  const queue: string[] = [];
  for (const node of graph.nodes) {
    if (indegree.get(node.id) === 0) queue.push(node.id);
  }
  const ordered: string[] = [];
  const seen = new Set<string>();
  for (let head = 0; head < queue.length; head += 1) {
    const id = queue[head];
    if (id === undefined || seen.has(id)) continue;
    seen.add(id);
    ordered.push(id);
    for (const next of downstream.get(id) ?? []) {
      const remaining = (indegree.get(next) ?? 0) - 1;
      indegree.set(next, remaining);
      if (remaining <= 0) queue.push(next);
    }
  }
  for (const node of graph.nodes) {
    if (!seen.has(node.id)) ordered.push(node.id);
  }
  return ordered;
}

export function strategySummary(node: LoopGraphNode): string | null {
  if (!node.strategy) return null;
  const parts = [node.strategy.kind];
  if (node.strategy.threshold) parts.push(node.strategy.threshold);
  if (node.strategy.missing) parts.push(`missing ${node.strategy.missing}`);
  return parts.join(" ");
}

export function iterationNamesSummary(node: LoopGraphNode): string | null {
  const parts: string[] = [];
  if (node.bindAs) parts.push(`as ${node.bindAs}`);
  if (node.indexAs) parts.push(`index ${node.indexAs}`);
  return parts.length === 0 ? null : parts.join(" · ");
}

export function routeSummary(node: LoopGraphNode): string | null {
  if (node.kind !== "route" && node.routes.length === 0) return null;
  const count = node.routes.length;
  const parts = [count === 1 ? "1 route" : `${count} routes`];

  parts.push(node.defaultRoute ? `default ${node.defaultRoute}` : "no default");
  return parts.join(" · ");
}

/** The fan-out summary line (`batch 1 · seq · ≤64`) shown under a fan-out node. */
export function fanOutSummary(node: LoopGraphNode): string | null {
  const strategy = strategySummary(node);
  const iteration = iterationNamesSummary(node);
  if (
    node.batchSize === undefined &&
    node.maxParallel === undefined &&
    node.maxFanOut === undefined &&
    strategy === null &&
    iteration === null
  ) {
    return null;
  }
  const parts: string[] = [];
  if (node.batchSize !== undefined) parts.push(`batch ${node.batchSize}`);
  if (node.maxParallel !== undefined)
    parts.push(node.maxParallel <= 1 ? "seq" : `×${node.maxParallel}`);
  if (node.maxFanOut !== undefined) parts.push(`≤${node.maxFanOut}`);
  if (strategy) parts.push(strategy);
  if (iteration) parts.push(iteration);
  return parts.join(" · ");
}
