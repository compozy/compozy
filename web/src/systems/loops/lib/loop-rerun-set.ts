import type { LoopGraph } from "./loop-graph";

export interface LoopRerunSet {
  fromNode: string;

  rerunNodes: string[];

  carriedNodes: string[];
}

export function buildRerunSet(graph: LoopGraph | null, fromNode: string): LoopRerunSet {
  if (!graph || fromNode === "") {
    return { fromNode, rerunNodes: fromNode === "" ? [] : [fromNode], carriedNodes: [] };
  }
  const downstream = new Map<string, string[]>();
  for (const edge of graph.edges) {
    downstream.set(edge.from, [...(downstream.get(edge.from) ?? []), edge.to]);
  }
  const rerun: string[] = [];
  const seen = new Set<string>();
  const queue = [fromNode];
  for (let head = 0; head < queue.length; head += 1) {
    const id = queue[head];
    if (id === undefined || seen.has(id)) continue;
    seen.add(id);
    rerun.push(id);
    for (const next of downstream.get(id) ?? []) queue.push(next);
  }
  const carried: string[] = [];
  for (const node of graph.nodes) {
    if (!seen.has(node.id)) carried.push(node.id);
  }
  return {
    fromNode,
    rerunNodes: rerun,
    carriedNodes: carried,
  };
}
