import { Empty } from "@agh/ui";
import { MousePointerClick } from "lucide-react";

import type { EditorEdge, EditorNode } from "../../lib/codec";
import { buildReferenceNamespace } from "../../lib/loop-references";
import type { FieldPath, FieldSpec } from "../../lib/loop-node-schema";
import type { LoopDefinition } from "../../types";
import { MonoTag } from "../mono-tag";
import { LoopEditorField } from "./loop-editor-field";

interface LoopEditorInspectorProps {
  node: EditorNode | null;
  fields: FieldSpec[];
  nodes: EditorNode[];
  edges: EditorEdge[];
  /** Increments only on a selection switch — the field container's remount key, so a
   *  rename (which mutates node.id) does not remount and drop focus. */
  selectionKey: number;
  definition: Pick<LoopDefinition, "inputs" | "start">;
  disabled: boolean;
  onChange: (path: FieldPath, value: unknown) => void;
}

/**
 * The set of nodes inside a fan-out branch body — every node reachable forward from a
 * fan-out node up to (not through) its paired `collect` barrier. `item`/`index` enter the
 * reference namespace only for these nodes (ADR-020 scopes them to the whole branch body,
 * which can be several hops deep), not merely a fan-out's direct successors.
 */
function fanoutBranchNodeIds(nodes: EditorNode[], edges: EditorEdge[]): Set<string> {
  const fanOutIds: string[] = [];
  const collectIds = new Set<string>();
  for (const node of nodes) {
    if (node.data.kind === "fan-out") {
      fanOutIds.push(node.id);
    } else if (node.data.kind === "collect") {
      collectIds.add(node.id);
    }
  }
  const adjacency = new Map<string, string[]>();
  for (const edge of edges) {
    const list = adjacency.get(edge.source) ?? [];
    list.push(edge.target);
    adjacency.set(edge.source, list);
  }
  const inBranch = new Set<string>();
  const queue = fanOutIds.flatMap(id => adjacency.get(id) ?? []);
  const seen = new Set(queue);
  while (queue.length > 0) {
    const current = queue.shift()!;
    inBranch.add(current);
    // A collect is the branch barrier: include nothing past it.
    if (collectIds.has(current)) continue;
    for (const next of adjacency.get(current) ?? []) {
      if (!seen.has(next)) {
        seen.add(next);
        queue.push(next);
      }
    }
  }
  collectIds.forEach(id => inBranch.delete(id));
  return inBranch;
}

/**
 * The per-node inspector (design §4.6). It renders the field set the DSL types produce
 * for the selected node's class/kind (loop-node-schema) — swapping entirely when the
 * selection changes — and routes every edit back through the codec. The field container is
 * keyed by `selectionKey` (which changes only on a selection switch, not a rename) so
 * per-field draft buffers (JSON editors) reset when switching nodes but a per-keystroke
 * rename never remounts the id input.
 */
export function LoopEditorInspector({
  node,
  fields,
  nodes,
  edges,
  selectionKey,
  definition,
  disabled,
  onChange,
}: LoopEditorInspectorProps) {
  const branchIds = fanoutBranchNodeIds(nodes, edges);
  const suggestions = node
    ? buildReferenceNamespace(definition, {
        nodes,
        edges,
        currentNodeId: node.id,
        inFanoutBranch: branchIds.has(node.id),
      })
    : [];

  if (!node) {
    return (
      <section
        className="flex h-full min-h-0 flex-col items-center justify-center bg-canvas px-6 py-10"
        data-testid="loop-editor-inspector-empty"
      >
        <Empty
          className="max-w-[16rem]"
          icon={MousePointerClick}
          title="No node selected"
          description="Select a node on the canvas to edit its fields."
        />
      </section>
    );
  }

  const raw = node.data.raw;
  return (
    <section
      className="flex h-full min-h-0 flex-col overflow-y-auto bg-canvas"
      data-testid="loop-editor-inspector"
    >
      <div className="sticky top-0 z-10 border-b border-line-soft bg-canvas px-4 py-3.5">
        <div className="flex items-center gap-2">
          <span
            className="text-item-title font-medium text-fg-strong"
            data-testid="loop-inspector-name"
          >
            {String(raw.id)}
          </span>
          <MonoTag className="rounded-xs bg-badge-fill px-1.5 py-0.5 text-pill-group-badge text-subtle">
            {node.data.nodeClass ?? "node"}
          </MonoTag>
        </div>
        <p className="mt-1.5 flex items-center gap-2 font-mono text-mono-id text-subtle">
          <span>{node.data.kind || "—"}</span>
          <span aria-hidden="true" className="size-0.5 rounded-full bg-faint" />
          <span>id: {String(raw.id)}</span>
        </p>
      </div>
      <div key={selectionKey} className="flex flex-col gap-3.5 px-4 py-4">
        {fields.map(field => (
          <LoopEditorField
            key={field.key}
            field={field}
            raw={raw}
            suggestions={suggestions}
            disabled={disabled}
            onChange={onChange}
          />
        ))}
      </div>
    </section>
  );
}
