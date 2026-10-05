// Suite: first-run onboarding progression
// Invariant: onboarding completes without creating a session; explicit Skip selects Global only
// after persistence succeeds, while normal Finish retains the selected project.
// Boundary IN: onboarding wizard orchestration and draft reset.
// Boundary OUT: provider/workspace adapters and completion persistence.
import { act, renderHook } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { onboardingDraftStore } from "../../stores/use-onboarding-draft-store";
import { ONBOARDING_STEP_COUNT, useOnboardingWizard } from "../use-onboarding-wizard";
import {
  activeWorkspaceStore,
  clearActiveWorkspaceSelection,
  setActiveWorkspaceId,
} from "@/systems/workspace/stores/active-workspace-store";

const mocks = vi.hoisted(() => ({
  commitDefaultModel: vi.fn(),
  completeOnboarding: vi.fn(),
  isRemoving: false,
  isResolving: false,
}));

vi.mock("../use-onboarding-default-model", () => ({
  useOnboardingDefaultModel: () => ({
    isValid: true,
    isCommitting: false,
    commit: mocks.commitDefaultModel,
  }),
}));

vi.mock("../use-onboarding-workspaces", () => ({
  useOnboardingWorkspaces: () => ({
    workspaces: [{ path: "/workspace", name: "workspace", workspaceId: "ws_main" }],
    isRemoving: mocks.isRemoving,
    isResolving: mocks.isResolving,
  }),
}));

vi.mock("../use-complete-onboarding", () => ({
  useCompleteOnboarding: () => ({
    isPending: false,
    mutateAsync: mocks.completeOnboarding,
  }),
}));

describe("useOnboardingWizard", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mocks.commitDefaultModel.mockResolvedValue(undefined);
    mocks.completeOnboarding.mockResolvedValue(undefined);
    mocks.isRemoving = false;
    mocks.isResolving = false;
    onboardingDraftStore.trigger.draftCleared();
    clearActiveWorkspaceSelection();
  });

  it("Should complete directly from the workspace step", async () => {
    const onComplete = vi.fn();
    const { result } = renderHook(() => useOnboardingWizard(onComplete));

    expect(ONBOARDING_STEP_COUNT).toBe(2);
    await act(async () => {
      await result.current.next();
    });
    expect(result.current.step).toBe(2);
    expect(mocks.commitDefaultModel).toHaveBeenCalledOnce();

    await act(async () => {
      await result.current.next();
    });

    expect(mocks.completeOnboarding).toHaveBeenCalledOnce();
    expect(onComplete).toHaveBeenCalledOnce();
    expect(onboardingDraftStore.getSnapshot().context.step).toBe(1);
  });

  it("Should reject navigation beyond the two-step flow", () => {
    onboardingDraftStore.trigger.stepVisited({ step: 2 });
    const { result } = renderHook(() => useOnboardingWizard(vi.fn()));

    act(() => result.current.goToStep(3));

    expect(result.current.step).toBe(2);
    expect(result.current.maxStep).toBe(2);
  });

  it.each([
    { action: "next" as const, scope: "workspace" },
    { action: "skipToGlobal" as const, scope: "global" },
  ])("Should finish through $action in $scope while remembering the project", async testCase => {
    setActiveWorkspaceId("ws_main");
    onboardingDraftStore.trigger.stepVisited({ step: 2 });
    const onComplete = vi.fn();
    const { result } = renderHook(() => useOnboardingWizard(onComplete));

    await act(async () => {
      await result.current[testCase.action]();
    });

    expect(activeWorkspaceStore.getSnapshot().context).toMatchObject({
      scope: testCase.scope,
      selectedWorkspaceId: "ws_main",
    });
    expect(onComplete).toHaveBeenCalledOnce();
    expect(onboardingDraftStore.getSnapshot().context.step).toBe(1);
  });

  it("Should preserve the project and setup draft when Skip cannot persist completion", async () => {
    setActiveWorkspaceId("ws_main");
    onboardingDraftStore.trigger.stepVisited({ step: 2 });
    mocks.completeOnboarding.mockRejectedValueOnce(new Error("Daemon unavailable"));
    const onComplete = vi.fn();
    const { result } = renderHook(() => useOnboardingWizard(onComplete));

    await act(async () => {
      await result.current.skipToGlobal();
    });

    expect(result.current.commitError).toBe("Daemon unavailable");
    expect(activeWorkspaceStore.getSnapshot().context).toMatchObject({
      scope: "workspace",
      selectedWorkspaceId: "ws_main",
    });
    expect(onboardingDraftStore.getSnapshot().context.step).toBe(2);
    expect(onComplete).not.toHaveBeenCalled();
  });

  it.each(["isResolving", "isRemoving"] as const)(
    "Should block completion while a workspace operation reports %s",
    async busyFlag => {
      onboardingDraftStore.trigger.stepVisited({ step: 2 });
      mocks[busyFlag] = true;
      const { result } = renderHook(() => useOnboardingWizard(vi.fn()));

      expect(result.current.canContinue).toBe(false);
      expect(result.current.isBusy).toBe(true);
      await act(async () => {
        await result.current.next();
        await result.current.skipToGlobal();
      });

      expect(mocks.completeOnboarding).not.toHaveBeenCalled();
    }
  );
});
