import { QueryClient } from "@tanstack/react-query";
import { beforeEach, describe, expect, it, vi } from "vitest";

const adapterMocks = vi.hoisted(() => ({
  getSettingsAttention: vi.fn(),
  getSettingsMCPServer: vi.fn(),
  getSettingsPersona: vi.fn(),
  listSettingsHooks: vi.fn(),
}));

vi.mock("../../adapters/settings-api", async importOriginal => ({
  ...(await importOriginal<typeof import("../../adapters/settings-api")>()),
  getSettingsAttention: adapterMocks.getSettingsAttention,
  getSettingsMCPServer: adapterMocks.getSettingsMCPServer,
  getSettingsPersona: adapterMocks.getSettingsPersona,
  listSettingsHooks: adapterMocks.listSettingsHooks,
}));

import {
  SETTINGS_QUERY_INTERVALS,
  shouldRetrySettingsQuery,
  settingsAttentionOptions,
  settingsAutomationOptions,
  settingsSandboxDetailOptions,
  settingsGeneralOptions,
  settingsHooksListOptions,
  settingsMCPServersListOptions,
  settingsMCPServerDetailOptions,
  settingsProviderDetailOptions,
  settingsPersonaOptions,
  settingsProvidersListOptions,
  settingsRestartStatusOptions,
  settingsUpdateOptions,
} from "../query-options";
import { settingsKeys } from "../query-keys";
import { SettingsApiError } from "../../adapters/settings-api";

beforeEach(() => {
  vi.clearAllMocks();
  adapterMocks.getSettingsAttention.mockResolvedValue({});
  adapterMocks.getSettingsPersona.mockResolvedValue({});
  adapterMocks.listSettingsHooks.mockResolvedValue({ hooks: [] });
});

describe("settings section options", () => {
  it("uses the configured stale and refetch intervals for sections", () => {
    const general = settingsGeneralOptions();
    const automation = settingsAutomationOptions();

    expect(general.staleTime).toBe(SETTINGS_QUERY_INTERVALS.sectionStaleTime);
    expect(general.refetchInterval).toBe(SETTINGS_QUERY_INTERVALS.sectionRefetchInterval);
    expect(automation.queryKey).toEqual(["settings", "section", "automation"]);
  });

  it("does not retry policy-blocked settings requests", () => {
    expect(
      shouldRetrySettingsQuery(
        0,
        new SettingsApiError(
          "remote HTTP API access is disabled unless the daemon is bound to a loopback host",
          403
        )
      )
    ).toBe(false);
    expect(shouldRetrySettingsQuery(0, new Error("temporary failure"))).toBe(true);
    expect(shouldRetrySettingsQuery(2, new Error("temporary failure"))).toBe(false);
  });

  it("binds persona cache identity and transport to the selected profile", async () => {
    const filter = { scope: "profile" as const, profile: "marketing" };
    const options = settingsPersonaOptions(filter);
    const signal = new AbortController().signal;

    expect(options.queryKey).toEqual([
      "settings",
      "section",
      "persona",
      "profile",
      "",
      "marketing",
    ]);
    if (typeof options.queryFn !== "function") throw new Error("persona query function missing");
    await options.queryFn({ signal } as never);
    expect(adapterMocks.getSettingsPersona).toHaveBeenCalledWith(filter, signal);
    expect(options.staleTime).toBe(SETTINGS_QUERY_INTERVALS.sectionStaleTime);
    expect(options.retry).toBe(shouldRetrySettingsQuery);
  });

  it("binds attention cache identity and transport to the selected profile", async () => {
    const filter = { scope: "profile" as const, profile: "marketing" };
    const options = settingsAttentionOptions(filter);
    const signal = new AbortController().signal;

    expect(options.queryKey).toEqual(["settings", "section", "attention", "profile", "marketing"]);
    if (typeof options.queryFn !== "function") throw new Error("attention query function missing");
    await options.queryFn({ signal } as never);
    expect(adapterMocks.getSettingsAttention).toHaveBeenCalledWith(filter, signal);
  });
});

describe("settings collection options", () => {
  it("disables the provider collection when its owner is inactive", () => {
    expect(settingsProvidersListOptions(false).enabled).toBe(false);
  });

  it("disables detail queries when name is empty", () => {
    expect(settingsProviderDetailOptions("").enabled).toBe(false);
    expect(settingsSandboxDetailOptions("").enabled).toBe(false);
  });

  it("enables detail queries when name is provided", () => {
    expect(settingsProviderDetailOptions("openai").enabled).toBe(true);
    expect(settingsSandboxDetailOptions("cloud").enabled).toBe(true);
  });

  it("includes scope and workspace filters in MCP list query keys", () => {
    const global = settingsMCPServersListOptions({ scope: "user" });
    const scoped = settingsMCPServersListOptions({
      scope: "workspace",
      workspace_id: "ws_alpha",
    });
    const disabled = settingsMCPServersListOptions({ scope: "workspace" }, false);

    expect(global.queryKey).toEqual([
      "settings",
      "collection",
      "mcp-servers",
      "list",
      "user",
      "",
      "",
    ]);
    expect(scoped.queryKey).toEqual([
      "settings",
      "collection",
      "mcp-servers",
      "list",
      "workspace",
      "ws_alpha",
      "",
    ]);
    expect(disabled.enabled).toBe(false);
  });

  it("uses an auth-flow polling interval when the caller requests one", () => {
    const authorizing = settingsMCPServersListOptions({ scope: "user" }, true, 1_000);

    expect(authorizing.refetchInterval).toBe(1_000);
  });

  it("binds hook cache identity and transport to the selected profile", async () => {
    const filter = {
      scope: "profile" as const,
      profile: "marketing",
      workspace_id: "ws_alpha",
    };
    const options = settingsHooksListOptions(filter);
    const signal = new AbortController().signal;

    expect(options.queryKey).toEqual([
      "settings",
      "collection",
      "hooks",
      "list",
      "profile",
      "ws_alpha",
      "marketing",
    ]);
    if (typeof options.queryFn !== "function") throw new Error("hooks query function missing");
    await options.queryFn({ signal } as never);
    expect(adapterMocks.listSettingsHooks).toHaveBeenCalledWith(filter, signal);
    expect(options.refetchInterval).toBe(SETTINGS_QUERY_INTERVALS.collectionRefetchInterval);
    expect(options.retry).toBe(shouldRetrySettingsQuery);
  });
});

describe("settings restart options", () => {
  it("is disabled while no operation id is active", () => {
    const disabled = settingsRestartStatusOptions(null, true);
    const enabled = settingsRestartStatusOptions("op_1", true);

    expect(disabled.enabled).toBe(false);
    expect(enabled.enabled).toBe(true);
    expect(enabled.queryKey).toEqual(["settings", "restart", "op_1"]);
  });

  it("polls through transient errors and stops only on terminal states", () => {
    const options = settingsRestartStatusOptions("op_1", true);
    const refetchInterval = options.refetchInterval as (query: {
      state: { data?: { status: string }; error?: Error | null; status?: string };
    }) => number | false;

    expect(refetchInterval({ state: {} })).toBe(SETTINGS_QUERY_INTERVALS.restartPollInterval);
    expect(refetchInterval({ state: { status: "error" } })).toBe(
      SETTINGS_QUERY_INTERVALS.restartPollInterval
    );
    expect(refetchInterval({ state: { data: { status: "stopping" } } })).toBe(
      SETTINGS_QUERY_INTERVALS.restartPollInterval
    );
    expect(refetchInterval({ state: { data: { status: "ready" } } })).toBe(false);
    expect(refetchInterval({ state: { data: { status: "failed" } } })).toBe(false);
  });

  it.each([403, 404])("Should stop restart polling after terminal HTTP %s errors", status => {
    const options = settingsRestartStatusOptions("op_1", true);
    const refetchInterval = options.refetchInterval as (query: {
      state: { error: Error };
    }) => number | false;

    expect(
      refetchInterval({
        state: { error: Object.assign(new Error("Restart unavailable"), { status }) },
      })
    ).toBe(false);
  });
});

describe("settings update options", () => {
  it("uses the section cadence at rest and the live cadence while an operation exists", () => {
    const options = settingsUpdateOptions();
    const refetchInterval = options.refetchInterval as (query: {
      state: { data?: { operation: object | null } };
    }) => number;

    expect(refetchInterval({ state: {} })).toBe(SETTINGS_QUERY_INTERVALS.sectionRefetchInterval);
    expect(refetchInterval({ state: { data: { operation: null } } })).toBe(
      SETTINGS_QUERY_INTERVALS.sectionRefetchInterval
    );
    expect(refetchInterval({ state: { data: { operation: {} } } })).toBe(
      SETTINGS_QUERY_INTERVALS.updateOperationPollInterval
    );
  });
});

// Invariant: MCP detail data cannot be reused across owner, profile, workspace, or logical-name identities.
// Owner: Settings query cache. Canonical suite: query-options.test.ts.
describe("MCP definition detail cache", () => {
  it("Should keep each selected definition separate and invalidate all definitions after mutation", async () => {
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    adapterMocks.getSettingsMCPServer.mockImplementation(async (name, filter) => ({
      server: { name, ...filter },
    }));
    const options = [
      settingsMCPServerDetailOptions("shared", { scope: "user", owner: "manual" }),
      settingsMCPServerDetailOptions("shared", { scope: "user", owner: "extension:kit" }),
      settingsMCPServerDetailOptions("shared", {
        scope: "profile",
        profile: "work",
        owner: "extension:kit",
      }),
      settingsMCPServerDetailOptions("shared", {
        scope: "profile",
        profile: "work",
        workspace_id: "ws-a",
        owner: "extension:kit",
      }),
      settingsMCPServerDetailOptions("second", { scope: "user", owner: "extension:kit" }),
    ];
    for (const option of options) await client.fetchQuery(option);
    expect(adapterMocks.getSettingsMCPServer).toHaveBeenCalledTimes(options.length);
    expect(client.getQueryData(options[0]!.queryKey)?.server.owner).toBe("manual");
    expect(client.getQueryData(options[1]!.queryKey)?.server.owner).toBe("extension:kit");
    expect(client.getQueryData(options[2]!.queryKey)?.server.profile).toBe("work");
    expect(client.getQueryData(options[3]!.queryKey)?.server.workspace_id).toBe("ws-a");
    expect(client.getQueryData(options[4]!.queryKey)?.server.name).toBe("second");
    await client.invalidateQueries({ queryKey: settingsKeys.mcpRoot() });
    expect(
      client
        .getQueryCache()
        .getAll()
        .every(query => query.state.isInvalidated)
    ).toBe(true);
    client.clear();
  });
});
