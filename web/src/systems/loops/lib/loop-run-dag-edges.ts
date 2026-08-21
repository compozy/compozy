import type { LoopGraph } from "./loop-graph";

/**
 * The run's real edges, after fan-out collapse.
 *
 * A lane drawn in topological order is not a chain, and connecting whatever
 * happens to sit next to each other would draw relationships the author never
 * wrote — two parallel reviewers would appear to feed one another. Every edge
 * here comes from `graph.edges`.
 *
 * Collapsing a fan-out hides its workers inside one band, so the authored path
 * that ran *through* a hidden worker has to survive the collapse: an edge whose
 * endpoint is hidden re-points at the band that now stands for it. That is
 * compression along a real path, never a new relationship. An edge that becomes
 * a self-loop under collapse (band → its own worker) simply disappears, because
 * it is now internal to the band.
 */

/** How much of the run has flowed along an edge. */
export type LoopDagEdgeState = "live" | "taken" | "idle" | "not_taken";

export interface LoopDagEdge {
  key: string;
  from: string;
  to: string;
  state: LoopDagEdgeState;
  /** Rank of the source column; the gutter it crosses is derived from this. */
  fromRank: number;
  toRank: number;
}

/**
 * Longest-path layering. A node sits one column right of its furthest ancestor,
 * so an edge always points forward and every column can be drawn left to right.
 */
export function rankNodes(graph: LoopGraph, order: readonly string[]): Map<string, number> {
  const ranks = new Map<string, number>();
  for (const nodeId of order) ranks.set(nodeId, 0);
  // Predecessors indexed once. Rescanning every edge per node made the layout of
  // a wide authored graph quadratic in a projection that runs on every read.
  const incoming = new Map<string, string[]>();
  for (const edge of graph.edges) {
    const bucket = incoming.get(edge.to);
    if (bucket) bucket.push(edge.from);
    else incoming.set(edge.to, [edge.from]);
  }
  // `order` is topological, so every predecessor is already ranked on arrival.
  for (const nodeId of order) {
    for (const from of incoming.get(nodeId) ?? []) {
      const fromRank = ranks.get(from);
      if (fromRank === undefined) continue;
      ranks.set(nodeId, Math.max(ranks.get(nodeId) ?? 0, fromRank + 1));
    }
  }
  return ranks;
}

export interface ResolveEdgesInput {
  graph: LoopGraph;
  /** Node ids the lane actually draws. */
  visible: ReadonlySet<string>;
  /** Hidden node id -> the band that now stands for it. */
  collapsedInto: ReadonlyMap<string, string>;
  ranks: ReadonlyMap<string, number>;
  edgeState: (from: string, to: string) => LoopDagEdgeState;
}

function resolve(
  nodeId: string,
  visible: ReadonlySet<string>,
  collapsedInto: ReadonlyMap<string, string>
): string | null {
  if (visible.has(nodeId)) return nodeId;
  const band = collapsedInto.get(nodeId);
  return band && visible.has(band) ? band : null;
}

export function resolveDagEdges({
  graph,
  visible,
  collapsedInto,
  ranks,
  edgeState,
}: ResolveEdgesInput): LoopDagEdge[] {
  const seen = new Set<string>();
  const edges: LoopDagEdge[] = [];
  for (const edge of graph.edges) {
    const from = resolve(edge.from, visible, collapsedInto);
    const to = resolve(edge.to, visible, collapsedInto);
    if (!from || !to) continue;
    // Internal to a collapsed band once both ends fold into it.
    if (from === to) continue;
    const key = `${from}->${to}`;
    if (seen.has(key)) continue;
    seen.add(key);
    edges.push({
      key,
      from,
      to,
      state: edgeState(from, to),
      fromRank: ranks.get(from) ?? 0,
      toRank: ranks.get(to) ?? 0,
    });
  }
  return edges;
}

/**
 * Edges crossing the gutter to the right of `rank`.
 *
 * An edge that spans several columns really does pass every gutter between
 * them, so it is drawn at each — the alternative is a line that vanishes
 * mid-graph and reappears, which reads as two unrelated edges.
 */
export function edgesCrossingGutter(edges: readonly LoopDagEdge[], rank: number): LoopDagEdge[] {
  return edges.filter(edge => edge.fromRank <= rank && edge.toRank > rank);
}

/** Trouble and liveness outrank calm when several edges share one gutter. */
const GUTTER_PRECEDENCE: LoopDagEdgeState[] = ["live", "taken", "not_taken", "idle"];

export function gutterState(edges: readonly LoopDagEdge[]): LoopDagEdgeState {
  const present = new Set(edges.map(edge => edge.state));
  return GUTTER_PRECEDENCE.find(state => present.has(state)) ?? "idle";
}
