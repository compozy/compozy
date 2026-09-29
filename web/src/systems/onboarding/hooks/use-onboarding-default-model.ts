import { isReasoningEffort, type ReasoningEffort, type RuntimeSpeed } from "@/lib/api-contract";
import { useSelector } from "@xstate/store-react";

import {
  defaultAuthModeForHarness,
  describeError,
  onboardingConfigurationState,
  onboardingModelSelection,
  onboardingRuntimeProviders,
  requireLoaded,
} from "../lib/default-model-view";
import { onboardingModelFacts, type OnboardingModelFact } from "../lib/model-facts";
import { buildOnboardingProviderRequest, existingApiKeyTargetEnv } from "../lib/provider-request";
import {
  onboardingDraftStore,
  type OnboardingAuthMode,
} from "../stores/use-onboarding-draft-store";
import { type RuntimeCatalogProvider, useRuntimeModelCatalog } from "@/systems/model-catalog";
import { useProviders } from "@/systems/providers";
import {
  type RuntimeModelOption,
  type RuntimeProviderOption,
  type RuntimeSelectorValue,
} from "@/systems/runtime";
import {
  usePutSettingsProvider,
  useSettingsPersona,
  useSettingsProvider,
  useUpdateSettingsPersona,
} from "@/systems/settings";

const ONBOARDING_PERSONA_FILTER = { scope: "user" } as const;

export interface OnboardingDefaultModelApi {
  providersLoading: boolean;
  providersError: string | null;
  runtimeValue: RuntimeSelectorValue;
  runtimeProviders: RuntimeProviderOption[];
  runtimeModels: RuntimeModelOption[];
  speed: RuntimeSpeed;
  /** Harness the selected provider runs on (`acp`, `pi_acp`, …), null until loaded. */
  harness: string | null;
  /** Display name of the selected provider; empty until one is picked. */
  providerName: string;
  /** Display name of the selected model; empty until one is picked. */
  modelName: string;
  /** Effort label when the model exposes selectable levels, else null. */
  reasoningLabel: string | null;
  /** Facts the runtime reports about the selected model, in reading order. */
  facts: OnboardingModelFact[];
  /** Effective mode: the harness default until the operator picks one. */
  authMode: OnboardingAuthMode;
  envVar: string;
  apiKey: string;
  catalogLoading: boolean;
  catalogRefreshing: boolean;
  catalogError: string | null;
  /** The bound-secret target env is still unknown — drives the field's invalid state. */
  missingEnvVar: boolean;
  configurationError: string | null;
  isValid: boolean;
  isCommitting: boolean;
  onRuntimeChange: (next: RuntimeSelectorValue, normalizedSpeed?: RuntimeSpeed) => void;
  onSpeedChange: (speed: RuntimeSpeed) => void;
  onRefreshCatalog: () => void;
  onAuthModeChange: (mode: OnboardingAuthMode) => void;
  onEnvVarChange: (envVar: string) => void;
  onApiKeyChange: (apiKey: string) => void;
  commit: () => Promise<void>;
}

function normalizeEffort(effort: string): ReasoningEffort | "" {
  return effort === "" ? "" : isReasoningEffort(effort) ? effort : "";
}

function updateRuntime(next: RuntimeSelectorValue, normalizedSpeed?: RuntimeSpeed): void {
  onboardingDraftStore.trigger.runtimeSelected({
    provider: next.provider,
    model: next.model,
    reasoning: normalizeEffort(next.reasoning_effort),
    ...(normalizedSpeed ? { normalizedSpeed } : {}),
  });
}

function updateSpeed(speed: RuntimeSpeed): void {
  onboardingDraftStore.trigger.speedSelected({ speed });
}

function updateAuthMode(authMode: OnboardingAuthMode): void {
  onboardingDraftStore.trigger.authModeChosen({ authMode });
}

function updateEnvVar(envVar: string): void {
  onboardingDraftStore.trigger.envVarEntered({ envVar });
}

function updateApiKey(apiKey: string): void {
  onboardingDraftStore.trigger.apiKeyEntered({ apiKey });
}

export function useOnboardingDefaultModel(): OnboardingDefaultModelApi {
  const draft = useSelector(onboardingDraftStore, state => state.context);
  const providersQuery = useProviders();
  const provider = draft.provider;

  const personaQuery = useSettingsPersona(ONBOARDING_PERSONA_FILTER);
  const providerDetailQuery = useSettingsProvider(provider, { enabled: provider.length > 0 });
  const updatePersona = useUpdateSettingsPersona();
  const putProvider = usePutSettingsProvider();

  const providerSettings = providerDetailQuery.data?.settings;
  const harness = providerSettings?.harness?.trim() || null;
  // Derived, never an effect: the harness default holds until the operator picks.
  const authMode = draft.authModeTouched ? draft.authMode : defaultAuthModeForHarness(harness);

  const runtimeProviders = onboardingRuntimeProviders(providersQuery.data?.providers ?? []);
  // Onboarding lets the operator browse every configured provider's catalog via
  // the single aggregate query, filtered to the configured providers.
  const catalogProviders: RuntimeCatalogProvider[] = runtimeProviders.map(entry => ({
    id: entry.id,
    needsAuth: entry.needs_auth,
  }));
  const catalog = useRuntimeModelCatalog(catalogProviders, { enabled: true });
  const runtimeModels = catalog.models;
  const selection = onboardingModelSelection(draft, runtimeProviders, runtimeModels);

  const configuration = onboardingConfigurationState({
    provider,
    authMode,
    envVar: draft.envVar,
    apiKeyTargetEnv: providerSettings ? existingApiKeyTargetEnv(providerSettings) : "",
    providerDetail: providerDetailQuery,
    persona: personaQuery,
  });

  const commit = async () => {
    const trimmedProvider = draft.provider.trim();
    if (trimmedProvider.length === 0) {
      throw new Error("Select a provider before continuing.");
    }
    const detail = requireLoaded(providerDetailQuery.data, providerDetailQuery.error, {
      failed: "Failed to load provider settings.",
      loading: "Provider settings are still loading.",
    });
    const config = requireLoaded(personaQuery.data?.config, personaQuery.error, {
      failed: "Failed to load profile defaults.",
      loading: "Profile defaults are still loading.",
    });
    const body = buildOnboardingProviderRequest(detail.settings, {
      model: draft.model.trim(),
      reasoning: draft.reasoning,
      speed: draft.speed,
      authMode,
      envVar: draft.envVar.trim(),
      apiKey: draft.apiKey.trim(),
      provider: trimmedProvider,
    });
    await putProvider.mutateAsync({ name: trimmedProvider, body });
    await updatePersona.mutateAsync({
      body: { config: { ...config, provider: trimmedProvider } },
      filter: ONBOARDING_PERSONA_FILTER,
    });
  };

  return {
    providersLoading: providersQuery.isLoading,
    providersError: providersQuery.error
      ? describeError("Failed to load providers.", providersQuery.error)
      : null,
    runtimeValue: {
      provider: draft.provider,
      model: draft.model,
      reasoning_effort: draft.reasoning,
    },
    runtimeProviders,
    runtimeModels,
    speed: draft.speed,
    harness,
    providerName: selection.providerName,
    modelName: selection.modelName,
    reasoningLabel: selection.reasoningLabel,
    facts: onboardingModelFacts(selection.selectedModel),
    authMode,
    envVar: draft.envVar,
    apiKey: draft.apiKey,
    catalogLoading: catalog.loading,
    catalogRefreshing: catalog.refreshing,
    catalogError: catalog.error,
    missingEnvVar: configuration.missingEnvVar,
    configurationError: configuration.configurationError,
    isValid: configuration.canCommit,
    isCommitting: putProvider.isPending || updatePersona.isPending,
    onRuntimeChange: updateRuntime,
    onSpeedChange: updateSpeed,
    onRefreshCatalog: catalog.refresh,
    onAuthModeChange: updateAuthMode,
    onEnvVarChange: updateEnvVar,
    onApiKeyChange: updateApiKey,
    commit,
  };
}
