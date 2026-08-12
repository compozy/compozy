// Suite: Task setup runtime query ownership
// Invariant: provider and model reads run only while their owning task window is live.
// Boundary IN: useTaskSetupRuntime liveness and workspace scope.
// Boundary OUT: TanStack Query transport and selector rendering.
import { renderHook } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const hooks = vi.hoisted(() => ({
  useRuntimeModelCatalog: vi.fn(),
  useSettingsProviders: vi.fn(),
  useWorkspace: vi.fn(),
}));

vi.mock("@/systems/model-catalog", () => ({
  providerNeedsAuth: vi.fn(),
  useRuntimeModelCatalog: hooks.useRuntimeModelCatalog,
}));

vi.mock("@/systems/settings", () => ({
  useSettingsProviders: hooks.useSettingsProviders,
}));

vi.mock("@/systems/workspace", () => ({
  useWorkspace: hooks.useWorkspace,
}));

import { useTaskSetupRuntime } from "../use-task-setup-runtime";

beforeEach(() => {
  vi.clearAllMocks();
  hooks.useRuntimeModelCatalog.mockReturnValue({
    error: null,
    loading: false,
    models: [],
    refresh: vi.fn(),
    refreshing: false,
  });
  hooks.useSettingsProviders.mockReturnValue({ data: undefined, error: null, isLoading: false });
  hooks.useWorkspace.mockReturnValue({ data: undefined, error: null, isLoading: false });
});

describe("useTaskSetupRuntime", () => {
  it("Should disable every remote read while the owner is inactive", () => {
    renderHook(() => useTaskSetupRuntime("ws_northstar", { enabled: false }));

    expect(hooks.useWorkspace).toHaveBeenCalledWith("ws_northstar", { enabled: false });
    expect(hooks.useSettingsProviders).toHaveBeenCalledWith({ enabled: false });
    expect(hooks.useRuntimeModelCatalog).toHaveBeenCalledWith([], { enabled: false });
  });

  it("Should read only the workspace provider source for a scoped task", () => {
    hooks.useWorkspace.mockReturnValue({
      data: { providers: [{ display_name: "Codex", name: "codex" }] },
      error: null,
      isLoading: false,
    });

    renderHook(() => useTaskSetupRuntime("ws_northstar"));

    expect(hooks.useWorkspace).toHaveBeenCalledWith("ws_northstar", { enabled: true });
    expect(hooks.useSettingsProviders).toHaveBeenCalledWith({ enabled: false });
    expect(hooks.useRuntimeModelCatalog).toHaveBeenCalledWith(
      [{ id: "codex", needsAuth: undefined }],
      { enabled: true }
    );
  });
});
