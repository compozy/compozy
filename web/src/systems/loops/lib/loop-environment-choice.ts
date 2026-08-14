import { LOOP_ENVIRONMENT_MODE_LABELS, LOOP_ENVIRONMENT_MODES } from "./loop-node-schema-types";
import type { LoopEnvironmentMode, LoopEnvironmentSpec } from "../types";

export const INHERIT_ENVIRONMENT = "__inherit__";
export type LoopEnvironmentChoice = LoopEnvironmentMode | typeof INHERIT_ENVIRONMENT;

export function loopEnvironmentItems(gitBacked: boolean, disabled: boolean) {
  return [
    { value: INHERIT_ENVIRONMENT as LoopEnvironmentChoice, label: "Inherit", disabled },
    ...LOOP_ENVIRONMENT_MODES.map(mode => ({
      value: mode as LoopEnvironmentChoice,
      label: LOOP_ENVIRONMENT_MODE_LABELS[mode],
      disabled: disabled || (!gitBacked && (mode === "worktree" || mode === "per_run")),
    })),
  ];
}

export function environmentSpecForChoice(
  choice: LoopEnvironmentChoice,
  current: LoopEnvironmentSpec | null
): LoopEnvironmentSpec | null {
  if (choice === INHERIT_ENVIRONMENT) return null;
  if (choice === "worktree") {
    return { mode: choice, worktree_ref: current?.worktree_ref ?? "" };
  }
  if (choice === "directory") {
    return { mode: choice, directory: current?.directory ?? "" };
  }
  return { mode: choice };
}
