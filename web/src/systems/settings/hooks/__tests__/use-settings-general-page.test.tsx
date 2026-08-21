import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, renderHook, waitFor } from "@testing-library/react";
import { createElement, type ReactNode } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("@tanstack/react-router", () => ({
  useMatchRoute: () => () => false,
}));

const workspaceState = vi.hoisted(() => ({
  activeWorkspaceId: "ws_alpha" as string | null,
}));

vi.mock("@/systems/workspace/hooks/use-active-workspace", () => ({
  useActiveWorkspace: () => ({
    activeWorkspaceId: workspaceState.activeWorkspaceId,
  }),
}));

vi.mock("@/systems/settings/adapters/settings-api", () => ({
  getSettingsGeneral: vi.fn(),
  listSettingsApplyRecords: vi.fn(),
  getSettingsUpdate: vi.fn(),
  applySettingsUpdate: vi.fn(),
  cancelSettingsUpdate: vi.fn(),
  reloadSettings: vi.fn(),
  updateSettingsGeneral: vi.fn(),
  getSettingsRestartStatus: vi.fn(),
  triggerSettingsRestart: vi.fn(),
  SettingsApiError: class SettingsApiError extends Error {
    status = 500;
  },
}));

import {
  applySettingsUpdate,
  cancelSettingsUpdate,
  getSettingsGeneral,
  listSettingsApplyRecords,
  getSettingsUpdate,
  reloadSettings,
  updateSettingsGeneral,
} from "@/systems/settings/adapters/settings-api";
import { resetSettingsRestartStore } from "@/systems/settings/stores/use-settings-restart-store";
import {
  settingsUpdateBothAvailableFixture,
  settingsUpdateStagedFixture,
} from "@/systems/settings/mocks/settings-update-fixture";

import { useSettingsGeneralPage } from "../use-settings-general-page";
import type { SettingsGeneralSection } from "@/systems/settings";

const envelope: SettingsGeneralSection = {
  section: "general",
  scope: "user",
  available_scopes: ["user"],
  actions: {
    restart: { available: true, behavior: "action_trigger", name: "restart" },
  },
  config: {
    daemon: {
      memory_report_interval: "5m",
      reload_timeouts: { bridges: "30s", mcp: "10s", providers: "5s" },
      socket: "/tmp/compozy.sock",
    },
    http: { host: "127.0.0.1", port: 2123 },
    limits: { max_concurrent_agents: 20 },
    permissions: { mode: "approve-all" },
    redact: { enabled: true },
    session_timeout: "0s",
  },
  config_paths: {
    daemon_info: "/tmp/daemon.json",
    global_config: "~/.compozy/config.toml",
    global_mcp_sidecar: "~/.compozy/mcp.json",
    home_dir: "~/.compozy",
    log_file: "~/.compozy/compozy.log",
  },
  runtime: {
    active_agents: 1,
    active_sessions: 1,
    available: true,
    total_sessions: 1,
    uptime_seconds: 60,
  },
};

function createWrapper() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  const wrapper = ({ children }: { children: ReactNode }) =>
    createElement(QueryClientProvider, { client: queryClient }, children);
  return { queryClient, wrapper };
}

beforeEach(() => {
  vi.clearAllMocks();
  workspaceState.activeWorkspaceId = "ws_alpha";
  resetSettingsRestartStore();
  vi.mocked(getSettingsGeneral).mockResolvedValue(envelope);
  vi.mocked(listSettingsApplyRecords).mockResolvedValue({ entries: [] });
  vi.mocked(reloadSettings).mockResolvedValue({
    active_config_hash: "sha256:test-active",
    active_generation: 1,
    applied: true,
    apply_record_id: "cfg_apply_reload",
    lifecycle: "live",
    next_action: "none",
    restart_required: false,
  });
  vi.mocked(getSettingsUpdate).mockResolvedValue(settingsUpdateBothAvailableFixture);
});

afterEach(() => {
  vi.restoreAllMocks();
});

describe("useSettingsGeneralPage", () => {
  it("loads the envelope and seeds the draft", async () => {
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useSettingsGeneralPage(), { wrapper });

    await waitFor(() => {
      expect(result.current.envelope).toBeTruthy();
      expect(result.current.draft).toEqual(envelope.config);
    });
  });

  it("records a restart-required applied label after a save mutation succeeds", async () => {
    vi.mocked(updateSettingsGeneral).mockResolvedValue({
      section: "general",
      scope: "user",
      applied: true,
      active_config_hash: "sha256:test-active",
      active_generation: 1,
      apply_record_id: "cfg_apply_test",
      lifecycle: "live",
      next_action: "none",
      restart_required: true,
      write_target: "global-config",
    });

    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useSettingsGeneralPage(), { wrapper });

    await waitFor(() => expect(result.current.draft).toBeTruthy());

    act(() => {
      result.current.setDraft({
        ...envelope.config,
        limits: { ...envelope.config.limits, max_concurrent_agents: 50 },
      });
      result.current.handleSave();
    });

    await waitFor(() => {
      expect(result.current.lastAppliedLabel).toContain("restart required");
    });
  });

  it("clears a dirty draft when the active workspace changes", async () => {
    const { wrapper } = createWrapper();
    const { result, rerender } = renderHook(() => useSettingsGeneralPage(), { wrapper });

    await waitFor(() => expect(result.current.draft).toBeTruthy());

    act(() => {
      result.current.setDraft({
        ...envelope.config,
        limits: { ...envelope.config.limits, max_concurrent_agents: 37 },
      });
    });

    await waitFor(() => expect(result.current.isDirty).toBe(true));

    act(() => {
      workspaceState.activeWorkspaceId = "ws_beta";
      rerender();
    });

    await waitFor(() => {
      expect(result.current.draft).toEqual(envelope.config);
      expect(result.current.isDirty).toBe(false);
    });
  });

  it("Should send the requested track to the apply endpoint and expose the daemon's answer", async () => {
    vi.mocked(applySettingsUpdate).mockResolvedValue({
      target: "runtime",
      status: "accepted",
      operation_id: "op-7f3a2c",
      message: "Started the runtime update.",
      holder: null,
    });
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useSettingsGeneralPage(), { wrapper });

    await waitFor(() => expect(result.current.update.data).toBeTruthy());

    act(() => {
      result.current.updateActions.apply("runtime");
    });

    await waitFor(() => {
      expect(applySettingsUpdate).toHaveBeenCalledWith({ target: "runtime" });
      expect(result.current.updateActions.result).toMatchObject({
        status: "accepted",
        operation_id: "op-7f3a2c",
      });
    });
  });

  it("Should keep a blocked apply answer verbatim instead of reporting success", async () => {
    vi.mocked(applySettingsUpdate).mockResolvedValue({
      target: "runtime",
      status: "blocked",
      message:
        "A runtime update is already in progress (holder pid 4242). Retry after it completes.",
      holder: {
        pid: 4242,
        pid_start_time: "2026-08-20T14:02:10Z",
        surface: "cli",
        executor_generation: "gen-1",
        lease_expires_at: "2026-08-20T14:07:11Z",
      },
    });
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useSettingsGeneralPage(), { wrapper });

    await waitFor(() => expect(result.current.update.data).toBeTruthy());

    act(() => {
      result.current.updateActions.apply("runtime");
    });

    await waitFor(() => {
      expect(result.current.updateActions.result).toMatchObject({ status: "blocked" });
    });
    expect(result.current.updateActions.result?.message).toContain("holder pid 4242");
    expect(result.current.updateActions.error).toBeNull();
  });

  it("Should cancel a dormant operation and expose the archived outcome", async () => {
    vi.mocked(getSettingsUpdate).mockResolvedValue(settingsUpdateStagedFixture);
    vi.mocked(cancelSettingsUpdate).mockResolvedValue({
      status: "canceled",
      operation_id: "op-7f3a2c",
      message: "Canceled dormant update operation; the update channel is free.",
      holder: null,
    });
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useSettingsGeneralPage(), { wrapper });

    await waitFor(() => expect(result.current.update.data?.operation).toBeTruthy());

    act(() => {
      result.current.updateActions.cancel();
    });

    await waitFor(() => {
      expect(cancelSettingsUpdate).toHaveBeenCalledTimes(1);
      expect(result.current.updateActions.cancelResult).toMatchObject({ status: "canceled" });
    });
  });
});
