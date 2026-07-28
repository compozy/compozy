// Suite: onboarding draft store
// Invariant: draft transitions preserve monotonic progress, unique workspace paths, credential boundaries, and v5 persistence.
// Boundary IN: onboarding draft state and local-storage persistence.
// Boundary OUT: wizard/query orchestration, owned by the onboarding hook suites.
import { rehydrateStore } from "@xstate/store/persist";
import { beforeEach, describe, expect, it } from "vitest";

import { ONBOARDING_DRAFT_STORAGE_KEY, onboardingDraftStore } from "../use-onboarding-draft-store";

function persistedContext(): Record<string, unknown> {
  const raw = window.localStorage.getItem(ONBOARDING_DRAFT_STORAGE_KEY);
  if (!raw) {
    return {};
  }
  return (JSON.parse(raw).context ?? {}) as Record<string, unknown>;
}

function draft() {
  return onboardingDraftStore.getSnapshot().context;
}

describe("onboardingDraftStore", () => {
  beforeEach(() => {
    window.localStorage.clear();
    onboardingDraftStore.trigger.draftCleared();
  });

  it("grows maxStep but never shrinks it as the user moves between steps", () => {
    onboardingDraftStore.trigger.stepVisited({ step: 2 });
    expect(draft().maxStep).toBe(2);

    onboardingDraftStore.trigger.stepVisited({ step: 1 });
    expect(draft().step).toBe(1);
    expect(draft().maxStep).toBe(2);
  });

  it("adds workspace drafts without duplicating the same path", () => {
    onboardingDraftStore.trigger.workspaceDraftAdded({
      workspace: { path: "/a", name: "a" },
    });
    onboardingDraftStore.trigger.workspaceDraftAdded({
      workspace: { path: "/a", name: "a-again" },
    });
    onboardingDraftStore.trigger.workspaceDraftAdded({
      workspace: { path: "/b", name: "b" },
    });

    expect(draft().workspaces).toEqual([
      { path: "/a", name: "a" },
      { path: "/b", name: "b" },
    ]);
  });

  it("clears credentials and re-arms the auth default when the provider changes", () => {
    onboardingDraftStore.trigger.runtimeSelected({
      provider: "claude",
      model: "opus",
      reasoning: "high",
    });
    onboardingDraftStore.trigger.authModeChosen({ authMode: "bound_secret" });
    onboardingDraftStore.trigger.envVarEntered({ envVar: "ANTHROPIC_API_KEY" });
    onboardingDraftStore.trigger.apiKeyEntered({ apiKey: "sk-super-secret" });

    onboardingDraftStore.trigger.runtimeSelected({
      provider: "codex",
      model: "gpt-5.6-sol",
      reasoning: "",
    });

    expect(draft()).toMatchObject({
      provider: "codex",
      model: "gpt-5.6-sol",
      authModeTouched: false,
      envVar: "",
      apiKey: "",
    });
  });

  it("keeps API keys in memory and out of persisted drafts", () => {
    onboardingDraftStore.trigger.runtimeSelected({
      provider: "claude",
      model: "opus",
      reasoning: "",
    });
    onboardingDraftStore.trigger.apiKeyEntered({ apiKey: "sk-super-secret" });

    expect(draft().apiKey).toBe("sk-super-secret");
    expect(persistedContext().provider).toBe("claude");
    expect(persistedContext()).not.toHaveProperty("apiKey");
    expect(window.localStorage.getItem(ONBOARDING_DRAFT_STORAGE_KEY)).not.toContain(
      "sk-super-secret"
    );
  });

  it("round-trips a v5 draft without restoring a memory-only API key", async () => {
    onboardingDraftStore.trigger.stepVisited({ step: 2 });
    onboardingDraftStore.trigger.runtimeSelected({
      provider: "claude",
      model: "opus",
      reasoning: "high",
    });
    onboardingDraftStore.trigger.authModeChosen({ authMode: "bound_secret" });
    onboardingDraftStore.trigger.envVarEntered({ envVar: "ANTHROPIC_API_KEY" });
    onboardingDraftStore.trigger.apiKeyEntered({ apiKey: "sk-super-secret" });
    onboardingDraftStore.trigger.workspaceDraftAdded({
      workspace: { path: "/workspace", name: "workspace", workspaceId: "ws_main" },
    });
    const persistedDraft = window.localStorage.getItem(ONBOARDING_DRAFT_STORAGE_KEY);

    onboardingDraftStore.trigger.draftCleared();
    window.localStorage.setItem(ONBOARDING_DRAFT_STORAGE_KEY, persistedDraft ?? "");
    await rehydrateStore(onboardingDraftStore);

    expect(draft()).toMatchObject({
      step: 2,
      maxStep: 2,
      provider: "claude",
      model: "opus",
      reasoning: "high",
      authMode: "bound_secret",
      authModeTouched: true,
      envVar: "ANTHROPIC_API_KEY",
      apiKey: "",
      workspaces: [{ path: "/workspace", name: "workspace", workspaceId: "ws_main" }],
    });
  });
});
