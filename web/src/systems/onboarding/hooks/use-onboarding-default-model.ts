import { isReasoningEffort, type ReasoningEffort } from "@/lib/api-contract";
import {
  providerNeedsAuth,
  useRuntimeModelCatalog,
  type RuntimeCatalogProvider,
} from "@/systems/model-catalog";
import { useProviders } from "@/systems/providers";
import {
  reasoningEffortLabel,
  type RuntimeModelOption,
  type RuntimeProviderOption,
  type RuntimeSelectorValue,
} from "@/systems/runtime";
import {
  useSettingsGeneral,
  useSettingsProvider,
  useUpdateSettingsGeneral,
  usePutSettingsProvider,
} from "@/systems/settings";

import { onboardingModelFacts, type OnboardingModelFact } from "../lib/model-facts";
import { buildOnboardingProviderRequest } from "../lib/provider-request";
import {
  useOnboardingDraftStore,
  type OnboardingAuthMode,
} from "../stores/use-onboarding-draft-store";

export interface OnboardingDefaultModelApi {
  providersLoading: boolean;
  providersError: string | null;
  runtimeValue: RuntimeSelectorValue;
  runtimeProviders: RuntimeProviderOption[];
  runtimeModels: RuntimeModelOption[];
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
  catalogLoaded: boolean;
  catalogRefreshing: boolean;
  catalogError: string | null;
  /** The bound-secret target env is still unknown — drives the field's invalid state. */
  missingEnvVar: boolean;
  configurationError: string | null;
  isValid: boolean;
  isCommitting: boolean;
  onRuntimeChange: (next: RuntimeSelectorValue) => void;
  onRefreshCatalog: () => void;
  onAuthModeChange: (mode: OnboardingAuthMode) => void;
  onEnvVarChange: (envVar: string) => void;
  onApiKeyChange: (apiKey: string) => void;
  commit: () => Promise<void>;
}

function describeError(fallback: string, error: unknown): string {
  if (error instanceof Error && error.message.trim().length > 0) {
    return error.message;
  }
  return fallback;
}

function normalizeEffort(effort: string): ReasoningEffort | "" {
  return effort === "" ? "" : isReasoningEffort(effort) ? effort : "";
}

/**
 * Providers reached through a key-bound harness (`pi_acp`) have no CLI sign-in
 * to reuse, so the API-key mode is their honest default; every other harness
 * spawns a CLI that already carries its own session.
 */
export function defaultAuthModeForHarness(harness: string | null): OnboardingAuthMode {
  return harness?.trim().toLowerCase() === "pi_acp" ? "bound_secret" : "native_cli";
}

function updateRuntime(next: RuntimeSelectorValue): void {
  const store = useOnboardingDraftStore.getState();
  const providerChanged = next.provider !== store.provider;
  store.patch({
    provider: next.provider,
    model: next.model,
    reasoning: normalizeEffort(next.reasoning_effort),
    // A new provider re-arms the harness default and drops credentials bound to
    // the previous one.
    ...(providerChanged ? { envVar: "", apiKey: "", authModeTouched: false } : {}),
  });
}

function updateAuthMode(authMode: OnboardingAuthMode): void {
  useOnboardingDraftStore
    .getState()
    .patch(
      authMode === "native_cli"
        ? { authMode, authModeTouched: true, envVar: "", apiKey: "" }
        : { authMode, authModeTouched: true }
    );
}

function updateEnvVar(envVar: string): void {
  useOnboardingDraftStore.getState().patch({ envVar });
}

function updateApiKey(apiKey: string): void {
  useOnboardingDraftStore.getState().patch({ apiKey });
}

export function useOnboardingDefaultModel(): OnboardingDefaultModelApi {
  const draft = useOnboardingDraftStore();
  const providersQuery = useProviders();
  const providers = providersQuery.data?.providers ?? [];

  const provider = draft.provider;

  const generalQuery = useSettingsGeneral();
  const providerDetailQuery = useSettingsProvider(provider, { enabled: provider.length > 0 });
  const updateGeneral = useUpdateSettingsGeneral();
  const putProvider = usePutSettingsProvider();

  const existingApiKeyTargetEnv =
    providerDetailQuery.data?.settings.credential_slots
      ?.find(slot => slot.name === "api_key")
      ?.target_env.trim() ?? "";

  const harness = providerDetailQuery.data?.settings.harness?.trim() || null;
  // Derived, never an effect: the harness default holds until the operator picks.
  const authMode = draft.authModeTouched ? draft.authMode : defaultAuthModeForHarness(harness);

  const runtimeProviders: RuntimeProviderOption[] = providers.map(entry => ({
    id: entry.name,
    name: entry.display_name?.trim() || entry.name,
    runtime_provider: entry.name,
    needs_auth: providerNeedsAuth(entry.auth_status?.state),
  }));

  // Onboarding lets the operator browse every configured provider's catalog via
  // the single aggregate query, filtered to the configured providers.
  const catalogProviders: RuntimeCatalogProvider[] = runtimeProviders.map(entry => ({
    id: entry.id,
    needsAuth: entry.needs_auth,
  }));
  const catalog = useRuntimeModelCatalog(catalogProviders, { enabled: true });
  const runtimeModels = catalog.models;

  const runtimeValue: RuntimeSelectorValue = {
    provider: draft.provider,
    model: draft.model,
    reasoning_effort: draft.reasoning,
  };

  const selectedModel = runtimeModels.find(
    entry => entry.provider === draft.provider && entry.id === draft.model
  );
  const providerName =
    runtimeProviders.find(entry => entry.id === draft.provider)?.name ?? draft.provider;
  const modelName = selectedModel?.name ?? draft.model;
  // `none` is not a selectable stop, so a model advertising only it has no levels.
  const selectableEfforts = selectedModel?.efforts.filter(effort => effort !== "none") ?? [];
  const reasoningLabel =
    selectableEfforts.length === 0
      ? null
      : draft.reasoning === ""
        ? "Default effort"
        : reasoningEffortLabel(draft.reasoning);

  const onRefreshCatalog = catalog.refresh;

  const commit = async () => {
    const trimmedProvider = draft.provider.trim();
    if (trimmedProvider.length === 0) {
      throw new Error("Select a provider before continuing.");
    }
    const detail = providerDetailQuery.data;
    if (!detail) {
      throw new Error(
        providerDetailQuery.error
          ? describeError("Failed to load provider settings.", providerDetailQuery.error)
          : "Provider settings are still loading."
      );
    }
    const config = generalQuery.data?.config;
    if (!config) {
      throw new Error(
        generalQuery.error
          ? describeError("Failed to load general settings.", generalQuery.error)
          : "General settings are still loading."
      );
    }
    const body = buildOnboardingProviderRequest(detail.settings, {
      model: draft.model.trim(),
      reasoning: draft.reasoning,
      authMode,
      envVar: draft.envVar.trim(),
      apiKey: draft.apiKey.trim(),
      provider: trimmedProvider,
    });
    await putProvider.mutateAsync({ name: trimmedProvider, body });
    await updateGeneral.mutateAsync({
      config: { ...config, defaults: { ...config.defaults, provider: trimmedProvider } },
    });
  };

  const providerSettingsError =
    provider.length > 0 && providerDetailQuery.error
      ? describeError("Failed to load provider settings.", providerDetailQuery.error)
      : null;
  const generalSettingsError = generalQuery.error
    ? describeError("Failed to load general settings.", generalQuery.error)
    : null;
  const missingBoundSecretTarget =
    authMode === "bound_secret" &&
    draft.envVar.trim().length === 0 &&
    existingApiKeyTargetEnv.length === 0;
  // The semantic flag, not its sentence, is what the field's invalid state reads.
  const missingEnvVar =
    provider.length > 0 && providerDetailQuery.isSuccess && missingBoundSecretTarget;
  const credentialTargetError = missingEnvVar
    ? "Enter the environment variable the provider expects."
    : null;
  const configurationError =
    providerSettingsError ?? generalSettingsError ?? credentialTargetError ?? null;
  const canCommit =
    provider.trim().length > 0 &&
    providerDetailQuery.isSuccess &&
    generalQuery.isSuccess &&
    !missingBoundSecretTarget;

  return {
    providersLoading: providersQuery.isLoading,
    providersError: providersQuery.error
      ? describeError("Failed to load providers.", providersQuery.error)
      : null,
    runtimeValue,
    runtimeProviders,
    runtimeModels,
    harness,
    providerName,
    modelName,
    reasoningLabel,
    facts: onboardingModelFacts(selectedModel, harness),
    authMode,
    envVar: draft.envVar,
    apiKey: draft.apiKey,
    catalogLoading: catalog.loading,
    catalogLoaded: catalog.loaded,
    catalogRefreshing: catalog.refreshing,
    catalogError: catalog.error,
    missingEnvVar,
    configurationError,
    isValid: canCommit,
    isCommitting: putProvider.isPending || updateGeneral.isPending,
    onRuntimeChange: updateRuntime,
    onRefreshCatalog,
    onAuthModeChange: updateAuthMode,
    onEnvVarChange: updateEnvVar,
    onApiKeyChange: updateApiKey,
    commit,
  };
}
