import type { OnboardingDefaultModelApi } from "../hooks/use-onboarding-default-model";
import type { OnboardingWorkspacesApi } from "../hooks/use-onboarding-workspaces";
import { onboardingStepMeta, type OnboardingWizardApi } from "../hooks/use-onboarding-wizard";
import { onboardingModelFacts } from "../lib/model-facts";

const noop = () => {};

export const onboardingDefaultModelFixture: OnboardingDefaultModelApi = {
  providersLoading: false,
  providersError: null,
  runtimeValue: { provider: "claude", model: "claude-opus-4-8", reasoning_effort: "high" },
  runtimeProviders: [
    { id: "claude", name: "Claude Code", harness: "acp", runtime_provider: "claude" },
    { id: "codex", name: "Codex", harness: "acp", runtime_provider: "codex" },
    { id: "gemini", name: "Gemini CLI", runtime_provider: "gemini" },
    { id: "openrouter", name: "OpenRouter", harness: "pi_acp", runtime_provider: "openrouter" },
  ],
  runtimeModels: [
    {
      id: "claude-opus-4-8",
      provider: "claude",
      name: "Claude Opus 4.8",
      efforts: ["low", "medium", "high", "xhigh", "max"],
      availability: "live",
      curated: true,
      context_window: 1_000_000,
      cost_input: 5,
      cost_output: 25,
      supports_tools: true,
      reasoning_source: "catalog",
    },
    {
      id: "claude-sonnet-5",
      provider: "claude",
      name: "Claude Sonnet 5",
      efforts: ["low", "medium", "high", "xhigh", "max"],
      availability: "live",
      curated: true,
      featured: true,
      context_window: 1_000_000,
      cost_input: 3,
      cost_output: 15,
      supports_tools: true,
      reasoning_source: "catalog",
    },
  ],
  harness: "acp",
  providerName: "Claude Code",
  modelName: "Claude Opus 4.8",
  reasoningLabel: "High",
  facts: [],
  authMode: "native_cli",
  envVar: "",
  apiKey: "",
  catalogLoading: false,
  catalogLoaded: true,
  catalogRefreshing: false,
  catalogError: null,
  missingEnvVar: false,
  configurationError: null,
  isValid: true,
  isCommitting: false,
  onRuntimeChange: noop,
  onRefreshCatalog: noop,
  onAuthModeChange: noop,
  onEnvVarChange: noop,
  onApiKeyChange: noop,
  commit: async () => {},
};

export const onboardingWorkspacesFixture: OnboardingWorkspacesApi = {
  currentPath: "/Users/operator/Dev",
  parent: "/Users/operator",
  home: "/Users/operator",
  entries: [
    { name: "compozy", path: "/Users/operator/Dev/compozy", is_dir: true },
    { name: "infra", path: "/Users/operator/Dev/infra", is_dir: true },
    { name: "notes", path: "/Users/operator/Dev/notes", is_dir: true },
    { name: "README.md", path: "/Users/operator/Dev/README.md", is_dir: false },
  ],
  isBrowsing: false,
  browseError: null,
  workspaces: [{ path: "/Users/operator/Dev/compozy", name: "compozy" }],
  isResolving: false,
  isRemoving: false,
  isCatalogLoading: false,
  catalogError: null,
  resolveError: null,
  navigateTo: noop,
  goToParent: noop,
  goHome: noop,
  addWorkspace: async () => {},
  removeWorkspace: async () => {},
  isAdded: (path: string) => path === "/Users/operator/Dev/compozy",
  reloadCatalog: async () => {},
};

export interface OnboardingWizardFixtureOverrides {
  wizard?: Partial<Omit<OnboardingWizardApi, "defaultModel" | "workspaces" | "meta">>;
  defaultModel?: Partial<OnboardingDefaultModelApi>;
  workspaces?: Partial<OnboardingWorkspacesApi>;
}

/**
 * A complete `OnboardingWizardApi` for stories and component tests. `meta`,
 * `canContinue`, `isLastStep` and the model facts are derived so a fixture can
 * never contradict the real wizard — override `step` and the rest follows.
 */
export function onboardingWizardFixture(
  overrides: OnboardingWizardFixtureOverrides = {}
): OnboardingWizardApi {
  const defaultModel = { ...onboardingDefaultModelFixture, ...overrides.defaultModel };
  defaultModel.facts =
    overrides.defaultModel?.facts ??
    onboardingModelFacts(
      defaultModel.runtimeModels.find(
        model =>
          model.provider === defaultModel.runtimeValue.provider &&
          model.id === defaultModel.runtimeValue.model
      ),
      defaultModel.harness
    );
  const workspaces = { ...onboardingWorkspacesFixture, ...overrides.workspaces };
  const step = overrides.wizard?.step ?? 1;
  return {
    step,
    maxStep: overrides.wizard?.maxStep ?? step,
    meta: onboardingStepMeta(step),
    defaultModel,
    workspaces,
    canContinue:
      overrides.wizard?.canContinue ??
      (step === 1 ? defaultModel.isValid : workspaces.workspaces.length > 0),
    isLastStep: overrides.wizard?.isLastStep ?? step === 2,
    isBusy: overrides.wizard?.isBusy ?? false,
    commitError: overrides.wizard?.commitError ?? null,
    goToStep: overrides.wizard?.goToStep ?? noop,
    back: overrides.wizard?.back ?? noop,
    next: overrides.wizard?.next ?? (async () => {}),
  };
}
