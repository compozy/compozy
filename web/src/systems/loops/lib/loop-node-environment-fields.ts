import type { LoopEnvironmentMode } from "../types";
import type { RawLoopNode } from "./codec";
import { getAtPath, type NodeFieldEdit } from "./loop-editor-draft";
import {
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

function environmentPath(key: string): (string | number)[] {
  return [...ENVIRONMENT_BASE_PATH, key];
}

/** The mode this node declares, or `null` when it declares none (inherits the loop default). */
export function environmentMode(raw: RawLoopNode): LoopEnvironmentMode | null {
  const value = getAtPath(raw, environmentPath("mode"));
  if (typeof value !== "string") return null;
  return LOOP_ENVIRONMENT_MODES.find(mode => mode === value) ?? null;
}

/**
 * Writes the chosen mode and clears whichever companion key the daemon forbids
 * for it, in one edit. `root` and `per_run` carry neither companion; `worktree`
 * carries only `worktree_ref`; `directory` carries only `directory`.
 */
export function environmentModeEdits(
  raw: RawLoopNode,
  mode: LoopEnvironmentMode | null
): NodeFieldEdit[] {
  if (mode === null) {
    // Clearing the whole descriptor is what "inherit the loop default" means —
    // an empty object would still be a declaration.
    return [{ path: [...ENVIRONMENT_BASE_PATH], value: undefined }];
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
  ];
}

function environmentModeInputs(mode: LoopEnvironmentMode | null): FieldSpec[] {
  if (mode === "worktree") {
    return [
      {
        type: "text",
        key: "environment_worktree_ref",
        label: "environment.worktree_ref",
        path: environmentPath("worktree_ref"),
        mono: true,
        required: true,
        reference: true,
        placeholder: "worktree name or id",
        hint: "One ready worktree in the loop's workspace. The run fails before session start if it cannot be resolved.",
      },
    ];
  }
  if (mode === "directory") {
    return [
      {
        type: "text",
        key: "environment_directory",
        label: "environment.directory",
        path: environmentPath("directory"),
        mono: true,
        required: true,
        reference: true,
        placeholder: "packages/api",
        hint: "A contained directory under the selected root, interpolable over the loop namespace.",
      },
    ];
  }
  return [];
}

/** The Environment descriptor plus whichever companion input its mode requires. */
export function environmentFields(raw: RawLoopNode, inheritLabel: string): FieldSpec[] {
  const mode = environmentMode(raw);
  const descriptor: EnvironmentFieldSpec = {
    type: "environment",
    key: "environment",
    label: "Environment",
    basePath: [...ENVIRONMENT_BASE_PATH],
    mode,
    inheritLabel,
    hint: "Where this node's agent runs. Leave it inheriting to follow the loop default.",
  };
  return [descriptor, ...environmentModeInputs(mode)];
}
