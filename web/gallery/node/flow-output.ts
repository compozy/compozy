import type { FlowEntry } from "../flow.ts";

/** Stable CLI rows: canvas-only fields (start/end flags) do not leak into the command contract. */
export function flowListRow(entry: FlowEntry) {
  return {
    id: entry.id,
    title: entry.title,
    spec: entry.spec,
    specs: entry.specs,
    nodes: entry.nodes.length,
    edges: entry.edges.length,
    file: entry.file,
  };
}

export function flowShowJson(entry: FlowEntry) {
  return {
    id: entry.id,
    title: entry.title,
    spec: entry.spec,
    description: entry.description,
    start: entry.start,
    nodes: entry.nodes.map(node =>
      node.kind === "prototype"
        ? {
            id: node.id,
            proto: node.proto,
            state: node.state,
            stateLabel: node.stateLabel,
            route: node.route,
            spec: node.spec,
            step: node.step,
            end: node.end,
          }
        : {
            id: node.id,
            external: node.external,
            label: node.label,
            url: node.url,
            step: node.step,
            end: node.end,
          }
    ),
    edges: entry.edges,
  };
}
