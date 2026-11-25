import { renderHook } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const state = {
  profiles: { data: [], isLoading: false, error: null as Error | null, refetch: vi.fn() },
};

vi.mock("../use-profile-selection", () => ({
  useActiveProfileView: () => ({ kind: "profile", profile: "default" }),
}));

vi.mock("../use-profiles", () => ({
  useProfiles: () => state.profiles,
}));

import { useProfilesSettingsPage } from "../use-profiles-settings-page";

describe("useProfilesSettingsPage", () => {
  beforeEach(() => {
    for (const query of [state.profiles]) {
      query.isLoading = false;
      query.error = null;
      query.refetch.mockReset();
    }
  });

  it("Should expose profile management controls", () => {
    const { result } = renderHook(() => useProfilesSettingsPage());
    expect(result.current.manageable).toBe(true);
  });

  it("Should surface the profile list loading and error state", () => {
    state.profiles.isLoading = true;
    const loading = renderHook(() => useProfilesSettingsPage());
    expect(loading.result.current.isLoading).toBe(true);
    loading.unmount();

    state.profiles.isLoading = false;
    state.profiles.error = new Error("Profile catalog unavailable");
    const failed = renderHook(() => useProfilesSettingsPage());
    expect(failed.result.current.errorMessage).toBe("Profile catalog unavailable");
  });

  it("Should retry the profile list", () => {
    const { result } = renderHook(() => useProfilesSettingsPage());

    result.current.refetch();

    expect(state.profiles.refetch).toHaveBeenCalledOnce();
  });
});
