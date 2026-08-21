import { act, renderHook } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { useSettingsHooksPage } from "../use-settings-hooks-page";

const scope = vi.hoisted(() => ({ destination: "marketing" }));
const workspace = vi.hoisted(() => ({ activeWorkspaceId: "ws-a" as string | null }));
const mutations = vi.hoisted(() => ({
  createPreset: vi.fn(),
  deletePreset: vi.fn(),
  putHook: vi.fn(),
  setPresetEnablement: vi.fn(),
}));

vi.mock("../use-settings-page", () => ({
  useSettingsPage: () => ({ restart: null }),
}));

vi.mock("@/systems/profiles", () => ({
  useProfileReadScope: () => ({
    destination: scope.destination,
    destinationOwner: null,
  }),
}));

vi.mock("@/systems/workspace", () => ({
  useActiveWorkspace: () => ({ activeWorkspaceId: workspace.activeWorkspaceId }),
}));

vi.mock("@/systems/notifications", () => ({
  useCreateNotificationPreset: () => ({ error: null, mutate: mutations.createPreset }),
  useDeleteNotificationPreset: () => ({ error: null, mutate: mutations.deletePreset }),
  useNotificationPresets: () => ({
    data: { presets: [] },
    error: null,
    isLoading: false,
    refetch: vi.fn(),
  }),
  useSetNotificationPresetEnablement: () => ({
    error: null,
    mutate: mutations.setPresetEnablement,
  }),
}));

vi.mock("@/systems/settings", () => ({
  SettingsApiError: Error,
  usePutSettingsHook: () => ({ error: null, mutate: mutations.putHook }),
  useSettingsHooks: () => ({
    data: {
      hooks: [
        {
          declaration: { enabled: false, event: "task.completed", matcher: {} },
          name: "build",
        },
      ],
    },
    error: null,
    isLoading: false,
    refetch: vi.fn(),
  }),
  useSettingsHooksExtensions: () => ({
    data: { transport_parity: { settings_http: true } },
    error: null,
    isLoading: false,
    refetch: vi.fn(),
  }),
}));

describe("useSettingsHooksPage", () => {
  beforeEach(() => {
    scope.destination = "marketing";
    workspace.activeWorkspaceId = "ws-a";
    for (const mutation of Object.values(mutations)) mutation.mockReset();
  });

  it("Should isolate pending hook mutations by destination and request", () => {
    const { result, rerender } = renderHook(() => useSettingsHooksPage());
    const hook = result.current.hooks[0];
    if (!hook) throw new Error("Expected the hook fixture.");

    act(() => result.current.toggleHookEnabled(hook, true));
    expect(result.current.pendingHookName).toBe("build");
    const firstSettled = mutations.putHook.mock.calls[0]?.[1]?.onSettled;
    if (typeof firstSettled !== "function") throw new Error("Expected the first settlement.");

    workspace.activeWorkspaceId = "ws-b";
    rerender();
    expect(result.current.pendingHookName).toBeNull();

    act(() => result.current.toggleHookEnabled(hook, true));
    expect(result.current.pendingHookName).toBe("build");
    const secondSettled = mutations.putHook.mock.calls[1]?.[1]?.onSettled;
    if (typeof secondSettled !== "function") throw new Error("Expected the second settlement.");

    act(() => firstSettled());
    expect(result.current.pendingHookName).toBe("build");

    act(() => secondSettled());
    expect(result.current.pendingHookName).toBeNull();
  });
});
