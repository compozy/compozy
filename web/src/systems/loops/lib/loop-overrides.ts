import type { LoopEffectiveConfig, RunLoopRequest } from "../types";
import { resolveLoopEffectiveConfig } from "./loop-effective-config";
import { LOOP_CEILINGS } from "./loop-limits";

/** The 6 per-run limit overrides that clamp to a daemon ceiling (§4.3 / §5.4). */
export type LoopOverrideKey =
  | "iteration_cap"
  | "budget_tokens"
  | "budget_wall_sec"
  | "no_progress_window"
  | "fan_out_width"
  | "gate_max_revisions";

/** The budget-exceeded policy is a select, not a clamped number, but rides the grid. */
export type LoopBudgetPolicy = "halt" | "escalate";

export interface LoopOverrideField {
  key: LoopOverrideKey;
  label: string;
  /** Hard numeric ceiling (minutes for wall clock); the field clamps at this value. */
  ceiling: number;
  /** Right-hand ceiling label (`/ 100`, `/ 20M`, `/ 7d`). */
  ceilingLabel: string;
  /** Placeholder shown when the override is unset (`off`, `≤ tasks`). */
  placeholder: string;
  /**
   * Per-loop default for this run, read from saved config; `null` when the default
   * is off/unbounded (renders the placeholder). The value the "overrides set" badge
   * compares against.
   */
  defaultValue: number | null;
}

/**
 * The 6 clamped override fields, each pairing its per-loop default (from the
 * saved configuration) with its hard daemon ceiling. Cost is deliberately absent — it is a
 * display-only metric with no enforced cap (ADR-017 §9.5.2), so the run form
 * renders NO cost-cap input.
 */
export function buildOverrideFields(effectiveConfig: LoopEffectiveConfig): LoopOverrideField[] {
  const effective = resolveLoopEffectiveConfig(effectiveConfig);
  const iterationDefault = effective.iteration_cap > 0 ? effective.iteration_cap : null;
  const tokensDefault = effective.budget_tokens > 0 ? effective.budget_tokens : null;
  const wallDefaultMin =
    effective.budget_wall_sec > 0 ? Math.round(effective.budget_wall_sec / 60) : null;
  return [
    {
      key: "iteration_cap",
      label: "Iteration cap",
      ceiling: LOOP_CEILINGS.iterationCap,
      ceilingLabel: `/ ${LOOP_CEILINGS.iterationCap}`,
      placeholder: "∞",
      defaultValue: iterationDefault,
    },
    {
      key: "budget_tokens",
      label: "Token budget",
      ceiling: LOOP_CEILINGS.tokensMax,
      ceilingLabel: `/ ${LOOP_CEILINGS.tokens}`,
      placeholder: "off",
      defaultValue: tokensDefault,
    },
    {
      key: "budget_wall_sec",
      label: "Wall clock (min)",
      ceiling: LOOP_CEILINGS.wallClockMinutes,
      ceilingLabel: `/ ${LOOP_CEILINGS.wallClock}`,
      placeholder: "off",
      defaultValue: wallDefaultMin,
    },
    {
      key: "no_progress_window",
      label: "No-progress window",
      ceiling: LOOP_CEILINGS.noProgressWindow,
      ceilingLabel: `/ ${LOOP_CEILINGS.noProgressWindow}`,
      placeholder: String(effective.no_progress_window),
      defaultValue: effective.no_progress_window,
    },
    {
      key: "fan_out_width",
      label: "Fan-out ceiling",
      ceiling: LOOP_CEILINGS.fanOutBreadth,
      ceilingLabel: `/ ${LOOP_CEILINGS.fanOutBreadth}`,
      placeholder: "≤ tasks",
      defaultValue: effective.fan_out_width > 0 ? effective.fan_out_width : null,
    },
    {
      key: "gate_max_revisions",
      label: "Gate max revisions",
      ceiling: LOOP_CEILINGS.gateMaxRevisions,
      ceilingLabel: `/ ${LOOP_CEILINGS.gateMaxRevisions}`,
      placeholder: String(effective.gate_max_revisions),
      defaultValue: effective.gate_max_revisions,
    },
  ];
}

/** The per-run override draft: the set numeric fields + the budget-exceeded policy. */
export interface LoopOverrideDraft {
  values: Partial<Record<LoopOverrideKey, number>>;
  budgetOnExceeded: LoopBudgetPolicy;
}

/** Seeds an empty draft with the contract's own budget policy so "no override" is truthful. */
export function initialOverrideDraft(effectiveConfig: LoopEffectiveConfig): LoopOverrideDraft {
  const effective = resolveLoopEffectiveConfig(effectiveConfig);
  return {
    values: {},
    budgetOnExceeded: effective.budget_on_exceeded === "escalate" ? "escalate" : "halt",
  };
}

/** Clamps a typed override to `[0, ceiling]`; non-finite input clears the override. */
export function clampOverrideValue(field: LoopOverrideField, raw: number): number {
  if (!Number.isFinite(raw)) return 0;
  const floored = Math.max(0, Math.trunc(raw));
  return Math.min(floored, field.ceiling);
}

/**
 * True when `value` equals the field's per-loop default. `0` on an off-by-default budget
 * field (`defaultValue === null`) counts as the default, since `0 = unlimited/off` — so
 * typing `0` never flips the badge or emits a redundant `budget_*: 0` override (N-004).
 */
function isFieldDefault(field: LoopOverrideField, value: number): boolean {
  if (field.defaultValue === null) return value === 0;
  return value === field.defaultValue;
}

/**
 * True when any override diverges from its per-loop default (typing the default back
 * in is not an override — mirrors the design's badge, §4.3). A set value that equals
 * the default, or a policy equal to the contract's, does not flip the badge.
 */
export function hasActiveOverrides(
  draft: LoopOverrideDraft,
  effectiveConfig: LoopEffectiveConfig
): boolean {
  const fields = buildOverrideFields(effectiveConfig);
  for (const field of fields) {
    const value = draft.values[field.key];
    if (value !== undefined && !isFieldDefault(field, value)) return true;
  }
  const effective = resolveLoopEffectiveConfig(effectiveConfig);
  return draft.budgetOnExceeded !== effective.budget_on_exceeded;
}

/**
 * Projects the draft into the `runLoop` `config_overrides` body: only fields that
 * differ from the per-loop default are sent (wall clock converted minutes -> seconds).
 * Returns `null` when nothing is overridden, so an untouched Advanced panel never
 * pins a redundant config.
 */
export function buildConfigOverrides(
  draft: LoopOverrideDraft,
  effectiveConfig: LoopEffectiveConfig
): NonNullable<RunLoopRequest["config_overrides"]> | null {
  if (!hasActiveOverrides(draft, effectiveConfig)) return null;
  const fields = buildOverrideFields(effectiveConfig);
  const overrides: NonNullable<RunLoopRequest["config_overrides"]> = {};
  for (const field of fields) {
    const value = draft.values[field.key];
    if (value === undefined || isFieldDefault(field, value)) continue;
    if (field.key === "budget_wall_sec") {
      overrides.budget_wall_sec = value * 60;
    } else {
      overrides[field.key] = value;
    }
  }
  const effective = resolveLoopEffectiveConfig(effectiveConfig);
  if (draft.budgetOnExceeded !== effective.budget_on_exceeded) {
    overrides.budget_on_exceeded = draft.budgetOnExceeded;
  }
  return overrides;
}
