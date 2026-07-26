import type { LoopEffectiveConfig, LoopRunRecord } from "../types";
import { UNBOUNDED_CAP } from "./loop-catalog";
import { resolveLoopEffectiveConfig } from "./loop-effective-config";

/**
 * Hard daemon ceilings (LOOPS-DESIGN-SPEC §5.4 / ADR-012/017/022). These are
 * compile-time backstops, never editable in the UI; the right-hand value of every
 * limit row. The left value is the per-loop default read from the definition.
 */
export const LOOP_CEILINGS = {
  iterationCap: 100,
  tokens: "20M",
  wallClock: "7d",
  noProgressWindow: 10,
  fanOutBreadth: 64,
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

export interface LoopLimitRow {
  label: string;
  /** Per-loop default (left). */
  value: string;
  /** Daemon ceiling or qualifier (right); prefixed `/` renders as `/ ceiling`. */
  ceiling: string;
}

/**
 * The 8 limit rows shown on the Loop-detail right rail, pairing each per-loop
 * default from saved configuration with its daemon ceiling. Cost is display-only (no
 * enforced USD cap, §9.5.2); fan-out breadth is bounded by the loaded task count.
 */
/**
 * The single policy→outcome vocabulary for `budget_on_exceeded`, shared by the
 * limits rail (split value/ceiling) and the inspect "On limit" tile (one line).
 * The param is the closed daemon union (`"halt" | "escalate"`), so an unmapped
 * policy fails at compile time rather than silently folding to `halt`.
 */
export function onExceededPolicy(policy: LoopRunRecord["budget_on_exceeded"]): {
  policy: "escalate" | "halt";
  target: string;
} {
  return {
    policy,
    target: policy === "escalate" ? "→ needs-approval" : "→ exhausted",
  };
}

export function buildLoopLimits(effectiveConfig: LoopEffectiveConfig): LoopLimitRow[] {
  const effective = resolveLoopEffectiveConfig(effectiveConfig);
  const { policy: onExceeded, target: onExceededTarget } = onExceededPolicy(
    effective.budget_on_exceeded
  );
  return [
    {
      label: "Iteration cap",
      value: effective.iteration_cap === 0 ? UNBOUNDED_CAP : String(effective.iteration_cap),
      ceiling: `/ ${LOOP_CEILINGS.iterationCap}`,
    },
    {
      label: "Token budget",
      value: formatTokenBudget(effective.budget_tokens),
      ceiling: `/ ${LOOP_CEILINGS.tokens}`,
    },
    {
      label: "Wall clock",
      value: formatWallClock(effective.budget_wall_sec),
      ceiling: `/ ${LOOP_CEILINGS.wallClock}`,
    },
    { label: "On exceeded", value: onExceeded, ceiling: onExceededTarget },
    { label: "Cost (USD)", value: "—", ceiling: "display-only" },
    {
      label: "No-progress window",
      value: String(effective.no_progress_window),
      ceiling: `/ ${LOOP_CEILINGS.noProgressWindow}`,
    },
    {
      label: "Fan-out breadth",
      value: effective.fan_out_width > 0 ? String(effective.fan_out_width) : "≤ tasks",
      ceiling: `/ ${LOOP_CEILINGS.fanOutBreadth}`,
    },
    {
      label: "Gate max revisions",
      value: String(effective.gate_max_revisions),
      ceiling: `/ ${LOOP_CEILINGS.gateMaxRevisions}`,
    },
  ];
}
