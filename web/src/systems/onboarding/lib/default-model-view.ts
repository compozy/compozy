import type {
  OnboardingAuthMode,
  OnboardingDraftState,
} from "../stores/use-onboarding-draft-store";
import { providerNeedsAuth } from "@/systems/model-catalog";
import type { ProviderSummary } from "@/systems/providers";
import {
  reasoningEffortLabel,
  type RuntimeModelOption,
  type RuntimeProviderOption,
} from "@/systems/runtime";

export function describeError(fallback: string, error: unknown): string {
  if (error instanceof Error && error.message.trim().length > 0) {
    return error.message;
  }
  return fallback;
}

/**
 * Providers reached through a key-bound harness (`pi_acp`) have no CLI sign-in
 * to reuse, so the API-key mode is their honest default; every other harness
 * spawns a CLI that already carries its own session.
 */
export function defaultAuthModeForHarness(harness: string | null): OnboardingAuthMode {
  return harness?.trim().toLowerCase() === "pi_acp" ? "bound_secret" : "native_cli";
}

export function onboardingRuntimeProviders(providers: ProviderSummary[]): RuntimeProviderOption[] {
  return providers.map(entry => ({
    id: entry.name,
    name: entry.display_name?.trim() || entry.name,
    runtime_provider: entry.name,
    runtime_strategy: entry.runtime_strategy,
    needs_auth: providerNeedsAuth(entry.auth_status?.state),
  }));
}

type OnboardingRuntimeDraft = Pick<OnboardingDraftState, "provider" | "model" | "reasoning">;

export interface OnboardingModelSelection {
  selectedModel: RuntimeModelOption | undefined;
  providerName: string;
  modelName: string;
  reasoningLabel: string | null;
}

function reasoningLabelFor(
  selectedModel: RuntimeModelOption | undefined,
  reasoning: OnboardingRuntimeDraft["reasoning"]
): string | null {
  // `none` is not a selectable stop, so a model advertising only it has no levels.
  const selectableEfforts = selectedModel?.efforts.filter(effort => effort !== "none") ?? [];
  if (selectableEfforts.length === 0) return null;
  return reasoning === "" ? "Default effort" : reasoningEffortLabel(reasoning);
}

/** Display names and effort label for the draft's provider/model pick. */
export function onboardingModelSelection(
  draft: OnboardingRuntimeDraft,
  runtimeProviders: RuntimeProviderOption[],
  runtimeModels: RuntimeModelOption[]
): OnboardingModelSelection {
  const selectedModel = runtimeModels.find(
    entry => entry.provider === draft.provider && entry.id === draft.model
  );
  const providerName =
    runtimeProviders.find(entry => entry.id === draft.provider)?.name ?? draft.provider;
  return {
    selectedModel,
    providerName,
    modelName: selectedModel?.name ?? draft.model,
    reasoningLabel: reasoningLabelFor(selectedModel, draft.reasoning),
  };
}

export interface OnboardingSettingsLoad {
  isSuccess: boolean;
  error: unknown;
}

export interface OnboardingConfigurationInput {
  provider: string;
  authMode: OnboardingAuthMode;
  envVar: string;
  /** Target env already bound on the provider's `api_key` slot, or "". */
  apiKeyTargetEnv: string;
  providerDetail: OnboardingSettingsLoad;
  persona: OnboardingSettingsLoad;
}

export interface OnboardingConfigurationState {
  /** The bound-secret target env is still unknown — drives the field's invalid state. */
  missingEnvVar: boolean;
  configurationError: string | null;
  canCommit: boolean;
}

function loadError(load: OnboardingSettingsLoad, fallback: string): string | null {
  return load.error ? describeError(fallback, load.error) : null;
}

/** Validity of the default-model step: provider settings, profile defaults, and credential target. */
export function onboardingConfigurationState(
  input: OnboardingConfigurationInput
): OnboardingConfigurationState {
  const { provider, providerDetail, persona } = input;
  const hasProvider = provider.length > 0;
  const missingBoundSecretTarget =
    input.authMode === "bound_secret" &&
    input.envVar.trim().length === 0 &&
    input.apiKeyTargetEnv.length === 0;
  // The semantic flag, not its sentence, is what the field's invalid state reads.
  const missingEnvVar = hasProvider && providerDetail.isSuccess && missingBoundSecretTarget;
  const providerSettingsError = hasProvider
    ? loadError(providerDetail, "Failed to load provider settings.")
    : null;
  const credentialTargetError = missingEnvVar
    ? "Enter the environment variable the provider expects."
    : null;
  return {
    missingEnvVar,
    configurationError:
      providerSettingsError ??
      loadError(persona, "Failed to load profile defaults.") ??
      credentialTargetError,
    canCommit:
      provider.trim().length > 0 &&
      providerDetail.isSuccess &&
      persona.isSuccess &&
      !missingBoundSecretTarget,
  };
}

/** Returns loaded settings data, or throws the load failure / still-loading sentence. */
export function requireLoaded<T>(
  data: T | undefined,
  error: unknown,
  messages: { failed: string; loading: string }
): T {
  if (data) return data;
  throw new Error(error ? describeError(messages.failed, error) : messages.loading);
}
