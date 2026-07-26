import type { ReasoningEffort } from "@/lib/api-contract";
import type { SettingsProviderRequest } from "@/systems/settings";

import type { OnboardingAuthMode } from "../stores/use-onboarding-draft-store";

export interface ProviderRequestInputs {
  model: string;
  reasoning: ReasoningEffort | "";
  authMode: OnboardingAuthMode;
  envVar: string;
  apiKey: string;
  provider: string;
}

type ProviderSettings = NonNullable<SettingsProviderRequest["settings"]>;
type ProviderModelsPayload = NonNullable<ProviderSettings["models"]>;

function existingApiKeyTargetEnv(current: ProviderSettings): string {
  const slot = current.credential_slots?.find(entry => entry.name === "api_key");
  return slot?.target_env?.trim() ?? "";
}

function buildProviderModels(current: ProviderSettings, model: string): ProviderModelsPayload {
  const base = current.models ?? {};
  const curated = (base.curated ?? []).map(entry => ({ id: entry.id }));
  if (model.length > 0 && !curated.some(entry => entry.id === model)) {
    curated.push({ id: model });
  }
  const models: ProviderModelsPayload = {
    ...base,
    ...(base.curated !== undefined || model.length > 0 ? { curated } : {}),
    ...(model.length > 0 ? { default: model } : {}),
  };
  return models;
}

// buildOnboardingProviderRequest produces a read-modify-write provider settings payload that
// persists the chosen default model + reasoning and the selected auth mode without dropping
// existing provider config. The API key (if any) is only ever sent as a vault-backed secret.
export function buildOnboardingProviderRequest(
  current: ProviderSettings,
  inputs: ProviderRequestInputs
): SettingsProviderRequest {
  const settings: ProviderSettings = {
    ...current,
    models: buildProviderModels(current, inputs.model),
    auth_mode: inputs.authMode,
  };
  const request: SettingsProviderRequest = { settings };
  if (inputs.model.length > 0 && inputs.reasoning !== "") {
    request.model_curation = {
      model_id: inputs.model,
      default_effort: inputs.reasoning,
    };
  }

  if (inputs.authMode !== "bound_secret") {
    delete settings.credential_slots;
    return request;
  }

  const targetEnv = inputs.envVar.trim() || existingApiKeyTargetEnv(current);
  if (targetEnv.length === 0) {
    throw new Error("Enter the environment variable the provider expects.");
  }
  const hasKey = inputs.apiKey.trim().length > 0;
  const secretRef = hasKey ? `vault:providers/${inputs.provider}/api_key` : `env:${targetEnv}`;
  settings.credential_slots = [
    {
      name: "api_key",
      target_env: targetEnv,
      secret_ref: secretRef,
      kind: "api_key",
      required: true,
    },
  ];
  if (!hasKey) {
    return request;
  }
  request.secrets = [
    { name: "api_key", secret_ref: secretRef, kind: "api_key", value: inputs.apiKey.trim() },
  ];
  return request;
}
