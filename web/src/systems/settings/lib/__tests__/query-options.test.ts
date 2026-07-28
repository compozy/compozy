import { describe, expect, it } from "vitest";

import {
  SETTINGS_QUERY_INTERVALS,
  shouldRetrySettingsQuery,
  settingsAutomationOptions,
  settingsSandboxDetailOptions,
  settingsGeneralOptions,
  settingsMCPServersListOptions,
  settingsProviderDetailOptions,
  settingsRestartStatusOptions,
} from "../query-options";
import { SettingsApiError } from "../../adapters/settings-api";

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
});

describe("settings collection options", () => {
  it("disables detail queries when name is empty", () => {
    expect(settingsProviderDetailOptions("").enabled).toBe(false);
    expect(settingsSandboxDetailOptions("").enabled).toBe(false);
  });

  it("enables detail queries when name is provided", () => {
    expect(settingsProviderDetailOptions("openai").enabled).toBe(true);
    expect(settingsSandboxDetailOptions("cloud").enabled).toBe(true);
  });

  it("includes scope and workspace filters in MCP list query keys", () => {
    const global = settingsMCPServersListOptions({ scope: "global" });
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
      "global",
      "",
    ]);
    expect(scoped.queryKey).toEqual([
      "settings",
      "collection",
      "mcp-servers",
      "list",
      "workspace",
      "ws_alpha",
    ]);
    expect(disabled.enabled).toBe(false);
  });

  it("uses an auth-flow polling interval when the caller requests one", () => {
    const authorizing = settingsMCPServersListOptions({ scope: "global" }, true, 1_000);

    expect(authorizing.refetchInterval).toBe(1_000);
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

  it("polls while the status is not terminal and stops on terminal states", () => {
    const options = settingsRestartStatusOptions("op_1", true);
    const refetchInterval = options.refetchInterval as (query: {
      state: { data?: { status: string } };
    }) => number | false;

    expect(refetchInterval({ state: {} })).toBe(SETTINGS_QUERY_INTERVALS.restartPollInterval);
    expect(refetchInterval({ state: { data: { status: "stopping" } } })).toBe(
      SETTINGS_QUERY_INTERVALS.restartPollInterval
    );
    expect(refetchInterval({ state: { data: { status: "ready" } } })).toBe(false);
    expect(refetchInterval({ state: { data: { status: "failed" } } })).toBe(false);
  });
});
