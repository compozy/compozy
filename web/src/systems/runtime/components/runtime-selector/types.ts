import { type ReasoningEffort } from "@/lib/api-contract";

/**
 * Canonical reasoning vocabulary for the frontend, single-sourced here. The
 * duplicated leaf-selector effort constants are deleted. Order drives the
 * intensity meter position; the label map drives every human-facing effort
 * label. Empty string is deliberately outside this set and means "use provider
 * default".
 */
export const REASONING_EFFORT_ORDER = [
  "none",
  "minimal",
  "low",
  "medium",
  "high",
  "xhigh",
  "max",
] as const satisfies readonly ReasoningEffort[];

export const REASONING_EFFORT_LABELS: Record<ReasoningEffort, string> = {
  none: "None",
  minimal: "Minimal",
  low: "Low",
  medium: "Medium",
  high: "High",
  xhigh: "Extra high",
  max: "Max",
};

/** 1-based canonical position (1..7) used to fill the intensity meter. */
export function reasoningEffortPosition(effort: string): number {
  const index = (REASONING_EFFORT_ORDER as readonly string[]).indexOf(effort);
  return index < 0 ? 0 : index + 1;
}

export function reasoningEffortLabel(effort: string): string {
  return (REASONING_EFFORT_LABELS as Record<string, string>)[effort] ?? effort;
}

/**
 * Emitted wire value. Empty `model`/`reasoning_effort` mean "use provider
 * default" and are omitted from POST bodies by each surface's submit mapper
 * (existing convention, `_spec.md` §7.9). `reasoning_effort` is constrained to
 * the canonical enum (or `""`) so no surface can ever emit an off-contract value
 * such as `ultra`; the selector only ever produces these values.
 */
export interface RuntimeSelectorValue {
  provider: string;
  model: string;
  reasoning_effort: ReasoningEffort | "";
}

/** `composer` removes trigger chrome while preserving runtime identity. */
export type RuntimeSelectorVariant = "default" | "small" | "compact" | "composer";

export type RuntimeAvailability = "live" | "stale" | "unavailable";
export type RuntimeReasoningSource = "acp" | "catalog";

/** Provider rail option. Auth state drives dimming + disabled model rows. */
export interface RuntimeProviderOption {
  id: string;
  name: string;
  harness?: string;
  runtime_provider?: string;
  needs_auth?: boolean;
}

/**
 * Presentation-only model option. Surfaces map the daemon `ProviderModelPayload`
 * onto this shape (see `model-catalog/lib/to-runtime-selector-options`) so the
 * selector stays decoupled from `systems/model-catalog`.
 */
export interface RuntimeModelOption {
  id: string;
  provider: string;
  name: string;
  context_window?: number | null;
  cost_input?: number | null;
  cost_output?: number | null;
  cost_cache_read?: number | null;
  cost_cache_write?: number | null;
  cost_reasoning?: number | null;
  supports_tools?: boolean | null;
  supports_reasoning?: boolean | null;
  /** Selectable effort subset the runtime can honor; empty = not selectable. */
  efforts: ReasoningEffort[];
  /** Canonical default within `efforts` ("" = provider default). Sanitized by the mapper. */
  default_effort?: ReasoningEffort | "";
  reasoning_source?: RuntimeReasoningSource;
  availability: RuntimeAvailability;
  curated?: boolean;
  featured?: boolean;
  release_date?: string;
  disabled?: boolean;
  disabled_reason?: string;
}

export type RuntimeReasoningMode = "levels" | "supported-nolevels" | "none" | "no-model";

export interface RuntimeReasoningState {
  mode: RuntimeReasoningMode;
  levels: ReasoningEffort[];
  /** Canonical default within the level set ("" = provider default). */
  defaultEffort: ReasoningEffort | "";
  source: RuntimeReasoningSource;
}

/**
 * Resolve the reasoning footer/trigger mode for a model (mirrors the design's
 * `reasoningStateFor`): no selected model renders the "pick a model" prompt; a
 * non-empty effort subset renders selectable levels; `supports_reasoning`
 * without levels renders the "provider decides" note; a selected model with
 * neither renders the "no reasoning" note. `no-model` and `none` are kept
 * distinct so the footer never claims "this model…" when nothing is selected.
 *
 * `none` is not a selectable stop: turning reasoning off is not something the
 * selector offers, so the slider starts at the lowest real level. A model
 * advertising only `none` therefore has no selectable levels at all, and a
 * `default_effort` outside the filtered set collapses to "" (provider default).
 */
export function resolveReasoningState(
  model: RuntimeModelOption | undefined
): RuntimeReasoningState {
  if (!model) {
    return { mode: "no-model", levels: [], defaultEffort: "", source: "catalog" };
  }
  const levels: ReasoningEffort[] = model.efforts.filter(effort => effort !== "none");
  if (levels.length > 0) {
    const fallback = model.default_effort ?? "";
    return {
      mode: "levels",
      levels,
      defaultEffort: fallback !== "" && levels.includes(fallback) ? fallback : "",
      source: model.reasoning_source ?? "catalog",
    };
  }
  if (model.supports_reasoning) {
    return { mode: "supported-nolevels", levels: [], defaultEffort: "", source: "catalog" };
  }
  return { mode: "none", levels: [], defaultEffort: "", source: "catalog" };
}
