import { isReasoningEffort } from "@/lib/api-contract";
import {
  runtimeModelKey,
  type RuntimeAvailability,
  type RuntimeModelOption,
} from "@/systems/runtime";

import type { ProviderModelPayload } from "../types";

/** Provider auth states that require sign-in before the provider can be used. */
const NEEDS_AUTH_STATES = new Set(["needs_login", "missing_credential"]);

export function providerNeedsAuth(state: string | undefined | null): boolean {
  return NEEDS_AUTH_STATES.has((state ?? "").trim());
}

function toAvailability(state: string, available: boolean | null | undefined): RuntimeAvailability {
  switch (state) {
    case "available_live":
      return "live";
    case "available_stale":
      return "stale";
    case "unavailable_live":
    case "unavailable_stale":
      return "unavailable";
    default:
      return available === false ? "unavailable" : "live";
  }
}

export interface ToRuntimeModelOptionsInput {
  /** When the owning provider needs sign-in, every model row is disabled. */
  providerNeedsAuth?: boolean;
}

/**
 * Adapt the daemon `ProviderModelPayload` list onto the presentation-agnostic
 * `RuntimeModelOption` shape the selector consumes. Availability, disabled
 * state, and the selectable effort subset are derived here so the selector
 * stays decoupled from `systems/model-catalog`.
 */
export function toRuntimeModelOptions(
  models: ProviderModelPayload[],
  input: ToRuntimeModelOptionsInput = {}
): RuntimeModelOption[] {
  const needsAuth = input.providerNeedsAuth ?? false;
  // Dedupe on the compound (provider, model) identity, never the bare model id:
  // two providers publishing the same model id are distinct rows that must both
  // survive a cross-provider merge.
  const seen = new Set<string>();
  const result: RuntimeModelOption[] = [];
  for (const model of models) {
    const id = model.model_id.trim();
    if (id.length === 0) continue;
    const key = runtimeModelKey(model.provider_id, id);
    if (seen.has(key)) continue;
    seen.add(key);
    const availability = needsAuth
      ? "unavailable"
      : toAvailability(model.availability_state, model.available);
    const disabled = needsAuth || availability === "unavailable";
    // Only canonical efforts survive, and the default is accepted ONLY when it is
    // canonical AND inside that filtered subset. An off-contract default (e.g.
    // "ultra") or a canonical-but-out-of-subset default (e.g. "max" for
    // ["low","high"]) collapses to "" — provider default — never a level the UI
    // would render as selected while the model can't honor it.
    const efforts = (model.reasoning_efforts ?? []).filter(isReasoningEffort);
    const rawDefault = model.default_reasoning_effort;
    const defaultEffort =
      rawDefault && isReasoningEffort(rawDefault) && efforts.includes(rawDefault) ? rawDefault : "";
    result.push({
      id,
      provider: model.provider_id,
      name: model.display_name?.trim() || id,
      context_window: model.context_window ?? null,
      cost_input: model.cost?.input_per_million ?? null,
      cost_output: model.cost?.output_per_million ?? null,
      cost_cache_read: model.cost?.cache_read_per_million ?? null,
      cost_cache_write: model.cost?.cache_write_per_million ?? null,
      cost_reasoning: model.cost?.reasoning_per_million ?? null,
      supports_tools: model.supports_tools ?? null,
      supports_reasoning: model.supports_reasoning ?? null,
      efforts,
      default_effort: defaultEffort,
      reasoning_source: model.reasoning_source,
      availability,
      curated: model.curated,
      featured: model.featured,
      release_date: model.release_date,
      disabled,
      disabled_reason: needsAuth ? "Sign in" : disabled ? "Unavailable" : undefined,
    });
  }
  return result;
}
