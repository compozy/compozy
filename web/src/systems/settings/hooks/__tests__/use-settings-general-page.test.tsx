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

vi.mock("@/systems/workspace", () => ({
  useActiveWorkspace: () => ({
    activeWorkspaceId: workspaceState.activeWorkspaceId,
  }),
}));

vi.mock("@/systems/settings/adapters/settings-api", () => ({
  getSettingsGeneral: vi.fn(),
  listSettingsApplyRecords: vi.fn(),
  getSettingsUpdate: vi.fn(),
  reloadSettings: vi.fn(),
  updateSettingsGeneral: vi.fn(),
  getSettingsRestartStatus: vi.fn(),
  triggerSettingsRestart: vi.fn(),
  SettingsApiError: class SettingsApiError extends Error {
    status = 500;
  },
}));

import {
  getSettingsGeneral,
  listSettingsApplyRecords,
  getSettingsUpdate,
  reloadSettings,
  updateSettingsGeneral,
} from "@/systems/settings/adapters/settings-api";
import { initialSettingsRestartState } from "@/systems/settings/stores/settings-restart-store";
import { useSettingsRestartStore } from "@/systems/settings/stores/use-settings-restart-store";
import type { SettingsGeneralSection } from "@/systems/settings";
import { useSettingsGeneralPage } from "../use-settings-general-page";

const envelope: SettingsGeneralSection = {
  section: "general",
  scope: "global",
  available_scopes: ["global"],
  actions: {
    restart: { available: true, behavior: "action_trigger", name: "restart" },
  },
  config: {
    daemon: {
      memory_report_interval: "5m",
      reload_timeouts: { bridges: "30s", mcp: "10s", providers: "5s" },
      socket: "/tmp/agh.sock",
    },
    defaults: { agent: "general", provider: "claude" },
    http: { host: "127.0.0.1", port: 2123 },
    limits: { max_concurrent_agents: 20 },
    permissions: { mode: "approve-all" },
    redact: { enabled: true },
    session_timeout: "0s",
  },
  config_paths: {
    daemon_info: "/tmp/daemon.json",
    global_config: "~/.agh/config.toml",
    global_mcp_sidecar: "~/.agh/mcp.json",
    home_dir: "~/.agh",
    log_file: "~/.agh/agh.log",
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
  useSettingsRestartStore.setState({
    ...initialSettingsRestartState,
    startRestart: useSettingsRestartStore.getState().startRestart,
    updateRestart: useSettingsRestartStore.getState().updateRestart,
    clearRestart: useSettingsRestartStore.getState().clearRestart,
    recordMutation: useSettingsRestartStore.getState().recordMutation,
  });
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
  vi.mocked(getSettingsUpdate).mockResolvedValue({
    supported: true,
    managed: false,
    install_method: "direct-binary",
    current_version: "v1.0.0",
    latest_version: "v1.1.0",
    available: true,
    status: "available",
    recommendation: "Run `agh update`.",
    release_url: "https://github.com/compozy/agh/releases/tag/v1.1.0",
    checked_at: "2026-05-03T19:00:00Z",
  });
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
      scope: "global",
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
        defaults: { ...envelope.config.defaults, provider: "stale-provider" },
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
});
