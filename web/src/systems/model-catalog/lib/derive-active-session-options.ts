import type { OperationResponse } from "@/lib/api-contract";

import { isKnownAvailabilityState, type ProviderModelPayload } from "../types";

type ACPCaps = NonNullable<OperationResponse<"listSessions", 200>["sessions"][number]["acp_caps"]>;
type ACPConfigOption = NonNullable<ACPCaps["config_options"]>[number];

const MODEL_OPTION_IDS = new Set(["model"]);
const REASONING_OPTION_IDS = new Set(["reasoning_effort", "effort"]);

export interface ModelOption {
  id: string;
  displayName: string;
  availabilityState: string;
  available: boolean | null;
  stale: boolean;
  refreshedAt?: string;
  source: "catalog" | "acp";
}

export interface ReasoningOption {
  value: string;
  label: string;
  source: "catalog" | "acp";
}

export interface ActiveSessionDerivedOptions {
  modelOptions: ModelOption[];
  reasoningOptions: ReasoningOption[];
  reasoningSupported: boolean;
  modelOverrideAvailable: boolean;
  reasoningOverrideAvailable: boolean;
  defaultReasoning: string | null;
  /** Source of the advertised reasoning options (`acp` config option vs catalog). */
  reasoningSource: "acp" | "catalog";
}

export interface DeriveOptionsInput {
  catalog: ProviderModelPayload[];
  configOptions?: ACPConfigOption[];
  selectedModel?: string | null;
}

export function deriveActiveSessionOptions(input: DeriveOptionsInput): ActiveSessionDerivedOptions {
  const acpModelOption = findConfigOption(input.configOptions, MODEL_OPTION_IDS);
  const acpReasoningOption = findConfigOption(input.configOptions, REASONING_OPTION_IDS);
  const modelOptions = buildModelOptions(input.catalog, acpModelOption);
  const reasoningOptions = buildReasoningOptions(
    input.catalog,
    acpReasoningOption,
    input.selectedModel ?? null
  );
  const reasoningSupported = reasoningOptions.length > 0;
  const defaultReasoning = pickDefaultReasoning(
    input.catalog,
    acpReasoningOption,
    input.selectedModel ?? null
  );
  const reasoningSource = resolveReasoningSource(
    input.catalog,
    acpReasoningOption,
    input.selectedModel ?? null
  );
  return {
    modelOptions,
    reasoningOptions,
    reasoningSupported,
    modelOverrideAvailable: acpModelOption !== undefined,
    reasoningOverrideAvailable: acpReasoningOption !== undefined,
    defaultReasoning,
    reasoningSource,
  };
}

function resolveReasoningSource(
  catalog: ProviderModelPayload[],
  acpOption: ACPConfigOption | undefined,
  selectedModel: string | null
): "acp" | "catalog" {
  if (acpOption) return "acp";
  const targetModel = selectedModel?.trim();
  if (targetModel) {
    const matched = catalog.find(model => model.model_id === targetModel);
    if (matched?.reasoning_source === "acp") return "acp";
  }
  return "catalog";
}

function findConfigOption(
  options: ACPConfigOption[] | undefined,
  ids: Set<string>
): ACPConfigOption | undefined {
  if (!options) return undefined;
  for (const option of options) {
    if (ids.has(option.id)) {
      return option;
    }
  }
  return undefined;
}

function buildModelOptions(
  catalog: ProviderModelPayload[],
  acpOption: ACPConfigOption | undefined
): ModelOption[] {
  if (acpOption) {
    const catalogById = new Map(catalog.map(model => [model.model_id, model]));
    const fromAcp = (acpOption.values ?? []).map<ModelOption>(value => {
      const enriched = catalogById.get(value.value);
      return {
        id: value.value,
        displayName: value.label?.trim() || enriched?.display_name?.trim() || value.value,
        availabilityState: enriched?.availability_state ?? "unknown",
        available: enriched?.available ?? null,
        stale: enriched?.stale ?? false,
        refreshedAt: enriched?.refreshed_at,
        source: "acp",
      };
    });
    return dedupeOptions(fromAcp);
  }
  const fromCatalog = catalog.map<ModelOption>(model => ({
    id: model.model_id,
    displayName: model.display_name?.trim() || model.model_id,
    availabilityState: isKnownAvailabilityState(model.availability_state)
      ? model.availability_state
      : "unknown",
    available: model.available ?? null,
    stale: model.stale,
    refreshedAt: model.refreshed_at,
    source: "catalog",
  }));
  return dedupeOptions(fromCatalog);
}

function buildReasoningOptions(
  catalog: ProviderModelPayload[],
  acpOption: ACPConfigOption | undefined,
  selectedModel: string | null
): ReasoningOption[] {
  if (acpOption) {
    const values = acpOption.values ?? [];
    if (values.length === 0) {
      return [];
    }
    return values.map<ReasoningOption>(value => ({
      value: value.value,
      label: value.label?.trim() || value.value,
      source: "acp",
    }));
  }
  const targetModel = selectedModel?.trim();
  if (!targetModel) {
    return [];
  }
  const matched = catalog.find(model => model.model_id === targetModel);
  if (!matched) {
    return [];
  }
  if (matched.supports_reasoning === false) {
    return [];
  }
  const efforts = matched.reasoning_efforts ?? [];
  if (efforts.length === 0) {
    return [];
  }
  const source = matched.reasoning_source === "acp" ? "acp" : "catalog";
  return efforts.map<ReasoningOption>(effort => ({
    value: effort,
    label: effort,
    source,
  }));
}

function pickDefaultReasoning(
  catalog: ProviderModelPayload[],
  acpOption: ACPConfigOption | undefined,
  selectedModel: string | null
): string | null {
  if (acpOption?.current && acpOption.current.trim().length > 0) {
    return acpOption.current.trim();
  }
  const targetModel = selectedModel?.trim();
  if (!targetModel) {
    return null;
  }
  const matched = catalog.find(model => model.model_id === targetModel);
  const candidate = matched?.default_reasoning_effort;
  if (typeof candidate !== "string") {
    return null;
  }
  const trimmed = candidate.trim();
  return trimmed.length === 0 ? null : trimmed;
}

function dedupeOptions(options: ModelOption[]): ModelOption[] {
  const seen = new Set<string>();
  const result: ModelOption[] = [];
  for (const option of options) {
    const id = option.id.trim();
    if (id.length === 0 || seen.has(id)) {
      continue;
    }
    seen.add(id);
    result.push({ ...option, id });
  }
  return result.sort((a, b) => a.id.localeCompare(b.id));
}
