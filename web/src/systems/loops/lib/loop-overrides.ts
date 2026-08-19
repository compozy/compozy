import type { LoopEffectiveConfig, LoopEnvironmentSpec, RunLoopRequest } from "../types";
import { resolveLoopEffectiveConfig } from "./loop-effective-config";
import { LOOP_CEILINGS } from "./loop-limits";

/** The six per-run numeric overrides. Most clamp to a daemon ceiling. */
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
  /** Hard numeric ceiling (minutes for wall clock); absent when the field has no fixed cap. */
  ceiling?: number;
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
      label: "Fan-out window",
      ceilingLabel: "no fixed cap",
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
  /** `null` follows the Loop default; a value overrides only this run. */
  environment: LoopEnvironmentSpec | null;
}

/** Seeds an empty draft with the contract's own budget policy so "no override" is truthful. */
export function initialOverrideDraft(effectiveConfig: LoopEffectiveConfig): LoopOverrideDraft {
  const effective = resolveLoopEffectiveConfig(effectiveConfig);
  return {
    values: {},
    budgetOnExceeded: effective.budget_on_exceeded === "escalate" ? "escalate" : "halt",
    environment: null,
  };
}

function environmentsEqual(
  left: LoopEnvironmentSpec | null | undefined,
  right: LoopEnvironmentSpec | null | undefined
): boolean {
  if (!left || !right) return left === right;
  return (
    left.mode === right.mode &&
    left.worktree_ref === right.worktree_ref &&
    left.directory === right.directory
  );
}

/** Empty companion values never form a runnable per-run override. */
export function isLoopEnvironmentOverrideValid(
  environment: LoopEnvironmentSpec | null,
  gitBacked: boolean
): boolean {
  if (!environment) return true;
  if ((environment.mode === "worktree" || environment.mode === "per_run") && !gitBacked) {
    return false;
  }
  if (environment.mode === "worktree") return Boolean(environment.worktree_ref?.trim());
  if (environment.mode === "directory") return Boolean(environment.directory?.trim());
  return true;
}

/** Normalizes a typed override and applies its ceiling when one exists. */
export function clampOverrideValue(field: LoopOverrideField, raw: number): number {
  if (!Number.isFinite(raw)) return 0;
  const floored = Math.max(0, Math.trunc(raw));
  return field.ceiling === undefined ? floored : Math.min(floored, field.ceiling);
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

export function hasActiveOverrides(
  draft: LoopOverrideDraft,
  effectiveConfig: LoopEffectiveConfig
): boolean {
  if (
    draft.environment &&
    !environmentsEqual(draft.environment, effectiveConfig.environment ?? null)
  ) {
    return true;
  }
  const fields = buildOverrideFields(effectiveConfig);
  for (const field of fields) {
    const value = draft.values[field.key];
    if (value !== undefined && !isFieldDefault(field, value)) return true;
  }
  const effective = resolveLoopEffectiveConfig(effectiveConfig);
  return draft.budgetOnExceeded !== effective.budget_on_exceeded;
}

/**
 * The one-line gist a folded Limits panel must still state: how many generations this
 * run gets, whether any budget is enforced, and whether the loop's saved defaults are
 * in play. Reads the draft on top of the effective config, so it stays true while the
 * operator types.
 */
export function summarizeRunLimits(
  draft: LoopOverrideDraft,
  effectiveConfig: LoopEffectiveConfig
): string {
  const effective = resolveLoopEffectiveConfig(effectiveConfig);
  const generations = draft.values.iteration_cap ?? effective.iteration_cap;
  const tokens = draft.values.budget_tokens ?? effective.budget_tokens;
  const wallSeconds =
    draft.values.budget_wall_sec !== undefined
      ? draft.values.budget_wall_sec * 60
      : effective.budget_wall_sec;
  const budgets = tokens > 0 || wallSeconds > 0 ? "budgets set" : "no budgets set";
  const source = hasActiveOverrides(draft, effectiveConfig) ? "overrides set" : "loop defaults";
  return `${generations} generations · ${budgets} · ${source}`;
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
  if (
    draft.environment &&
    !environmentsEqual(draft.environment, effectiveConfig.environment ?? null)
  ) {
    overrides.environment = draft.environment;
  }
  return overrides;
}
