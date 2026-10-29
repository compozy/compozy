/**
 * The journey flow contract. A flow is a `web/gallery/flows/<id>.flow.ts` file:
 *
 *   export const flow = defineFlow({ title, spec?, description?, start, nodes, edges });
 *
 * Like prototype `meta`, the argument must be a pure literal: discovery reads it from the syntax
 * tree and never runs the module. A node is a prototype state (`proto` + `state`) or a place
 * outside the web app (`external`); an edge's `on` names the control a person uses, with the
 * same text as the real button or link.
 */

/** Places a CompozyOS journey can leave the web app for. */
export type ExternalService = "github" | "terminal" | "desktop" | "docs" | "other";
export type EdgeKind = "navigate" | "overlay" | "redirect" | "external";
export type PrototypeNode = { proto: string; state: string; end?: boolean };
export type ExternalNode = {
  external: ExternalService;
  /** Required for `other`; replaces the service name otherwise. */
  label?: string;
  /** Absolute `https://` URL the inspector opens. */
  url?: string;
  end?: boolean;
};
export type FlowNode = PrototypeNode | ExternalNode;
export type FlowEdge = readonly [
  from: string,
  to: string,
  options: { on: string; kind?: EdgeKind },
];
export type FlowDefinition = {
  title: string;
  /** Owner spec, the name of a `.compozy/tasks/<id>/` directory. */
  spec?: string;
  description?: string;
  start: readonly [string, ...string[]];
  nodes: Record<string, FlowNode>;
  edges: readonly FlowEdge[];
};

export function defineFlow<const Flow extends FlowDefinition>(flow: Flow): Flow {
  return flow;
}

export const externalLabels: Record<ExternalService, string> = {
  github: "GitHub",
  terminal: "Terminal",
  desktop: "Desktop app",
  docs: "CompozyOS docs",
  other: "",
};

export const edgeKinds: readonly EdgeKind[] = ["navigate", "overlay", "redirect", "external"];

type NodePosition = { id: string; end: boolean; start: boolean; step: number };
export type ResolvedNode = NodePosition &
  (
    | {
        kind: "prototype";
        proto: string;
        state: string;
        stateLabel: string;
        title: string;
        route: string | null;
        spec: string | null;
      }
    | { kind: "external"; external: ExternalService; label: string; url: string | null }
  );
export type ResolvedEdge = { from: string; to: string; on: string; kind: EdgeKind };
export type FlowEntry = {
  id: string;
  file: string;
  title: string;
  description: string | null;
  /** Owner spec; null for a flow that belongs to no spec. */
  spec: string | null;
  start: string[];
  /** Owner spec plus every spec its prototype nodes belong to. */
  specs: string[];
  /** In journey (breadth-first) order; `step` is the 1-based position. */
  nodes: ResolvedNode[];
  edges: ResolvedEdge[];
};
export type FlowProblemCode =
  | "flow_outside_root"
  | "invalid_flow_id"
  | "missing_flow"
  | "invalid_flow"
  | "unknown_spec"
  | "unknown_prototype"
  | "unknown_state"
  | "duplicate_node"
  | "invalid_node"
  | "unknown_node"
  | "duplicate_edge"
  | "unreachable"
  | "dead_end"
  | "end_with_edges"
  | "external_edge";
export type FlowProblem = {
  code: FlowProblemCode;
  flow: string | null;
  file: string;
  node?: string;
  message: string;
};
/** A prototype state of a spec some flow touches that no flow reaches. Never fails a check. */
export type CoverageWarning = {
  code: "uncovered_state";
  spec: string;
  slug: string;
  state: string;
};
export type Occurrence = { flow: string; node: string; step: number; steps: number };

/** Comments of flow `<id>` live under `flow-<id>`, a prefix prototype slugs cannot use. */
export const flowKeyPrefix = "flow-";
export const flowKey = (flow: string) => `${flowKeyPrefix}${flow}`;
export function flowOfKey(key: string) {
  return key.startsWith(flowKeyPrefix) ? key.slice(flowKeyPrefix.length) : null;
}
