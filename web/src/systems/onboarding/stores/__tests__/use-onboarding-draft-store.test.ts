import { beforeEach, describe, expect, it } from "vitest";

import {
  ONBOARDING_DRAFT_STORAGE_KEY,
  useOnboardingDraftStore,
} from "../use-onboarding-draft-store";

function persistedState(): Record<string, unknown> {
  const raw = window.localStorage.getItem(ONBOARDING_DRAFT_STORAGE_KEY);
  if (!raw) {
    return {};
  }
  return (JSON.parse(raw).state ?? {}) as Record<string, unknown>;
}

describe("useOnboardingDraftStore", () => {
  beforeEach(() => {
    window.localStorage.clear();
    useOnboardingDraftStore.getState().reset();
  });

  it("grows maxStep but never shrinks it as the user moves between steps", () => {
    const store = useOnboardingDraftStore.getState();
    store.setStep(2);
    expect(useOnboardingDraftStore.getState().maxStep).toBe(2);
    store.setStep(1);
    expect(useOnboardingDraftStore.getState().step).toBe(1);
    expect(useOnboardingDraftStore.getState().maxStep).toBe(2);
    store.setStep(2);
    expect(useOnboardingDraftStore.getState().step).toBe(2);
    expect(useOnboardingDraftStore.getState().maxStep).toBe(2);
  });

  it("adds workspaces without duplicating the same path", () => {
    const store = useOnboardingDraftStore.getState();
    store.addWorkspace({ path: "/a", name: "a" });
    store.addWorkspace({ path: "/a", name: "a-again" });
    store.addWorkspace({ path: "/b", name: "b" });
    expect(useOnboardingDraftStore.getState().workspaces).toEqual([
      { path: "/a", name: "a" },
      { path: "/b", name: "b" },
    ]);
  });

  it("removes a workspace by path", () => {
    const store = useOnboardingDraftStore.getState();
    store.addWorkspace({ path: "/a", name: "a" });
    store.addWorkspace({ path: "/b", name: "b" });
    store.removeWorkspace("/a");
    expect(useOnboardingDraftStore.getState().workspaces).toEqual([{ path: "/b", name: "b" }]);
  });

  it("never persists the API key to local storage", () => {
    useOnboardingDraftStore.getState().patch({ apiKey: "sk-super-secret", provider: "claude" });
    const persisted = persistedState();
    expect(persisted.provider).toBe("claude");
    expect(persisted.apiKey).toBe("");
    expect(useOnboardingDraftStore.getState().apiKey).toBe("sk-super-secret");
  });

  it("persists whether the operator has overridden the harness auth default", () => {
    expect(useOnboardingDraftStore.getState().authModeTouched).toBe(false);

    useOnboardingDraftStore.getState().patch({ authMode: "bound_secret", authModeTouched: true });

    expect(persistedState().authModeTouched).toBe(true);
    expect(persistedState().authMode).toBe("bound_secret");
  });

  it("resets to the initial draft", () => {
    const store = useOnboardingDraftStore.getState();
    store.patch({ provider: "claude", model: "opus", apiKey: "x", authModeTouched: true });
    store.addWorkspace({ path: "/a", name: "a" });
    store.reset();
    const state = useOnboardingDraftStore.getState();
    expect(state.provider).toBe("");
    expect(state.model).toBe("");
    expect(state.apiKey).toBe("");
    expect(state.authMode).toBe("native_cli");
    expect(state.authModeTouched).toBe(false);
    expect(state.workspaces).toEqual([]);
    expect(state.step).toBe(1);
  });
});
