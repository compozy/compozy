import type { LoopValidationIssue } from "../types";

/**
 * Renders a loop definition as the `agh.loop/v1` document the DSL view shows (ADR-015:
 * FS-as-truth). It is a READ render in v1 — the graph is the editing surface; structural
 * edits (including `start[]`) come from file/agent authoring (an editable DSL is a v1
 * deferral). Lines belonging to a node the linter flagged are marked `highlight`, and the
 * specific offending field line (when the code maps to a known field) is marked
 * `offending`, so the Graph/DSL toggle can surface exactly what publish rejects.
 */

export interface DslLine {
  text: string;
  /** The line belongs to a node the linter attached an error to. */
  highlight: boolean;
  /** The line is the specific field the lint code points at. */
  offending: boolean;
}

// Maps a deterministic lint code to the node field its message concerns, so the DSL
// view can underline the exact line (not just the node block). Extend as codes land.
const CODE_FIELD: Record<string, string> = {
  fan_out_ceiling_exceeded: "max_fan_out",
  verdict_policy_requires_judge: "verdict_policy",
  condition_not_bool: "condition",
  node_id_invalid: "id",
};

function isPlainObject(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function scalar(value: unknown): string {
  if (value === null) return "null";
  if (typeof value === "string") {
    // Quote strings that carry YAML-significant characters or template braces.
    return /[:#{}[\]]|^\s|\s$/.test(value) || value === "" ? JSON.stringify(value) : value;
  }
  // Backstop: a non-empty object/array should never reach here (emit recurses), but an
  // empty `{}`/`[]` and any stray object must serialize as valid YAML, never `[object Object]`.
  if (typeof value === "object") return JSON.stringify(value);
  return String(value);
}

function emitMultilineString(value: string, indent: number, lines: string[]): void {
  const pad = "  ".repeat(indent);
  for (const line of value.split("\n")) {
    lines.push(`${pad}${line}`);
  }
}

function emit(value: unknown, indent: number, lines: string[]): void {
  const pad = "  ".repeat(indent);
  if (isPlainObject(value)) {
    for (const [key, child] of Object.entries(value)) {
      // An empty object/array is a flow-style leaf (`key: {}` / `key: []`); only a
      // non-empty container gets a block header + recursion. Emitting `{}` for an empty
      // object closes the `[object Object]` hole (Channel post / Call tool / Gate seeds).
      if (isPlainObject(child) && Object.keys(child).length > 0) {
        lines.push(`${pad}${key}:`);
        emit(child, indent + 1, lines);
      } else if (Array.isArray(child) && child.length > 0) {
        lines.push(`${pad}${key}:`);
        emit(child, indent + 1, lines);
      } else if (typeof child === "string" && child.includes("\n")) {
        lines.push(`${pad}${key}: |`);
        emitMultilineString(child, indent + 1, lines);
      } else {
        lines.push(`${pad}${key}: ${scalar(child)}`);
      }
    }
    return;
  }
  if (Array.isArray(value)) {
    for (const item of value) {
      if (isPlainObject(item) || Array.isArray(item)) {
        // Intentional block-sequence style: the `-` sits on its own line and the item's keys
        // follow one indent deeper. This is valid YAML and keeps every item key at a single
        // predictable depth, which the offending-field highlight (NODE_KEY_INDENT) relies on;
        // an inline `- key: …` would put the first key at a different column.
        lines.push(`${pad}-`);
        emit(item, indent + 1, lines);
      } else if (typeof item === "string" && item.includes("\n")) {
        lines.push(`${pad}- |`);
        emitMultilineString(item, indent + 1, lines);
      } else {
        lines.push(`${pad}- ${scalar(item)}`);
      }
    }
    return;
  }
  lines.push(`${pad}${scalar(value)}`);
}

function rawString(value: unknown): string {
  return typeof value === "string" ? value : "";
}

/**
 * Serializes the definition to highlighted DSL lines. `byNode` supplies the linter's
 * per-node issues; a node's whole block highlights, and its offending field line (via
 * `CODE_FIELD`) is additionally marked.
 */
export function buildDslView(
  definition: object,
  byNode: ReadonlyMap<string, LoopValidationIssue[]>
): DslLine[] {
  const lines: DslLine[] = [];
  for (const [topKey, topValue] of Object.entries(definition)) {
    if (topKey !== "graph") {
      const block: string[] = [];
      emit({ [topKey]: topValue }, 0, block);
      for (const text of block) lines.push({ text, highlight: false, offending: false });
      continue;
    }
    emitGraph(topValue, byNode, lines);
  }
  return lines;
}

// The indent (in spaces) of a node's own keys when its block is emitted at `emit([node], 2)`.
const NODE_KEY_INDENT = 6;

function emitGraph(
  graph: unknown,
  byNode: ReadonlyMap<string, LoopValidationIssue[]>,
  lines: DslLine[]
): void {
  lines.push({ text: "graph:", highlight: false, offending: false });
  const nodes = isPlainObject(graph) && Array.isArray(graph.nodes) ? graph.nodes : [];
  const edges = isPlainObject(graph) && Array.isArray(graph.edges) ? graph.edges : [];
  lines.push({ text: "  nodes:", highlight: false, offending: false });
  for (const node of nodes) {
    const nodeId = isPlainObject(node) ? rawString(node.id) : "";
    const issues = nodeId ? byNode.get(nodeId) : undefined;
    const offendingFields = new Set<string>();
    for (const issue of issues ?? []) {
      const field = CODE_FIELD[issue.code];
      if (field) offendingFields.add(field);
    }
    const block: string[] = [];
    emit([node], 2, block);
    for (const text of block) {
      const trimmed = text.trim();
      const fieldKey = trimmed.replace(/^-\s*/, "").split(":")[0];
      // Only a node's own top-level keys (6-space indent under `emit([node], 2, …)`) can be
      // the offending field — never a same-named key nested inside params/map/etc.
      const leadingSpaces = text.length - text.trimStart().length;
      lines.push({
        text,
        highlight: Boolean(issues),
        offending: leadingSpaces === NODE_KEY_INDENT && offendingFields.has(fieldKey),
      });
    }
  }
  lines.push({ text: "  edges:", highlight: false, offending: false });
  const edgeBlock: string[] = [];
  emit(edges, 2, edgeBlock);
  for (const text of edgeBlock) lines.push({ text, highlight: false, offending: false });
}
