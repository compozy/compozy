import type { LoopAnnotation, LoopConfig, LoopEffectiveConfig } from "../types";

/**
 * How a loop is configured, and where its nodes sit on the canvas.
 *
 * A separate concern from the runs and the catalog: these answer "what are this
 * loop's settings" rather than "what has this loop done", and the two questions
 * are asked by different surfaces.
 */

export const loopConfigFixture: LoopConfig = {
  iteration_cap: 16,
  budget_tokens: 750_000,
  budget_wall_sec: null,
  budget_on_exceeded: "escalate",
  fan_out_width: 4,
  gate_max_revisions: 3,
  human_gate_enabled: true,
  no_progress_window: 3,
  reattempt_strategy: "failed_only",
  enabled_checks_json: null,
};

export const loopEffectiveConfigFixture: LoopEffectiveConfig = {
  budget_on_exceeded: "escalate",
  budget_tokens: 750_000,
  budget_wall_sec: 0,
  enabled_checks_json: {},
  // Resolved Loop environment: no node or Loop override, so runs execute at the
  // workspace root.
  environment: { mode: "root" },
  fan_out_width: 4,
  gate_max_revisions: 3,
  human_gate_enabled: true,
  iteration_cap: 16,
  runtime_defaults: {
    worker: { provider: "openai", model: "gpt-5.4" },
    judge: { provider: "anthropic", model: "claude-sonnet-4" },
  },
  runtime_rules: [
    {
      match: { type: "implementation" },
      runtime: { reasoning: "high" },
    },
  ],
  no_progress_window: 3,
  reattempt_strategy: "failed_only",
  request_expire_after: "",
};

export const loopAnnotationsFixture: LoopAnnotation[] = [
  { node_id: "load_tasks", x: 120, y: 80 },
  { node_id: "implement", x: 360, y: 80 },
];
