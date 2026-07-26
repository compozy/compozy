import { AlertTriangle } from "lucide-react";
import { Handle, Position, type NodeProps } from "@xyflow/react";

import { cn } from "@agh/ui";

import type { EditorNode } from "../../lib/codec";
import { MonoTag } from "../mono-tag";

const KIND_IN_LABEL = new Set(["gate", "fan-out", "collect", "branch", "sub-loop"]);

function classLabel(nodeClass: string | null, kind: string): string {
  const base = nodeClass ?? "node";
  return KIND_IN_LABEL.has(kind) ? `${base} · ${kind}` : base;
}

function num(value: unknown): number | undefined {
  return typeof value === "number" && Number.isFinite(value) ? value : undefined;
}

/** The fan-out knob chips (batch / parallelism / ceiling) — the author-time truthful
 *  "branch" summary; runtime fanned items only exist on the run page, never here. */
function fanOutChips(raw: EditorNode["data"]["raw"]): string[] {
  if (raw.kind !== "fan-out") return [];
  const chips: string[] = [];
  const batch = num(raw.batch_size);
  const parallel = num(raw.max_parallel);
  const ceiling = num(raw.max_fan_out);
  if (batch !== undefined) chips.push(`batch ${batch}`);
  if (parallel !== undefined) chips.push(parallel <= 1 ? "seq" : `×${parallel}`);
  if (ceiling !== undefined) chips.push(`≤${ceiling}`);
  return chips;
}

/**
 * One neutral node card on the DAG canvas (design §4.6). Color never encodes class:
 * selection is an accent ring, a lint error is a danger ring + corner badge, everything
 * else is a flat surface with mono class/kind labels. A single custom node type
 * dispatches on `data.nodeClass`/`kind` rather than a type per kind, so new kinds
 * render without new React Flow node types.
 */
export function LoopEditorNode({ data, selected }: NodeProps<EditorNode>) {
  const { raw, nodeClass, kind, hasError } = data;
  const chips = fanOutChips(raw);
  return (
    <div
      className={cn(
        "relative flex min-h-14 w-[132px] flex-col gap-1 rounded-md border bg-canvas-tint px-3 py-2.5 transition-colors",
        hasError ? "border-danger" : "border-line hover:border-line-strong",
        selected && !hasError && "border-accent-dim ring-1 ring-accent-dim",
        selected && hasError && "ring-1 ring-danger"
      )}
      data-testid="loop-editor-node"
      data-node-id={raw.id}
      data-node-error={hasError ? "true" : "false"}
    >
      <Handle type="target" position={Position.Left} className="!size-2 !border-line !bg-canvas" />
      <MonoTag
        className={cn(
          "text-pill-group-badge tracking-[0.07em]",
          selected ? "text-accent-strong" : "text-faint"
        )}
      >
        {classLabel(nodeClass, kind)}
      </MonoTag>
      <span
        className="min-w-0 truncate text-small-body font-medium leading-tight text-fg-strong"
        title={String(raw.id)}
      >
        {String(raw.id)}
      </span>
      <span
        className="min-w-0 truncate font-mono text-mono-id text-subtle"
        title={kind || undefined}
      >
        {kind || "—"}
      </span>
      {chips.length > 0 ? (
        <div className="mt-0.5 flex flex-wrap gap-1" data-testid="loop-editor-node-branches">
          {chips.map(chip => (
            <span
              key={chip}
              className="rounded-xs bg-badge-fill px-1 py-px font-mono text-pill-group-badge text-subtle"
            >
              {chip}
            </span>
          ))}
        </div>
      ) : null}
      {hasError ? (
        <span
          className="absolute -right-2 -top-2 grid size-4.5 place-items-center rounded-full border-2 border-canvas bg-danger text-accent-ink"
          data-testid="loop-editor-node-badge"
          title="This node has a validation error"
        >
          <AlertTriangle aria-hidden="true" className="size-2.5" />
        </span>
      ) : null}
      <Handle type="source" position={Position.Right} className="!size-2 !border-line !bg-canvas" />
    </div>
  );
}
