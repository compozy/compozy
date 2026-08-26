import type {
  LoopConfig,
  LoopConfigUpdateRequest,
  LoopContract,
  LoopEffectiveConfig,
  LoopEnvironmentSpec,
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

export type LoopReattemptStrategy = NonNullable<LoopConfig["reattempt_strategy"]>;
const DEFAULT_REATTEMPT: LoopReattemptStrategy = "failed_only";

export interface LoopConfigDraft {
  checks: Record<string, LoopConfigCheckState>;
  humanGateEnabled: boolean;
  reattemptStrategy: LoopReattemptStrategy;
  limits: LoopOverrideDraft;
  /** The loop-level environment default. `null` means the loop pins none. */
  environment: LoopEnvironmentSpec | null;
}

/**
 * Keeps only the companion key its mode allows, so a mode switch can never send
 * a combination the daemon rejects (`worktree` carries only `worktree_ref`,
 * `directory` only `directory`, `root`/`per_run` neither).
 */
export function normalizeLoopEnvironment(
  raw: LoopEnvironmentSpec | null | undefined
): LoopEnvironmentSpec | null {
  if (!raw) return null;
  if (raw.mode === "worktree") {
    return { mode: "worktree", worktree_ref: raw.worktree_ref ?? "" };
  }
  if (raw.mode === "directory") {
    return { mode: "directory", directory: raw.directory ?? "" };
  }
  return { mode: raw.mode };
}

function normalizeReattempt(
  raw: LoopConfig["reattempt_strategy"] | null | undefined
): LoopReattemptStrategy {
  if (raw === "full_body") return "full_body";
  if (raw === "halt") return "halt";
  return DEFAULT_REATTEMPT;
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

/** Seeds the limit-override draft from stored config and normalizes each numeric value. */
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
    environment: base.environment,
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
    environment: normalizeLoopEnvironment(stored?.environment),
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
    environment: null,
  };
}

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
      // `PUT /config` replaces the stored block wholesale, so the environment
      // default has to ride every save — omitting it would silently unpin it.
      environment: normalizeLoopEnvironment(draft.environment),
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
