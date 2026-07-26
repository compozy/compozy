import { create } from "zustand";
import { createJSONStorage, persist } from "zustand/middleware";

import type { ReasoningEffort } from "@/lib/api-contract";

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
  authMode: OnboardingAuthMode;
  /**
   * False while `authMode` is still the harness-derived default, true once the
   * operator picks a mode. Changing provider re-arms the default by clearing it,
   * which keeps the effective mode derived state instead of an effect.
   */
  authModeTouched: boolean;
  envVar: string;
  apiKey: string;
  workspaces: OnboardingWorkspaceDraft[];
}

export interface OnboardingDraftStore extends OnboardingDraftState {
  setStep: (step: number) => void;
  patch: (patch: Partial<OnboardingDraftState>) => void;
  addWorkspace: (workspace: OnboardingWorkspaceDraft) => void;
  removeWorkspace: (path: string) => void;
  reset: () => void;
}

const initialState: OnboardingDraftState = {
  step: 1,
  maxStep: 1,
  provider: "",
  model: "",
  reasoning: "",
  authMode: "native_cli",
  authModeTouched: false,
  envVar: "",
  apiKey: "",
  workspaces: [],
};

/** Bumped on every draft-shape change; drafts are never migrated. */
export const ONBOARDING_DRAFT_STORAGE_KEY = "agh:onboarding:draft:v4";

const draftStorage = createJSONStorage<OnboardingDraftState>(() => {
  if (typeof window === "undefined") {
    throw new Error("localStorage is unavailable");
  }
  return window.localStorage;
});

export const useOnboardingDraftStore = create<OnboardingDraftStore>()(
  persist(
    set => ({
      ...initialState,
      setStep: step => set(state => ({ step, maxStep: Math.max(state.maxStep, step) })),
      patch: patch => set(patch),
      addWorkspace: workspace =>
        set(state =>
          state.workspaces.some(item => item.path === workspace.path)
            ? state
            : { workspaces: [...state.workspaces, workspace] }
        ),
      removeWorkspace: path =>
        set(state => ({ workspaces: state.workspaces.filter(item => item.path !== path) })),
      reset: () => set({ ...initialState }),
    }),
    {
      name: ONBOARDING_DRAFT_STORAGE_KEY,
      storage: draftStorage,
      // The API key is intentionally NOT persisted — it lives only in memory.
      partialize: state => ({
        step: state.step,
        maxStep: state.maxStep,
        provider: state.provider,
        model: state.model,
        reasoning: state.reasoning,
        authMode: state.authMode,
        authModeTouched: state.authModeTouched,
        envVar: state.envVar,
        apiKey: "",
        workspaces: state.workspaces,
      }),
    }
  )
);
