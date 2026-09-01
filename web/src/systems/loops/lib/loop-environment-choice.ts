import { LOOP_ENVIRONMENT_MODE_LABELS } from "./loop-node-schema-types";
import type { LoopEnvironmentMode, LoopEnvironmentSpec } from "../types";

export const INHERIT_ENVIRONMENT = "__inherit__";
export type LoopEnvironmentChoice = LoopEnvironmentMode | typeof INHERIT_ENVIRONMENT;

const WEB_EDITABLE_ENVIRONMENT_MODES = ["root", "worktree"] as const;

function isReadOnlyWebMode(
  mode: LoopEnvironmentChoice
): mode is Extract<LoopEnvironmentMode, "directory" | "per_run"> {
  return mode === "directory" || mode === "per_run";
}

export function loopEnvironmentItems(
  gitBacked: boolean,
  disabled: boolean,
  current: LoopEnvironmentChoice
) {
  const items = [
    { value: INHERIT_ENVIRONMENT as LoopEnvironmentChoice, label: "Inherit", disabled },
    ...WEB_EDITABLE_ENVIRONMENT_MODES.map(mode => ({
      value: mode as LoopEnvironmentChoice,
      label: LOOP_ENVIRONMENT_MODE_LABELS[mode],
      disabled: disabled || (!gitBacked && mode === "worktree"),
    })),
  ];
  if (isReadOnlyWebMode(current)) {
    items.push({
      value: current,
      label: `${LOOP_ENVIRONMENT_MODE_LABELS[current]} (read-only)`,
      disabled: true,
    });
  }
  return items;
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
