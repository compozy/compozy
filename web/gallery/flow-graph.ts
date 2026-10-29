/**
 * Pure graph rules of journey flows, shared by discovery (`node/flows.ts`), the CLI and the
 * gallery: journey order, unreachable nodes, dead ends, duplicate edges, which flows use each
 * prototype state and the coverage warnings.
 */
import type { CoverageWarning, FlowEntry, Occurrence, ResolvedEdge } from "./flow.ts";

/** Node ids in breadth-first order from the start nodes, following edges in file order. */
export function bfsOrder(start: readonly string[], edges: readonly ResolvedEdge[]): string[] {
  const order = [...new Set(start)];
  const seen = new Set(order);
  for (let index = 0; index < order.length; index += 1) {
    for (const edge of edges) {
      if (edge.from !== order[index] || seen.has(edge.to)) continue;
      seen.add(edge.to);
      order.push(edge.to);
    }
  }
  return order;
}

export function unreachable(nodeIds: readonly string[], order: readonly string[]): string[] {
  const reached = new Set(order);
  return nodeIds.filter(id => !reached.has(id));
}

/** Nodes with no outgoing edge that do not declare `end: true`. */
export function deadEnds(
  nodes: readonly { id: string; end: boolean }[],
  edges: readonly ResolvedEdge[]
): string[] {
  const sources = new Set(edges.map(edge => edge.from));
  return nodes.flatMap(node => (!node.end && !sources.has(node.id) ? [node.id] : []));
}

/** Nodes that declare `end: true` yet have an outgoing edge. */
export function endsWithEdges(
  nodes: readonly { id: string; end: boolean }[],
  edges: readonly ResolvedEdge[]
): string[] {
  const sources = new Set(edges.map(edge => edge.from));
  return nodes.flatMap(node => (node.end && sources.has(node.id) ? [node.id] : []));
}

/** `external` edges between two prototype nodes: the journey never leaves the app there. */
export function externalEdgesInsideApp(
  nodes: readonly { id: string; kind: "prototype" | "external" }[],
  edges: readonly ResolvedEdge[]
): ResolvedEdge[] {
  const kindOf = new Map(nodes.map(node => [node.id, node.kind]));
  return edges.filter(
    edge =>
      edge.kind === "external" &&
      kindOf.get(edge.from) === "prototype" &&
      kindOf.get(edge.to) === "prototype"
  );
}

/** Every repeat of an edge with the same source, target and trigger. */
export function duplicateEdges(edges: readonly ResolvedEdge[]): ResolvedEdge[] {
  const seen = new Set<string>();
  return edges.filter(({ from, to, on }) => {
    const key = JSON.stringify([from, to, on]);
    if (seen.has(key)) return true;
    seen.add(key);
    return false;
  });
}

/** `${slug}:${state}` → every flow node that shows that prototype state. */
export function membership(flows: readonly FlowEntry[]): Map<string, Occurrence[]> {
  const result = new Map<string, Occurrence[]>();
  for (const flow of flows) {
    for (const node of flow.nodes) {
      if (node.kind !== "prototype") continue;
      const key = `${node.proto}:${node.state}`;
      const occurrences = result.get(key) ?? [];
      occurrences.push({ flow: flow.id, node: node.id, step: node.step, steps: flow.nodes.length });
      result.set(key, occurrences);
    }
  }
  return result;
}

export function flowCount(occurrences: readonly Occurrence[]): number {
  return new Set(occurrences.map(occurrence => occurrence.flow)).size;
}

/** States of prototypes in specs some flow touches that no flow uses. */
export function coverage(
  flows: readonly FlowEntry[],
  prototypes: readonly { slug: string; spec?: string; states: readonly string[] }[]
): CoverageWarning[] {
  const touched = new Set(flows.flatMap(flow => flow.specs));
  const used = membership(flows);
  return prototypes.flatMap(prototype => {
    const { spec, slug } = prototype;
    if (!spec || !touched.has(spec)) return [];
    return prototype.states.flatMap(state =>
      used.has(`${slug}:${state}`) ? [] : [{ code: "uncovered_state" as const, spec, slug, state }]
    );
  });
}
