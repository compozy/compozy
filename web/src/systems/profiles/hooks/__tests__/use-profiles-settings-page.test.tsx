import { renderHook } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const state: { tier: "local" | "remote" | undefined } = { tier: undefined };

vi.mock("@/systems/gateway", () => ({
  useGatewayAccessTier: () => state.tier,
}));

vi.mock("@/systems/workspace", () => ({
  useWorkspaces: () => ({ data: [] }),
}));

vi.mock("../use-profile-lens", () => ({
  useProfileLens: () => ({ scope: "global" }),
}));

vi.mock("../use-profile-selection", () => ({
  useActiveProfileView: () => ({ kind: "profile", profile: "default" }),
}));

vi.mock("../use-profiles", () => ({
  useProfiles: () => ({ data: [], isLoading: false, error: null, refetch: vi.fn() }),
  useProfileSelectionMap: () => ({ data: [] }),
}));

import { useProfilesSettingsPage } from "../use-profiles-settings-page";

describe("useProfilesSettingsPage", () => {
  beforeEach(() => {
    state.tier = undefined;
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
});
