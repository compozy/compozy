// Suite: live attention settings writes
// Invariant: a pending full-settings write is the displayed state and blocks a
// second write based on stale config; write failures remain visible.
// Owning layer: Settings Attention page model. No prior suite owned this flow.
import { act, renderHook } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const { query, mutation, profileReadScope, requestSystemNotifications, useSettingsAttention } =
  vi.hoisted(() => ({
    query: {
      data: undefined as
        | {
            config: {
              toasts: boolean;
              sound: boolean;
              system: boolean;
              muted_workspaces: string[];
            };
          }
        | undefined,
      isLoading: false,
      error: null as Error | null,
      refetch: vi.fn(),
    },
    mutation: {
      isPending: false,
      variables: undefined as
        | {
            body: {
              config: {
                toasts: boolean;
                sound: boolean;
                system: boolean;
                muted_workspaces: string[];
              };
            };
            filter: { scope: "profile"; profile: string };
          }
        | undefined,
      error: null as Error | null,
      mutate: vi.fn(),
    },
    profileReadScope: { destination: "marketing" },
    requestSystemNotifications: vi.fn(),
    useSettingsAttention: vi.fn(),
  }));

vi.mock("@/systems/os", () => ({
  requestSystemNotifications,
  systemNotificationState: () => "default",
}));
vi.mock("@/systems/profiles", () => ({
  useProfileReadScope: () => profileReadScope,
}));
vi.mock("../../adapters/settings-api", () => ({
  SettingsApiError: class SettingsApiError extends Error {},
}));
vi.mock("../use-settings-page", () => ({
  useSettingsPage: () => ({ restart: { isVisible: false } }),
}));
vi.mock("../use-settings-sections", () => ({ useSettingsAttention }));
vi.mock("../use-settings-mutations", () => ({
  useUpdateSettingsAttention: () => mutation,
}));

import { useSettingsAttentionPage } from "../use-settings-attention-page";

const storedConfig = {
  toasts: true,
  sound: true,
  system: false,
  muted_workspaces: [] as string[],
};

describe("useSettingsAttentionPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    query.data = { config: storedConfig };
    query.isLoading = false;
    query.error = null;
    mutation.isPending = false;
    mutation.variables = undefined;
    mutation.error = null;
    profileReadScope.destination = "marketing";
    useSettingsAttention.mockReturnValue(query);
  });

  it("Should display and lock the full config currently being written", () => {
    mutation.isPending = true;
    mutation.variables = {
      body: { config: { ...storedConfig, toasts: false } },
      filter: { scope: "profile", profile: "marketing" },
    };
    const { result } = renderHook(() => useSettingsAttentionPage());

    expect(useSettingsAttention).toHaveBeenCalledWith({
      scope: "profile",
      profile: "marketing",
    });
    expect(result.current.config?.toasts).toBe(false);
    expect(result.current.isSaving).toBe(true);
    act(() => result.current.setSound(false));
    expect(mutation.mutate).not.toHaveBeenCalled();
  });

  it("Should expose the failed write without replacing daemon config", () => {
    mutation.error = new Error("write rejected");
    const { result } = renderHook(() => useSettingsAttentionPage());

    expect(result.current.config).toEqual(storedConfig);
    expect(result.current.saveError).toBe("write rejected");
  });

  it("Should not display another profile's pending attention candidate", () => {
    mutation.isPending = true;
    mutation.variables = {
      body: { config: { ...storedConfig, toasts: false } },
      filter: { scope: "profile", profile: "sales" },
    };
    const { result } = renderHook(() => useSettingsAttentionPage());

    expect(result.current.config).toEqual(storedConfig);
    expect(result.current.isSaving).toBe(true);
  });

  it("Should write workspace mutes to the active profile", () => {
    const { result } = renderHook(() => useSettingsAttentionPage());

    act(() => result.current.muteWorkspace("ws_0123456789abcdef"));

    expect(mutation.mutate).toHaveBeenCalledWith({
      body: {
        config: {
          ...storedConfig,
          muted_workspaces: ["ws_0123456789abcdef"],
        },
      },
      filter: { scope: "profile", profile: "marketing" },
    });
  });
});
