// Suite: live attention settings writes
// Invariant: a pending full-config write is the displayed state and blocks a
// second write based on stale config; write failures remain visible.
// Owning layer: Settings Attention page model. No prior suite owned this flow.
import { act, renderHook } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const { query, mutation, requestSystemNotifications } = vi.hoisted(() => ({
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
          config: {
            toasts: boolean;
            sound: boolean;
            system: boolean;
            muted_workspaces: string[];
          };
        }
      | undefined,
    error: null as Error | null,
    mutate: vi.fn(),
  },
  requestSystemNotifications: vi.fn(),
}));

vi.mock("@/systems/os", () => ({
  requestSystemNotifications,
  systemNotificationState: () => "default",
}));
vi.mock("../../adapters/settings-api", () => ({
  SettingsApiError: class SettingsApiError extends Error {},
}));
vi.mock("../use-settings-page", () => ({
  useSettingsPage: () => ({ restart: { isVisible: false } }),
}));
vi.mock("../use-settings-sections", () => ({ useSettingsAttention: () => query }));
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
  });

  it("Should display and lock the full config currently being written", () => {
    mutation.isPending = true;
    mutation.variables = { config: { ...storedConfig, toasts: false } };
    const { result } = renderHook(() => useSettingsAttentionPage());

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
});
