import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, renderHook, waitFor } from "@testing-library/react";
import { createElement, type ReactNode } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("../../adapters/settings-api", () => ({
  deleteSettingsSandbox: vi.fn(),
  deleteSettingsHook: vi.fn(),
  deleteSettingsMCPServer: vi.fn(),
  deleteSettingsProvider: vi.fn(),
  putSettingsSandbox: vi.fn(),
  putSettingsHook: vi.fn(),
  putSettingsMCPServer: vi.fn(),
  putSettingsProvider: vi.fn(),
  reloadSettings: vi.fn(),
  updateSettingsAutomation: vi.fn(),
  updateSettingsGeneral: vi.fn(),
  updateSettingsHooksExtensions: vi.fn(),
  updateSettingsMemory: vi.fn(),
  updateSettingsNetwork: vi.fn(),
  updateSettingsObservability: vi.fn(),
  updateSettingsRoles: vi.fn(),
  updateSettingsSkills: vi.fn(),
}));

vi.mock("../../adapters/settings-mcp-auth-api", () => ({
  beginSettingsMCPAuth: vi.fn(),
  exchangeSettingsMCPAuth: vi.fn(),
  logoutSettingsMCPAuth: vi.fn(),
}));

import {
  deleteSettingsMCPServer,
  deleteSettingsProvider,
  putSettingsMCPServer,
  reloadSettings,
  updateSettingsGeneral,
  updateSettingsHooksExtensions,
  updateSettingsMemory,
  updateSettingsRoles,
} from "../../adapters/settings-api";
import {
  exchangeSettingsMCPAuth,
  logoutSettingsMCPAuth,
} from "../../adapters/settings-mcp-auth-api";
import { settingsKeys } from "../../lib/query-keys";
import {
  settingsHooksExtensionsSectionFixture,
  settingsMemoryConfigFixture,
} from "../../mocks/fixtures";
import { settingsRolesSectionFixture } from "../../mocks/roles-fixtures";
import { initialSettingsRestartState } from "../../stores/settings-restart-store";
import { useSettingsRestartStore } from "../../stores/use-settings-restart-store";
import {
  useDeleteSettingsMCPServer,
  useDeleteSettingsProvider,
  useExchangeMCPAuth,
  useLogoutMCPAuth,
  usePutSettingsMCPServer,
  useReloadSettings,
  useUpdateSettingsGeneral,
  useUpdateSettingsHooksExtensions,
  useUpdateSettingsMemory,
  useUpdateSettingsRoles,
} from "../use-settings-mutations";

function createWrapper() {
  const queryClient = new QueryClient({
    defaultOptions: { mutations: { retry: false }, queries: { retry: false } },
  });

  const wrapper = ({ children }: { children: ReactNode }) =>
    createElement(QueryClientProvider, { client: queryClient }, children);

  return { queryClient, wrapper };
}

const generalMutation = {
  active_config_hash: "sha256:active-live",
  active_generation: 42,
  section: "general" as const,
  scope: "global" as const,
  applied: true,
  apply_record_id: "cfg_apply_001",
  lifecycle: "restart-required" as const,
  next_action: "restart-daemon" as const,
  restart_required: true,
  restart_scope: "daemon",
  warnings: ["restart the daemon"],
  write_target: "global-config" as const,
};

beforeEach(() => {
  vi.clearAllMocks();
  useSettingsRestartStore.setState({
    ...initialSettingsRestartState,
    startRestart: useSettingsRestartStore.getState().startRestart,
    updateRestart: useSettingsRestartStore.getState().updateRestart,
    clearRestart: useSettingsRestartStore.getState().clearRestart,
    recordMutation: useSettingsRestartStore.getState().recordMutation,
  });
});

afterEach(() => {
  vi.restoreAllMocks();
});

describe("useUpdateSettingsGeneral", () => {
  it("records mutation state and invalidates the general section plus apply records", async () => {
    const { queryClient, wrapper } = createWrapper();
    const invalidateSpy = vi.spyOn(queryClient, "invalidateQueries");
    vi.mocked(updateSettingsGeneral).mockResolvedValue(generalMutation);

    const { result } = renderHook(() => useUpdateSettingsGeneral(), { wrapper });

    await act(async () => {
      await result.current.mutateAsync({
        config: {
          daemon: {
            memory_report_interval: "5m",
            reload_timeouts: { bridges: "30s", mcp: "10s", providers: "5s" },
            socket: "/tmp/a.sock",
          },
          defaults: { agent: "claude-code" },
          http: { host: "127.0.0.1", port: 2123 },
          limits: { max_concurrent_agents: 4 },
          permissions: { mode: "approve-reads" as const },
          redact: { enabled: true },
          session_timeout: "30m",
        },
      });
    });

    await waitFor(() => {
      expect(invalidateSpy).toHaveBeenCalledWith({
        queryKey: settingsKeys.section("general"),
      });
      expect(invalidateSpy).toHaveBeenCalledWith({
        queryKey: settingsKeys.applyRoot(),
      });
    });

    expect(useSettingsRestartStore.getState().lastMutation?.restartRequired).toBe(true);
    expect(useSettingsRestartStore.getState().lastMutation?.warnings).toEqual([
      "restart the daemon",
    ]);
    expect(useSettingsRestartStore.getState().lastMutation?.nextAction).toBe("restart-daemon");
    expect(useSettingsRestartStore.getState().lastMutation?.applyRecordId).toBe("cfg_apply_001");

    const memoryInvalidations = invalidateSpy.mock.calls.filter(([arg]) =>
      JSON.stringify(arg?.queryKey).includes("memory")
    );
    expect(memoryInvalidations).toHaveLength(0);
  });
});

describe("useUpdateSettingsMemory", () => {
  it("invalidates memory section and apply records", async () => {
    const { queryClient, wrapper } = createWrapper();
    const invalidateSpy = vi.spyOn(queryClient, "invalidateQueries");
    vi.mocked(updateSettingsMemory).mockResolvedValue({
      ...generalMutation,
      section: "memory" as const,
    });

    const { result } = renderHook(() => useUpdateSettingsMemory(), { wrapper });

    await act(async () => {
      await result.current.mutateAsync({
        config: {
          ...settingsMemoryConfigFixture,
          dream: { ...settingsMemoryConfigFixture.dream, min_hours: 1 },
        },
      });
    });

    await waitFor(() => {
      expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: settingsKeys.section("memory") });
      expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: settingsKeys.applyRoot() });
    });
  });
});

describe("useUpdateSettingsRoles", () => {
  it("Should reconcile the section cache and invalidate role consumers", async () => {
    const { queryClient, wrapper } = createWrapper();
    const invalidateSpy = vi.spyOn(queryClient, "invalidateQueries");
    const updatedConfig = structuredClone(settingsRolesSectionFixture.config);
    updatedConfig.dream.model = "updated-dream-model";
    queryClient.setQueryData(settingsKeys.section("roles"), settingsRolesSectionFixture);
    vi.mocked(updateSettingsRoles).mockResolvedValue({
      ...generalMutation,
      section: "roles" as const,
      lifecycle: "live" as const,
      next_action: "none" as const,
      restart_required: false,
    });

    const { result } = renderHook(() => useUpdateSettingsRoles(), { wrapper });
    await act(async () => {
      await result.current.mutateAsync({ config: updatedConfig });
    });

    expect(queryClient.getQueryData(settingsKeys.section("roles"))).toMatchObject({
      config: { dream: { model: "updated-dream-model" } },
    });
    await waitFor(() => {
      expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: settingsKeys.section("roles") });
      expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: settingsKeys.rolesStatus() });
      expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: settingsKeys.applyRoot() });
    });
  });
});

describe("useUpdateSettingsHooksExtensions", () => {
  it("Should invalidate marketplace discovery after policy changes", async () => {
    const { queryClient, wrapper } = createWrapper();
    const invalidateSpy = vi.spyOn(queryClient, "invalidateQueries");
    vi.mocked(updateSettingsHooksExtensions).mockResolvedValue({
      ...generalMutation,
      section: "hooks-extensions" as const,
    });
    const { result } = renderHook(() => useUpdateSettingsHooksExtensions(), { wrapper });

    await act(async () => {
      await result.current.mutateAsync({ config: settingsHooksExtensionsSectionFixture.config });
    });

    await waitFor(() => {
      expect(invalidateSpy).toHaveBeenCalledWith({
        queryKey: settingsKeys.section("hooks-extensions"),
      });
      expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: ["marketplace"] });
    });
  });
});

describe("useReloadSettings", () => {
  it("records reload state and invalidates all settings queries", async () => {
    const { queryClient, wrapper } = createWrapper();
    const invalidateSpy = vi.spyOn(queryClient, "invalidateQueries");
    vi.mocked(reloadSettings).mockResolvedValue(generalMutation);

    const { result } = renderHook(() => useReloadSettings(), { wrapper });

    await act(async () => {
      await result.current.mutateAsync();
    });

    await waitFor(() => {
      expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: settingsKeys.all });
    });

    expect(useSettingsRestartStore.getState().lastMutation?.activeGeneration).toBe(42);
  });
});

describe("provider mutations", () => {
  it("invalidates provider detail and list on delete", async () => {
    const { queryClient, wrapper } = createWrapper();
    const invalidateSpy = vi.spyOn(queryClient, "invalidateQueries");
    vi.mocked(deleteSettingsProvider).mockResolvedValue({
      ...generalMutation,
      section: "general" as const,
    });

    const { result } = renderHook(() => useDeleteSettingsProvider(), { wrapper });

    await act(async () => {
      await result.current.mutateAsync("openai");
    });

    await waitFor(() => {
      expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: settingsKeys.providersRoot() });
      expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: settingsKeys.applyRoot() });
      expect(invalidateSpy).toHaveBeenCalledWith({
        queryKey: settingsKeys.providerDetail("openai"),
      });
    });
  });
});

describe("mcp server mutations", () => {
  it("invalidates the entire mcp-server root on put", async () => {
    const { queryClient, wrapper } = createWrapper();
    const invalidateSpy = vi.spyOn(queryClient, "invalidateQueries");
    vi.mocked(putSettingsMCPServer).mockResolvedValue({
      ...generalMutation,
      section: "general" as const,
    });

    const { result } = renderHook(() => usePutSettingsMCPServer(), { wrapper });

    await act(async () => {
      await result.current.mutateAsync({
        name: "github",
        body: { server: { name: "github", command: "gh" } },
        filter: { scope: "global", target: "sidecar" },
      });
    });

    await waitFor(() => {
      expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: settingsKeys.mcpRoot() });
      expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: settingsKeys.applyRoot() });
    });

    expect(putSettingsMCPServer).toHaveBeenCalledWith(
      "github",
      { server: { name: "github", command: "gh" } },
      { scope: "global", target: "sidecar" }
    );
  });

  it("forwards scope and target filters on delete", async () => {
    const { wrapper } = createWrapper();
    vi.mocked(deleteSettingsMCPServer).mockResolvedValue({
      ...generalMutation,
      section: "general" as const,
    });

    const { result } = renderHook(() => useDeleteSettingsMCPServer(), { wrapper });

    await act(async () => {
      await result.current.mutateAsync({
        name: "github",
        filter: { scope: "workspace", workspace_id: "ws_alpha", target: "auto" },
      });
    });

    expect(deleteSettingsMCPServer).toHaveBeenCalledWith("github", {
      scope: "workspace",
      workspace_id: "ws_alpha",
      target: "auto",
    });
  });
});

describe("mcp auth mutations", () => {
  it("re-reads the scoped mcp list after a successful exchange without recording a restart", async () => {
    const { queryClient, wrapper } = createWrapper();
    const invalidateSpy = vi.spyOn(queryClient, "invalidateQueries");
    vi.mocked(exchangeSettingsMCPAuth).mockResolvedValue({
      server_name: "linear",
      scope: "workspace",
      status: "authenticated",
      token_present: true,
      refreshable: true,
    });

    const { result } = renderHook(() => useExchangeMCPAuth(), { wrapper });

    await act(async () => {
      await result.current.mutateAsync({
        name: "linear",
        filter: { scope: "workspace", workspace_id: "ws_alpha" },
        body: { code: "abc123" },
      });
    });

    await waitFor(() => {
      expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: settingsKeys.mcpRoot() });
    });
    // Auth is a runtime op, not a config edit: it must not queue a pending restart.
    expect(useSettingsRestartStore.getState().lastMutation).toBeNull();
    expect(exchangeSettingsMCPAuth).toHaveBeenCalledWith(
      "linear",
      { scope: "workspace", workspace_id: "ws_alpha" },
      { code: "abc123" }
    );
  });

  it("re-reads the scoped mcp list after logout", async () => {
    const { queryClient, wrapper } = createWrapper();
    const invalidateSpy = vi.spyOn(queryClient, "invalidateQueries");
    vi.mocked(logoutSettingsMCPAuth).mockResolvedValue({
      server_name: "linear",
      scope: "global",
      status: "needs_login",
      token_present: false,
      refreshable: false,
    });

    const { result } = renderHook(() => useLogoutMCPAuth(), { wrapper });

    await act(async () => {
      await result.current.mutateAsync({ name: "linear", filter: { scope: "global" } });
    });

    await waitFor(() => {
      expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: settingsKeys.mcpRoot() });
    });
  });
});
