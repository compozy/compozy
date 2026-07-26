// Suite: first-run onboarding progression
// Invariant: onboarding completes after the runtime and workspace steps without creating a session.
// Boundary IN: onboarding wizard orchestration and draft reset.
// Boundary OUT: provider/workspace adapters and completion persistence.
import { act, renderHook } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { useOnboardingDraftStore } from "../../stores/use-onboarding-draft-store";
import { ONBOARDING_STEP_COUNT, useOnboardingWizard } from "../use-onboarding-wizard";

const mocks = vi.hoisted(() => ({
  commitDefaultModel: vi.fn(),
  completeOnboarding: vi.fn(),
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
    useOnboardingDraftStore.getState().reset();
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
    expect(useOnboardingDraftStore.getState().step).toBe(1);
  });

  it("Should reject navigation beyond the two-step flow", () => {
    useOnboardingDraftStore.getState().setStep(2);
    const { result } = renderHook(() => useOnboardingWizard(vi.fn()));

    act(() => result.current.goToStep(3));

    expect(result.current.step).toBe(2);
    expect(result.current.maxStep).toBe(2);
  });
});
