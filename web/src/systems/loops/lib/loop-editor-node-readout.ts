import type { EditorNode } from "./codec";
import { LOOP_ENVIRONMENT_MODE_LABELS } from "./loop-node-schema-types";
import type { LoopEnvironmentMode, LoopEnvironmentSpec } from "../types";

type RawEditorNode = EditorNode["data"]["raw"];

const KIND_IN_LABEL = new Set(["gate", "fan-out", "collect", "branch", "sub-loop", "route", "ask"]);

/** The editor card's class pill: the node class, suffixed by its kind for structural kinds. */
export function editorNodeClassLabel(nodeClass: string | null, kind: string): string {
  const base = nodeClass ?? "node";
  return KIND_IN_LABEL.has(kind) ? `${base} · ${kind}` : base;
}

function num(value: unknown): number | undefined {
  return typeof value === "number" && Number.isFinite(value) ? value : undefined;
}

/** The fan-out knob chips (batch / parallelism / author bound) — the author-time truthful
 *  "branch" summary; runtime fanned items only exist on the run page, never here. */
export function fanOutChips(raw: RawEditorNode): string[] {
  if (raw.kind !== "fan-out") return [];
  const chips: string[] = [];
  const batch = num(raw.batch_size);
  const parallel = num(raw.max_parallel);
  const authorBound = num(raw.max_fan_out);
  if (batch !== undefined) chips.push(`batch ${batch}`);
  if (parallel !== undefined) chips.push(parallel <= 1 ? "seq" : `×${parallel}`);
  if (authorBound !== undefined) chips.push(`≤${authorBound}`);
  return chips;
}

/**
 * The node's own environment declaration, as a short readout.
 *
 * Only what the node itself declares is rendered: the loop-level default is not
 * part of the canvas node model, so naming it here would be a guess rather than
 * a readback.
 */
export interface EnvironmentReadout {
  label: string;
  inherited: boolean;
}

function specLabel(spec: {
  mode?: unknown;
  worktree_ref?: unknown;
  directory?: unknown;
}): string | undefined {
  if (typeof spec.mode !== "string") return undefined;
  if (spec.mode === "worktree" && typeof spec.worktree_ref === "string" && spec.worktree_ref) {
    return `${LOOP_ENVIRONMENT_MODE_LABELS.worktree} · ${spec.worktree_ref}`;
  }
  if (spec.mode === "directory" && typeof spec.directory === "string" && spec.directory) {
    return `${LOOP_ENVIRONMENT_MODE_LABELS.directory} · ${spec.directory}`;
  }
  if (spec.mode in LOOP_ENVIRONMENT_MODE_LABELS) {
    return LOOP_ENVIRONMENT_MODE_LABELS[spec.mode as LoopEnvironmentMode];
  }
  return undefined;
}

export function environmentReadout(
  raw: RawEditorNode,
  loopDefault?: LoopEnvironmentSpec
): EnvironmentReadout | undefined {
  if (raw.kind !== "run-agent" && raw.kind !== "goal") return undefined;
  const params = raw.params;
  if (typeof params === "object" && params !== null) {
    const environment = (params as Record<string, unknown>).environment;
    if (typeof environment === "object" && environment !== null) {
      const label = specLabel(environment as Record<string, unknown>);
      if (label) return { label, inherited: false };
    }
  }
  const defaultLabel = loopDefault ? specLabel(loopDefault) : undefined;
  return defaultLabel ? { label: `${defaultLabel} · loop default`, inherited: true } : undefined;
}
