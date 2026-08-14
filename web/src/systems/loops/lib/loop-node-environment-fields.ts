import type { LoopEnvironmentMode, LoopEnvironmentSpec } from "../types";
import type { RawLoopNode } from "./codec";
import { getAtPath, type NodeFieldEdit } from "./loop-editor-draft";
import {
  LOOP_ENVIRONMENT_MODE_LABELS,
  LOOP_ENVIRONMENT_MODES,
  type EnvironmentFieldSpec,
  type FieldSpec,
} from "./loop-node-schema-types";

/**
 * The one execution-environment control on agent-executing nodes (`run-agent`,
 * `goal`). It replaced the retired `params.cwd` field outright — a definition
 * still carrying `cwd` fails validation with `environment_cwd_removed`, and
 * there is never a second directory field anywhere on a node.
 */

const ENVIRONMENT_BASE_PATH = ["params", "environment"] as const;

/** Daemon `environment_cwd_removed` payload. Web authors nothing here. */
export const ENVIRONMENT_CWD_REMOVED_CODE = "environment_cwd_removed";
export const ENVIRONMENT_CWD_REMOVED_MESSAGE =
  "params.cwd is retired; use params.environment.directory";

function environmentPath(key: string): (string | number)[] {
  return [...ENVIRONMENT_BASE_PATH, key];
}

/** The mode this node declares, or `null` when it declares none (inherits the loop default). */
export function environmentMode(raw: RawLoopNode): LoopEnvironmentMode | null {
  const value = getAtPath(raw, environmentPath("mode"));
  if (typeof value !== "string") return null;
  return LOOP_ENVIRONMENT_MODES.find(mode => mode === value) ?? null;
}

export function hasRetiredCwd(raw: RawLoopNode): boolean {
  const params = raw.params;
  if (typeof params !== "object" || params === null) return false;
  return (params as Record<string, unknown>).cwd !== undefined;
}

/**
 * Writes the chosen mode and clears whichever companion key the daemon forbids
 * for it, in one edit. `root` and `per_run` carry neither companion; `worktree`
 * carries only `worktree_ref`; `directory` carries only `directory`. Choosing
 * any environment also drops retired `params.cwd`.
 */
export function environmentModeEdits(
  raw: RawLoopNode,
  mode: LoopEnvironmentMode | null
): NodeFieldEdit[] {
  const clearCwd: NodeFieldEdit = { path: ["params", "cwd"], value: undefined };
  if (mode === null) {
    // Clearing the whole descriptor is what "inherit the loop default" means —
    // an empty object would still be a declaration.
    return [{ path: [...ENVIRONMENT_BASE_PATH], value: undefined }, clearCwd];
  }
  const carriedRef = getAtPath(raw, environmentPath("worktree_ref"));
  const carriedDirectory = getAtPath(raw, environmentPath("directory"));
  return [
    { path: environmentPath("mode"), value: mode },
    {
      path: environmentPath("worktree_ref"),
      value: mode === "worktree" ? (carriedRef ?? "") : undefined,
    },
    {
      path: environmentPath("directory"),
      value: mode === "directory" ? (carriedDirectory ?? "") : undefined,
    },
    clearCwd,
  ];
}

/** Mode-specific inspector hint. Inherit has none. */
export function environmentHint(
  mode: LoopEnvironmentMode | null,
  loopDefault?: LoopEnvironmentSpec
): string | undefined {
  if (mode === "root") {
    return "Runs at the workspace root. Part of the session binding key.";
  }
  if (mode === "worktree") {
    if (!loopDefault) return undefined;
    return `Overrides the loop default (${LOOP_ENVIRONMENT_MODE_LABELS[loopDefault.mode]}) for this node.`;
  }
  if (mode === "per_run") {
    return "Name and branch are generated at run start — run/<task-slug>.";
  }
  if (mode === "directory") {
    return "Resolved when the run starts. Type {{ to autocomplete references.";
  }
  return undefined;
}

/** The Environment descriptor. Companion inputs belong to the environment renderer. */
export function environmentFields(raw: RawLoopNode, inheritLabel: string): FieldSpec[] {
  const mode = environmentMode(raw);
  const descriptor: EnvironmentFieldSpec = {
    type: "environment",
    key: "environment",
    label: "Environment",
    basePath: [...ENVIRONMENT_BASE_PATH],
    mode,
    inheritLabel,
  };
  return [descriptor];
}
