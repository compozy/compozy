import type { LoopEffectiveConfig, RunLoopRequest } from "../types";

export type LoopEffectiveConfigView = Pick<
  LoopEffectiveConfig,
  | "human_gate_enabled"
  | "reattempt_strategy"
  | "iteration_cap"
  | "budget_tokens"
  | "budget_wall_sec"
  | "budget_on_exceeded"
  | "no_progress_window"
  | "fan_out_width"
  | "gate_max_revisions"
>;

export type LoopRunConfigOverrides = NonNullable<RunLoopRequest["config_overrides"]>;

/**
 * Projects the current per-run request over the daemon-resolved per-Loop config.
 * The daemon is the only owner of definition, workspace, and built-in defaults.
 */
export function resolveLoopEffectiveConfig(
  effective: LoopEffectiveConfigView,
  runOverrides: LoopRunConfigOverrides | null = null
): LoopEffectiveConfigView {
  return {
    human_gate_enabled: runOverrides?.human_gate_enabled ?? effective.human_gate_enabled,
    reattempt_strategy: runOverrides?.reattempt_strategy ?? effective.reattempt_strategy,
    iteration_cap: runOverrides?.iteration_cap ?? effective.iteration_cap,
    budget_tokens: runOverrides?.budget_tokens ?? effective.budget_tokens,
    budget_wall_sec: runOverrides?.budget_wall_sec ?? effective.budget_wall_sec,
    budget_on_exceeded:
      (runOverrides?.budget_on_exceeded ?? effective.budget_on_exceeded) === "escalate"
        ? "escalate"
        : "halt",
    no_progress_window: runOverrides?.no_progress_window ?? effective.no_progress_window,
    fan_out_width: runOverrides?.fan_out_width ?? effective.fan_out_width,
    gate_max_revisions: runOverrides?.gate_max_revisions ?? effective.gate_max_revisions,
  };
}
