import type {
  LoopConfig,
  LoopConfigUpdateRequest,
  LoopContract,
  LoopEffectiveConfig,
} from "../types";
import {
  buildCheckDescriptors,
  defaultCheckStates,
  initialCheckStates,
  serializeEnabledChecks,
  type LoopConfigCheckDescriptor,
  type LoopConfigCheckState,
} from "./loop-config-checks";
import {
  buildOverrideFields,
  clampOverrideValue,
  initialOverrideDraft,
  type LoopBudgetPolicy,
  type LoopOverrideDraft,
  type LoopOverrideKey,
} from "./loop-overrides";

/** Re-attempt granularity (ADR-009). `failed_only` is the sensible runtime default. */
export type LoopReattemptStrategy = NonNullable<LoopConfig["reattempt_strategy"]>;
const DEFAULT_REATTEMPT: LoopReattemptStrategy = "failed_only";

/**
 * The full no-fork configure draft (design §4.7): the editable verification checks, the
 * human-approval-gate flag, the re-attempt strategy, and the 6 per-loop limit overrides
 * (reused from the run-form Advanced grid). Structural fields are deliberately absent —
 * changing them requires a fork (ADR-009).
 */
export interface LoopConfigDraft {
  checks: Record<string, LoopConfigCheckState>;
  humanGateEnabled: boolean;
  reattemptStrategy: LoopReattemptStrategy;
  limits: LoopOverrideDraft;
}

function normalizeReattempt(
  raw: LoopConfig["reattempt_strategy"] | null | undefined
): LoopReattemptStrategy {
  return raw === "full_body" ? "full_body" : DEFAULT_REATTEMPT;
}

function normalizePolicy(
  raw: string | null | undefined,
  fallback: LoopBudgetPolicy
): LoopBudgetPolicy {
  if (raw === "escalate") return "escalate";
  if (raw === "halt") return "halt";
  return fallback;
}

/** Reads one stored limit column, converting wall-clock seconds back to the field's minutes. */
function storedLimitValue(key: LoopOverrideKey, stored: LoopConfig): number | null {
  switch (key) {
    case "iteration_cap":
      return stored.iteration_cap ?? null;
    case "budget_tokens":
      return stored.budget_tokens ?? null;
    case "budget_wall_sec":
      return stored.budget_wall_sec != null ? Math.round(stored.budget_wall_sec / 60) : null;
    case "no_progress_window":
      return stored.no_progress_window ?? null;
    case "fan_out_width":
      return stored.fan_out_width ?? null;
    case "gate_max_revisions":
      return stored.gate_max_revisions ?? null;
    default:
      return null;
  }
}

/** Seeds the limit-override draft from the stored config, clamping every value to its ceiling. */
function limitsFromStored(
  effectiveConfig: LoopEffectiveConfig,
  stored: LoopConfig | null
): LoopOverrideDraft {
  const base = initialOverrideDraft(effectiveConfig);
  if (!stored) return base;
  const values = { ...base.values };
  for (const field of buildOverrideFields(effectiveConfig)) {
    const raw = storedLimitValue(field.key, stored);
    if (raw == null) continue;
    values[field.key] = clampOverrideValue(field, raw);
  }
  return {
    values,
    budgetOnExceeded: normalizePolicy(stored.budget_on_exceeded, base.budgetOnExceeded),
  };
}

/**
 * Seeds the configure draft from the stored `loop_config` overlaid on the contract defaults.
 * A missing stored config yields the inherited defaults (all checks on, human gate off,
 * `failed_only`, limits at the loop defaults) so an unconfigured loop opens truthfully.
 */
export function initialConfigDraft(
  descriptors: LoopConfigCheckDescriptor[],
  stored: LoopConfig | null,
  effectiveConfig: LoopEffectiveConfig
): LoopConfigDraft {
  return {
    checks: initialCheckStates(descriptors, stored?.enabled_checks_json ?? null),
    humanGateEnabled: stored?.human_gate_enabled ?? false,
    reattemptStrategy: normalizeReattempt(stored?.reattempt_strategy),
    limits: limitsFromStored(effectiveConfig, stored),
  };
}

/** The inherited-defaults draft used by Reset: every check on, human off, `failed_only`, no limits. */
export function resetConfigDraft(
  descriptors: LoopConfigCheckDescriptor[],
  effectiveConfig: LoopEffectiveConfig
): LoopConfigDraft {
  return {
    checks: defaultCheckStates(descriptors),
    humanGateEnabled: false,
    reattemptStrategy: DEFAULT_REATTEMPT,
    limits: initialOverrideDraft(effectiveConfig),
  };
}

/**
 * Projects the draft into the `PUT /config` body. Unlike the ephemeral run-form overrides,
 * the persistent config store pins every value the operator actually sets: a typed numeric is
 * stored explicitly (blank = `null` = inherit), so a pin is never dropped just because it
 * equals the definition default while a diverging `[loops.defaults.*]` operator layer — which
 * the web layer can't see — would otherwise win (R-004). Wall clock is stored in seconds. The
 * human gate, re-attempt strategy and budget policy are sent explicitly as the per-loop
 * defaults. There is NO cost field — cost is display-only (ADR-017 §9.5.2).
 */
export function buildLoopConfigRequest(
  draft: LoopConfigDraft,
  descriptors: LoopConfigCheckDescriptor[]
): LoopConfigUpdateRequest {
  const values = draft.limits.values;
  return {
    config: {
      iteration_cap: values.iteration_cap ?? null,
      budget_tokens: values.budget_tokens ?? null,
      budget_wall_sec: values.budget_wall_sec !== undefined ? values.budget_wall_sec * 60 : null,
      no_progress_window: values.no_progress_window ?? null,
      fan_out_width: values.fan_out_width ?? null,
      gate_max_revisions: values.gate_max_revisions ?? null,
      budget_on_exceeded: draft.limits.budgetOnExceeded,
      human_gate_enabled: draft.humanGateEnabled,
      reattempt_strategy: draft.reattemptStrategy,
      enabled_checks_json: serializeEnabledChecks(descriptors, draft.checks),
    },
  };
}

/** Convenience: build the descriptors + seeded draft in one call for the view-model hook. */
export function buildConfigureModel(
  contract: LoopContract,
  stored: LoopConfig | null,
  effectiveConfig: LoopEffectiveConfig
) {
  const descriptors = buildCheckDescriptors(contract);
  return { descriptors, draft: initialConfigDraft(descriptors, stored, effectiveConfig) };
}
