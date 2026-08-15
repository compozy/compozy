import type { LoopDefinitionGraph } from "../types";

// The daemon serializes a full dsl.Graph, but OpenAPI types nodes and edges as
// opaque records. Fixtures keep the runtime node shape and cast at this boundary.
export function fixtureGraph(
  nodes: ReadonlyArray<Record<string, unknown>>,
  edges: ReadonlyArray<{ from: string; to: string }>
): LoopDefinitionGraph {
  return { nodes, edges } as unknown as LoopDefinitionGraph;
}
