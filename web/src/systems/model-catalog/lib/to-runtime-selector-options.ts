import { isReasoningEffort } from "@/lib/api-contract";

import type { ProviderModelPayload } from "../types";
import {
  type RuntimeAvailability,
  type RuntimeACPOption,
  type RuntimeModelConfiguration,
  runtimeModelKey,
  type RuntimeModelOption,
} from "@/systems/runtime";

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

function normalizeConfigurations(
  configurations: ProviderModelPayload["configurations"]
): RuntimeModelConfiguration[] | undefined {
  if (!configurations || configurations.length === 0) return undefined;
  const seen = new Set<string>();
  const normalized: RuntimeModelConfiguration[] = [];
  for (const configuration of configurations) {
    const reasoning = configuration.reasoning_effort;
    const key = [
      typeof reasoning === "string" ? reasoning : "",
      configuration.fast === true ? "true" : configuration.fast === false ? "false" : "",
      configuration.thinking === true ? "true" : configuration.thinking === false ? "false" : "",
    ].join("\u001f");
    if (seen.has(key)) continue;
    const normalizedConfiguration: RuntimeModelConfiguration = {
      ...(typeof reasoning === "string" && isReasoningEffort(reasoning)
        ? { reasoning_effort: reasoning }
        : {}),
      ...(typeof configuration.fast === "boolean" ? { fast: configuration.fast } : {}),
      ...(typeof configuration.thinking === "boolean" ? { thinking: configuration.thinking } : {}),
    };
    if (Object.keys(normalizedConfiguration).length === 0) continue;
    seen.add(key);
    normalized.push(normalizedConfiguration);
  }
  return normalized.length > 0 ? normalized : undefined;
}

function normalizeConfigOptions(
  options: ProviderModelPayload["config_options"]
): RuntimeACPOption[] | undefined {
  if (!options || options.length === 0) return undefined;
  const seen = new Set<string>();
  const normalized: RuntimeACPOption[] = [];
  for (const option of options) {
    const id = option.id.trim();
    if (id.length === 0 || seen.has(id)) continue;
    seen.add(id);
    normalized.push({
      id,
      kind: option.kind,
      ...(option.label?.trim() ? { label: option.label.trim() } : {}),
      ...(option.description?.trim() ? { description: option.description.trim() } : {}),
      ...(option.category?.trim() ? { category: option.category.trim() } : {}),
      ...(option.current_value_id?.trim()
        ? { current_value_id: option.current_value_id.trim() }
        : {}),
      ...(typeof option.current_bool === "boolean" ? { current_bool: option.current_bool } : {}),
      ...(option.values && option.values.length > 0
        ? {
            values: option.values.map(value => ({
              value: value.value.trim(),
              ...(value.label?.trim() ? { label: value.label.trim() } : {}),
              ...(value.description?.trim() ? { description: value.description.trim() } : {}),
              ...(value.group_id?.trim() ? { group_id: value.group_id.trim() } : {}),
              ...(value.group_label?.trim() ? { group_label: value.group_label.trim() } : {}),
            })),
          }
        : {}),
    });
  }
  return normalized.length > 0 ? normalized : undefined;
}

function modelEfforts(
  rawEfforts: ProviderModelPayload["reasoning_efforts"],
  configurations: RuntimeModelConfiguration[] | undefined
): RuntimeModelOption["efforts"] {
  const configured = configurations
    ?.map(configuration => configuration.reasoning_effort)
    .filter(
      (effort): effort is NonNullable<typeof effort> =>
        typeof effort === "string" && isReasoningEffort(effort)
    );
  const candidates = configurations ? (configured ?? []) : (rawEfforts ?? []);
  return [...new Set(candidates.filter(isReasoningEffort))];
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
    const configurations = normalizeConfigurations(model.configurations);
    const acpOptions = normalizeConfigOptions(model.config_options);
    // Only canonical efforts survive, and the default is accepted ONLY when it is
    // canonical AND inside that filtered subset. An off-contract default (e.g.
    // "ultra") or a canonical-but-out-of-subset default (e.g. "max" for
    // ["low","high"]) collapses to "" — provider default — never a level the UI
    // would render as selected while the model can't honor it.
    const efforts = modelEfforts(model.reasoning_efforts, configurations);
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
      configurations,
      ...(acpOptions ? { acp_options: acpOptions } : {}),
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
