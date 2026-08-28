import { createStore } from "@xstate/store";
import { persist } from "@xstate/store/persist";

import type { ReasoningEffort, RuntimeSpeed } from "@/lib/api-contract";

export type OnboardingAuthMode = "native_cli" | "bound_secret";

export interface OnboardingWorkspaceDraft {
  path: string;
  name: string;
  workspaceId?: string;
}

export interface OnboardingDraftState {
  step: number;
  maxStep: number;
  provider: string;
  model: string;
  reasoning: ReasoningEffort | "";
  speed: RuntimeSpeed;
  authMode: OnboardingAuthMode;
  /**
   * False while `authMode` is still the harness-derived default, true once the
   * operator picks a mode. Changing provider re-arms the default by clearing it,
   * which keeps the effective mode derived state instead of an effect.
   */
  authModeTouched: boolean;
  envVar: string;
  /** Memory-only secret; the persist extension deliberately never receives it. */
  apiKey: string;
  workspaces: OnboardingWorkspaceDraft[];
}

const initialDraft: OnboardingDraftState = {
  step: 1,
  maxStep: 1,
  provider: "",
  model: "",
  reasoning: "",
  speed: "normal",
  authMode: "native_cli",
  authModeTouched: false,
  envVar: "",
  apiKey: "",
  workspaces: [],
};

/** Hard cut: v5 drafts are intentionally ignored rather than migrated. */
export const ONBOARDING_DRAFT_STORAGE_KEY = "compozy:onboarding:draft:v6";

export const onboardingDraftStore = createStore({
  context: initialDraft,
  on: {
    stepVisited: (context, event: { step: number }) => ({
      ...context,
      step: event.step,
      maxStep: Math.max(context.maxStep, event.step),
    }),
    runtimeSelected: (
      context,
      event: {
        provider: string;
        model: string;
        reasoning: ReasoningEffort | "";
        normalizedSpeed?: RuntimeSpeed;
      }
    ) => {
      const providerChanged = event.provider !== context.provider;
      return {
        ...context,
        provider: event.provider,
        model: event.model,
        reasoning: event.reasoning,
        ...(event.normalizedSpeed ? { speed: event.normalizedSpeed } : {}),
        ...(providerChanged ? { envVar: "", apiKey: "", authModeTouched: false } : {}),
      };
    },
    speedSelected: (context, event: { speed: RuntimeSpeed }) => ({
      ...context,
      speed: event.speed,
    }),
    authModeChosen: (context, event: { authMode: OnboardingAuthMode }) => ({
      ...context,
      authMode: event.authMode,
      authModeTouched: true,
      ...(event.authMode === "native_cli" ? { envVar: "", apiKey: "" } : {}),
    }),
    envVarEntered: (context, event: { envVar: string }) => ({ ...context, envVar: event.envVar }),
    apiKeyEntered: (context, event: { apiKey: string }) => ({ ...context, apiKey: event.apiKey }),
    workspaceDraftAdded: (context, event: { workspace: OnboardingWorkspaceDraft }) => {
      if (context.workspaces.some(workspace => workspace.path === event.workspace.path)) {
        return context;
      }
      return { ...context, workspaces: [...context.workspaces, event.workspace] };
    },
    workspaceDraftRemoved: (context, event: { path: string }) => ({
      ...context,
      workspaces: context.workspaces.filter(workspace => workspace.path !== event.path),
    }),
    draftCleared: () => initialDraft,
  },
}).with(
  persist({
    name: ONBOARDING_DRAFT_STORAGE_KEY,
    // The API key is intentionally absent, rather than persisted as an empty
    // value, so a stored draft cannot contain a secret at all.
    pick: context => ({
      step: context.step,
      maxStep: context.maxStep,
      provider: context.provider,
      model: context.model,
      reasoning: context.reasoning,
      speed: context.speed,
      authMode: context.authMode,
      authModeTouched: context.authModeTouched,
      envVar: context.envVar,
      workspaces: context.workspaces,
    }),
  })
);
