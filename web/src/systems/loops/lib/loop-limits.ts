import type { LoopEffectiveConfig, LoopRunRecord } from "../types";
import { UNBOUNDED_CAP } from "./loop-catalog";
import { resolveLoopEffectiveConfig } from "./loop-effective-config";

export const LOOP_CEILINGS = {
  iterationCap: 100,
  tokens: "20M",
  wallClock: "7d",
  noProgressWindow: 10,
  gateMaxRevisions: 10,
  /** Numeric forms of the token/wall-clock ceilings the run-form override grid clamps to. */
  tokensMax: 20_000_000,
  wallClockMinutes: 7 * 1_440,
} as const;

/** Compact token count (`500K`, `20M`, `2.4M`); `off` when budgets are unset. */
export function formatTokenBudget(tokens: number): string {
  if (tokens <= 0) return "off";
  if (tokens >= 1_000_000) {
    const millions = tokens / 1_000_000;
    return `${Number.isInteger(millions) ? millions : millions.toFixed(1)}M`;
  }
  if (tokens >= 1_000) {
    const thousands = tokens / 1_000;
    return `${Number.isInteger(thousands) ? thousands : thousands.toFixed(1)}K`;
  }
  return String(tokens);
}

/** Wall-clock budget as `off` (unset) or a compact `Nd`/`Nh`/`Nm`/`Ns` label. */
export function formatWallClock(seconds: number): string {
  if (seconds <= 0) return "off";
  const day = 86_400;
  const hour = 3_600;
  const minute = 60;
  if (seconds % day === 0) return `${seconds / day}d`;
  if (seconds >= day) return `${(seconds / day).toFixed(1)}d`;
  if (seconds >= hour) return `${Math.round(seconds / hour)}h`;
  if (seconds >= minute) return `${Math.round(seconds / minute)}m`;
  return `${seconds}s`;
}

/**
 * Plain labels for the per-loop limits, shared by the detail rail, the configure
 * dialog, and the run-form overrides so every surface names a limit the same way.
 * The config keys stay canonical in payloads.
 */
export const LOOP_LIMIT_LABELS = {
  iteration_cap: "Max rounds",
  budget_tokens: "Token budget",
  budget_wall_sec: "Time limit",
  budget_on_exceeded: "When a budget runs out",
  no_progress_window: "Rounds without progress",
  fan_out_width: "Parallel workers",
  gate_max_revisions: "Max revision requests",
} as const;

/** Display-only labels for `budget_on_exceeded`; the submitted value stays the enum. */
export const LOOP_BUDGET_POLICY_LABELS = {
  halt: "Stop the run",
  escalate: "Pause and ask me",
} as const satisfies Record<LoopRunRecord["budget_on_exceeded"], string>;

export interface LoopLimitRow {
  label: string;
  /** Per-loop default. */
  value: string;
  /** Hard ceiling or qualifier, read on demand (tooltip), never a column. */
  ceiling: string;
}

/** Limit rows on the Loop-detail right rail, each with its ceiling for the on-demand hint. */
export function buildLoopLimits(effectiveConfig: LoopEffectiveConfig): LoopLimitRow[] {
  const effective = resolveLoopEffectiveConfig(effectiveConfig);
  return [
    {
      label: LOOP_LIMIT_LABELS.iteration_cap,
      value: effective.iteration_cap === 0 ? UNBOUNDED_CAP : String(effective.iteration_cap),
      ceiling: `Up to ${LOOP_CEILINGS.iterationCap}`,
    },
    {
      label: LOOP_LIMIT_LABELS.budget_tokens,
      value: formatTokenBudget(effective.budget_tokens),
      ceiling: `Up to ${LOOP_CEILINGS.tokens}`,
    },
    {
      label: LOOP_LIMIT_LABELS.budget_wall_sec,
      value: formatWallClock(effective.budget_wall_sec),
      ceiling: `Up to ${LOOP_CEILINGS.wallClock}`,
    },
    {
      label: LOOP_LIMIT_LABELS.budget_on_exceeded,
      value: LOOP_BUDGET_POLICY_LABELS[effective.budget_on_exceeded],
      ceiling: "",
    },
    {
      label: LOOP_LIMIT_LABELS.no_progress_window,
      value: String(effective.no_progress_window),
      ceiling: `Up to ${LOOP_CEILINGS.noProgressWindow}`,
    },
    {
      label: LOOP_LIMIT_LABELS.fan_out_width,
      value: effective.fan_out_width > 0 ? String(effective.fan_out_width) : "Auto",
      ceiling: "No fixed limit",
    },
    {
      label: LOOP_LIMIT_LABELS.gate_max_revisions,
      value: String(effective.gate_max_revisions),
      ceiling: `Up to ${LOOP_CEILINGS.gateMaxRevisions}`,
    },
  ];
}
