import { useMCPOverrideEditor } from "../use-mcp-override-editor";
import type { SettingsMCPServerEntry } from "../../types";
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
  updateSettingsAttention: vi.fn(),
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
  updateSettingsAttention,
  updateSettingsGeneral,
  updateSettingsHooksExtensions,
  updateSettingsMemory,
  updateSettingsRoles,
} from "../../adapters/settings-api";
import {
  exchangeSettingsMCPAuth,
  logoutSettingsMCPAuth,
} from "../../adapters/settings-mcp-auth-api";
import { extensionKeys } from "@/systems/extensions";
import { marketplaceKeys } from "@/systems/marketplace";
import { settingsKeys } from "../../lib/query-keys";
import {
  settingsHooksExtensionsSectionFixture,
  settingsMemoryConfigFixture,
} from "../../mocks/fixtures";
import { settingsRolesSectionFixture } from "../../mocks/roles-fixtures";
import { settingsRestartStore } from "../../stores/settings-restart-store";
import { resetSettingsRestartStore } from "../../stores/use-settings-restart-store";
import {
  useDeleteSettingsMCPServer,
  useDeleteSettingsProvider,
  useExchangeMCPAuth,
  useLogoutMCPAuth,
  usePutSettingsMCPServer,
  useReloadSettings,
  useUpdateSettingsAttention,
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
  scope: "user" as const,
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
  resetSettingsRestartStore();
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
          http: { host: "127.0.0.1", port: 2123 },
          limits: { max_concurrent_agents: 4 },
          permissions: { mode: "approve-reads" as const },
          redact: { enabled: true },
          session_timeout: "30m",
          terminal: {
            default_shell: "",
            shell_integration: true,
            scrollback_bytes: 1_048_576,
            detached_ttl: "24h",
            exit_retention: "15m",
            recording: false,
            recording_retention_days: 30,
            max_per_workspace: 8,
            max_per_daemon: 32,
            max_subscribers: 16,
          },
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

    expect(settingsRestartStore.getSnapshot().context.lastMutation?.restartRequired).toBe(true);
    expect(settingsRestartStore.getSnapshot().context.lastMutation?.warnings).toEqual([
      "restart the daemon",
    ]);
    expect(settingsRestartStore.getSnapshot().context.lastMutation?.nextAction).toBe(
      "restart-daemon"
    );
    expect(settingsRestartStore.getSnapshot().context.lastMutation?.applyRecordId).toBe(
      "cfg_apply_001"
    );

    const memoryInvalidations = invalidateSpy.mock.calls.filter(([arg]) =>
      JSON.stringify(arg?.queryKey).includes("memory")
    );
    expect(memoryInvalidations).toHaveLength(0);
  });
});

describe("useUpdateSettingsAttention", () => {
  it("Should start the live write before returning from the user gesture", () => {
    const { wrapper } = createWrapper();
    vi.mocked(updateSettingsAttention).mockResolvedValue({
      ...generalMutation,
      section: "attention" as const,
      lifecycle: "live" as const,
      next_action: "none" as const,
      restart_required: false,
    });
    const { result } = renderHook(() => useUpdateSettingsAttention(), { wrapper });
    const variables = {
      body: {
        config: {
          toasts: true,
          sound: false,
          system: false,
          muted_workspaces: [],
        },
      },
      filter: { scope: "user" as const },
    };

    act(() => {
      result.current.mutate(variables);
      expect(updateSettingsAttention).toHaveBeenCalledWith(variables.body, variables.filter);
    });
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

    expect(settingsRestartStore.getSnapshot().context.lastMutation?.activeGeneration).toBe(42);
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
        filter: { scope: "user", target: "sidecar" },
      });
    });

    await waitFor(() => {
      expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: settingsKeys.mcpRoot() });
      expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: settingsKeys.applyRoot() });
    });

    expect(putSettingsMCPServer).toHaveBeenCalledWith(
      "github",
      { server: { name: "github", command: "gh" } },
      { scope: "user", target: "sidecar" }
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
      owner: "manual",
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
        body: { redirect_url: "http://127.0.0.1:2123/api/mcp/oauth/callback?code=abc123&state=x" },
      });
    });

    await waitFor(() => {
      expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: settingsKeys.mcpRoot() });
    });
    // Auth is a runtime op, not a config edit: it must not queue a pending restart.
    expect(settingsRestartStore.getSnapshot().context.lastMutation).toBeNull();
    expect(exchangeSettingsMCPAuth).toHaveBeenCalledWith(
      "linear",
      { scope: "workspace", workspace_id: "ws_alpha" },
      { redirect_url: "http://127.0.0.1:2123/api/mcp/oauth/callback?code=abc123&state=x" }
    );
  });

  it("re-reads the scoped mcp list after logout", async () => {
    const { queryClient, wrapper } = createWrapper();
    const invalidateSpy = vi.spyOn(queryClient, "invalidateQueries");
    vi.mocked(logoutSettingsMCPAuth).mockResolvedValue({
      server_name: "linear",
      owner: "manual",
      scope: "user",
      status: "needs_login",
      token_present: false,
      refreshable: false,
    });

    const { result } = renderHook(() => useLogoutMCPAuth(), { wrapper });

    await act(async () => {
      await result.current.mutateAsync({ name: "linear", filter: { scope: "user" } });
    });

    await waitFor(() => {
      expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: settingsKeys.mcpRoot() });
    });
  });
});

// Invariant: owner-qualified MCP mutations refresh the installed extension and catalog observations too.
// Owner: Settings mutation effects. Canonical suite: use-settings-mutations.test.tsx.
describe("extension-owned MCP reconciliation", () => {
  it.each(["put", "delete", "exchange", "logout"] as const)(
    "Should invalidate all affected views after %s",
    async operation => {
      const { queryClient, wrapper } = createWrapper();
      const filter = {
        scope: "profile" as const,
        profile: "work",
        workspace_id: "ws-a",
        owner: "extension:kit",
      };
      const keys = [
        settingsKeys.mcpDetail("shared", filter),
        extensionKeys.list("ws-a", "work"),
        [...marketplaceKeys.all, "observed-extension"],
      ];
      for (const key of keys) queryClient.setQueryData(key, { observed: "before" });
      vi.mocked(putSettingsMCPServer).mockResolvedValue(generalMutation);
      vi.mocked(deleteSettingsMCPServer).mockResolvedValue(generalMutation);
      const auth = {
        server_name: "shared",
        owner: "extension:kit",
        scope: "profile",
        profile: "work",
        workspace_id: "ws-a",
        status: "authenticated",
        token_present: true,
        refreshable: true,
      };
      vi.mocked(exchangeSettingsMCPAuth).mockResolvedValue(auth);
      vi.mocked(logoutSettingsMCPAuth).mockResolvedValue({
        ...auth,
        status: "needs_login",
        token_present: false,
      });
      const { result, unmount } = renderHook(
        () => ({
          put: usePutSettingsMCPServer(),
          delete: useDeleteSettingsMCPServer(),
          exchange: useExchangeMCPAuth(),
          logout: useLogoutMCPAuth(),
        }),
        { wrapper }
      );
      await act(async () => {
        const params = { name: "shared", filter };
        switch (operation) {
          case "put":
            await result.current.put.mutateAsync({
              ...params,
              body: { server: { name: "shared", env: { DEBUG: "true" } } },
            });
            break;
          case "delete":
            await result.current.delete.mutateAsync(params);
            break;
          case "exchange":
            await result.current.exchange.mutateAsync({
              ...params,
              body: { redirect_url: "https://callback.example.test" },
            });
            break;
          case "logout":
            await result.current.logout.mutateAsync(params);
            break;
        }
      });
      for (const key of keys) expect(queryClient.getQueryState(key)?.isInvalidated).toBe(true);
      if (operation === "exchange" || operation === "logout") {
        expect(settingsRestartStore.getSnapshot().context.lastMutation).toBeNull();
      }
      unmount();
      queryClient.clear();
    }
  );
});

// Invariant: override edit/reset target the exact extension and preserve the draft on mutation failure.
// Owner: Settings mutation integration; canonical suite: use-settings-mutations.test.tsx.
describe("extension MCP override editor", () => {
  const entry: SettingsMCPServerEntry = {
    name: "github",
    owner: "extension:kit",
    runtime_name: "kit.github",
    transport: "http",
    scope: "workspace",
    workspace_id: "ws-a",
    url: "https://manifest.example/mcp",
    source_metadata: {
      available_targets: [],
      effective_source: { kind: "extension", scope: "workspace", workspace_id: "ws-a" },
    },
    override: { headers: { "X-Team": "team-a" } },
  };
  it("Should save and reset only the selected owner and keep failed edits open", async () => {
    const { wrapper, queryClient } = createWrapper();
    const { result, unmount } = renderHook(() => useMCPOverrideEditor(), { wrapper });
    act(() => result.current.openEdit(entry));
    const draft = { env: [], headers: [{ key: "X-Team", value: "team-b" }], url: "" };
    act(() => result.current.editorProps?.onChange(draft));
    vi.mocked(putSettingsMCPServer).mockRejectedValueOnce(new Error("publication failed"));
    act(() => result.current.editorProps?.onSave());
    await waitFor(() => expect(result.current.editorProps?.saveError).toBe("publication failed"));
    expect(result.current.editorProps?.draft).toEqual(draft);
    const filter = { scope: "workspace", workspace_id: "ws-a", owner: "extension:kit" };
    expect(putSettingsMCPServer).toHaveBeenCalledWith(
      "github",
      {
        server: {
          name: "github",
          env: {},
          headers: { "X-Team": "team-b" },
          url: "",
        },
      },
      filter
    );
    vi.mocked(putSettingsMCPServer).mockResolvedValueOnce(generalMutation);
    act(() => result.current.editorProps?.onSave());
    await waitFor(() => expect(result.current.editorProps).toBeNull());
    act(() => result.current.openEdit(entry));
    vi.mocked(deleteSettingsMCPServer).mockResolvedValueOnce(generalMutation);
    act(() => result.current.editorProps?.onReset());
    await waitFor(() => expect(result.current.editorProps).toBeNull());
    expect(deleteSettingsMCPServer).toHaveBeenCalledWith("github", filter);
    act(() => result.current.openEdit({ ...entry, owner: "manual" }));
    expect(result.current.editorProps).toBeNull();
    unmount();
    queryClient.clear();
  });
});
