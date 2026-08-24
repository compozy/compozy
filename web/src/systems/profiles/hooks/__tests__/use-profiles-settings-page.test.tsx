import { renderHook } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const state = {
  tier: undefined as "local" | "remote" | undefined,
  profiles: { data: [], isLoading: false, error: null as Error | null, refetch: vi.fn() },
  selections: { data: [], isLoading: false, error: null as Error | null, refetch: vi.fn() },
  workspaces: { data: [], isLoading: false, error: null as Error | null, refetch: vi.fn() },
};

vi.mock("@/systems/gateway", () => ({
  useGatewayAccessTier: () => state.tier,
}));

vi.mock("@/systems/workspace", () => ({
  useWorkspaces: () => state.workspaces,
}));

vi.mock("../use-profile-lens", () => ({
  useProfileLens: () => ({ scope: "global" }),
}));

vi.mock("../use-profile-selection", () => ({
  useActiveProfileView: () => ({ kind: "profile", profile: "default" }),
}));

vi.mock("../use-profiles", () => ({
  useProfiles: () => state.profiles,
  useProfileSelectionMap: () => state.selections,
}));

import { useProfilesSettingsPage } from "../use-profiles-settings-page";

describe("useProfilesSettingsPage", () => {
  beforeEach(() => {
    state.tier = undefined;
    for (const query of [state.profiles, state.selections, state.workspaces]) {
      query.isLoading = false;
      query.error = null;
      query.refetch.mockReset();
    }
  });

  it("Should hide management controls until the gateway is known to be local", () => {
    const { result } = renderHook(() => useProfilesSettingsPage());
    expect(result.current.manageable).toBe(false);
  });

  it("Should expose management controls for the local gateway", () => {
    state.tier = "local";
    const { result } = renderHook(() => useProfilesSettingsPage());
    expect(result.current.manageable).toBe(true);
  });

  it("Should keep an explicitly remote gateway read-only", () => {
    state.tier = "remote";
    const { result } = renderHook(() => useProfilesSettingsPage());
    expect(result.current.manageable).toBe(false);
  });

  it("Should include selection and workspace queries in loading and error state", () => {
    state.selections.isLoading = true;
    const loading = renderHook(() => useProfilesSettingsPage());
    expect(loading.result.current.isLoading).toBe(true);
    loading.unmount();

    state.selections.isLoading = false;
    state.workspaces.error = new Error("Workspace catalog unavailable");
    const failed = renderHook(() => useProfilesSettingsPage());
    expect(failed.result.current.errorMessage).toBe("Workspace catalog unavailable");
  });

  it("Should retry profiles, selections, and workspace names together", () => {
    const { result } = renderHook(() => useProfilesSettingsPage());

    result.current.refetch();

    expect(state.profiles.refetch).toHaveBeenCalledOnce();
    expect(state.selections.refetch).toHaveBeenCalledOnce();
    expect(state.workspaces.refetch).toHaveBeenCalledOnce();
  });
});
