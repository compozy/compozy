import type { EditorEdge, EditorNode } from "./codec";
import type { LoopDefinition } from "../types";

/**
 * The single reference namespace both the `{{ }}` template surface and the CEL
 * condition surface resolve against (ADR-020). The editor's autocomplete offers
 * exactly these paths so an author never hand-types an unvalidated reference; the
 * shared linter still owns whether a path resolves (`unknown_reference`). This module
 * only enumerates the reachable shape from the current definition — it does not judge.
 */

export type LoopReferenceKind =
  | "input"
  | "node-output"
  | "node-status"
  | "fanout"
  | "trigger"
  | "generation";

export interface LoopReferenceSuggestion {
  /** The dotted path as it appears after `{{ .` or in a CEL identifier. */
  path: string;
  kind: LoopReferenceKind;
  detail: string;
}

function inputNames(definition: Pick<LoopDefinition, "inputs">): string[] {
  const inputs = definition.inputs;
  return inputs ? Object.keys(inputs) : [];
}

/** Topological ancestors of `nodeId` (nodes reachable by walking edges backwards). */
function upstreamNodeIds(nodeId: string, edges: readonly EditorEdge[]): Set<string> {
  const incoming = new Map<string, string[]>();
  for (const edge of edges) {
    const list = incoming.get(edge.target) ?? [];
    list.push(edge.source);
    incoming.set(edge.target, list);
  }
  const ancestors = new Set<string>();
  const queue = [...(incoming.get(nodeId) ?? [])];
  while (queue.length > 0) {
    const current = queue.shift()!;
    if (ancestors.has(current)) continue;
    ancestors.add(current);
    for (const parent of incoming.get(current) ?? []) queue.push(parent);
  }
  return ancestors;
}

/**
 * Enumerates the reference paths reachable from a node. When `edges` are supplied, node
 * outputs are restricted to the current node's topological ancestors — the only nodes it
 * can legally reference (a forward reference is `unknown_reference` at publish). Without
 * edges, all-but-current nodes are offered. `item`/`index` appear only inside a fan-out
 * branch body; `trigger.*` only when a start binding can deliver a payload. Ordering is
 * stable for deterministic autocomplete. The linter still owns final resolution.
 */
export function buildReferenceNamespace(
  definition: Pick<LoopDefinition, "inputs" | "start">,
  options: {
    nodes: readonly EditorNode[];
    edges?: readonly EditorEdge[];
    currentNodeId?: string;
    inFanoutBranch?: boolean;
  } = { nodes: [] }
): LoopReferenceSuggestion[] {
  const suggestions: LoopReferenceSuggestion[] = [];
  for (const name of inputNames(definition)) {
    suggestions.push({ path: `inputs.${name}`, kind: "input", detail: "declared loop input" });
  }
  const ancestors =
    options.edges && options.currentNodeId
      ? upstreamNodeIds(options.currentNodeId, options.edges)
      : null;
  for (const node of options.nodes) {
    if (node.id === options.currentNodeId) continue;
    // With topology, offer only ancestors (the nodes this one can legally reference).
    if (ancestors && !ancestors.has(node.id)) continue;
    suggestions.push({
      path: `nodes.${node.id}.output`,
      kind: "node-output",
      detail: `${node.data.kind || node.data.nodeClass || "node"} output`,
    });
    suggestions.push({
      path: `nodes.${node.id}.status`,
      kind: "node-status",
      detail: "per-node terminal status",
    });
  }
  if (options.inFanoutBranch) {
    suggestions.push({ path: "item", kind: "fanout", detail: "the fanned unit (fan-out branch)" });
    suggestions.push({
      path: "index",
      kind: "fanout",
      detail: "the fanned index (fan-out branch)",
    });
  }
  // `trigger.<path>` is the activation payload — trigger/webhook starts carry one
  // (ADR-020/023). Payload-less direct starts (manual/cli/uds) do not.
  const TRIGGER_START_KINDS = new Set(["trigger", "webhook", "schedule", "network"]);
  const hasTriggerStart = (definition.start ?? []).some(binding =>
    TRIGGER_START_KINDS.has(binding.kind)
  );
  if (hasTriggerStart) {
    suggestions.push({ path: "trigger", kind: "trigger", detail: "the activation payload" });
  }
  suggestions.push({ path: "generation", kind: "generation", detail: "current generation number" });
  return suggestions;
}

/**
 * Filters the namespace for a `{{ .`-triggered or CEL query. `query` is the partial
 * path already typed after the trigger (e.g. `inp`); an empty query returns all.
 */
export function filterReferences(
  suggestions: readonly LoopReferenceSuggestion[],
  query: string
): LoopReferenceSuggestion[] {
  const needle = query.trim().toLowerCase();
  if (needle === "") return [...suggestions];
  return suggestions.filter(suggestion => suggestion.path.toLowerCase().includes(needle));
}

/** Namespace roots that trigger the CEL identifier picker (ADR-020). */
const CEL_ROOTS = ["inputs", "nodes", "item", "index", "trigger", "generation"];

function activeTemplateQuery(value: string, caret: number): string | null {
  const upToCaret = value.slice(0, caret);
  const open = upToCaret.lastIndexOf("{{");
  if (open === -1) return null;
  const close = upToCaret.lastIndexOf("}}");
  if (close > open) return null;
  const fragment = upToCaret.slice(open + 2).replace(/^\s*\.?/, "");
  // A space after the reference path means the author moved on to a function/pipe.
  if (/\s/.test(fragment)) return null;
  return fragment;
}

function activeCelQuery(value: string, caret: number): string | null {
  const match = value.slice(0, caret).match(/[A-Za-z_][A-Za-z0-9_.]*$/);
  if (!match) return null;
  const token = match[0];
  const firstSegment = token.split(".")[0];
  // Only offer the picker on a token that could be a namespace path, not on a CEL
  // function/operator identifier — so `size(` or `matches` never pops the list.
  const rootish = CEL_ROOTS.some(
    root => root.startsWith(firstSegment) || firstSegment.startsWith(root)
  );
  return rootish ? token : null;
}

/**
 * The active reference fragment under the caret, or `null` when not in a reference.
 * `template` mode reads a `{{ .partial` fragment; `cel` mode reads a trailing namespace
 * identifier (`nodes.`, `inputs.`, `item`, …) since CEL conditions have no braces.
 */
export function activeReferenceQuery(
  value: string,
  caret: number,
  mode: "template" | "cel" = "template"
): string | null {
  return mode === "cel" ? activeCelQuery(value, caret) : activeTemplateQuery(value, caret);
}
