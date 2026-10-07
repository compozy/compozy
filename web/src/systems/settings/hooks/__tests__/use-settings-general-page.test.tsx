import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, renderHook, waitFor } from "@testing-library/react";
import { createElement, type ReactNode } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { statusFixture } from "@/systems/status/mocks";

vi.mock("@/systems/status/adapters/daemon-api", () => ({
  fetchStatus: vi.fn(async () => statusFixture),
}));

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
import { settingsRestartRequiredMutationFixture } from "@/systems/settings/mocks/fixtures";

import { useSettingsGeneralPage } from "../use-settings-general-page";
import type { SettingsGeneralSection, SettingsMutationResult } from "@/systems/settings";

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
      reload_timeouts: { mcp: "10s", providers: "5s" },
      socket: "/tmp/compozy.sock",
    },
    http: { host: "127.0.0.1", port: 2123 },
    limits: { max_concurrent_agents: 20 },
    permissions: { mode: "approve-all" },
    redact: { enabled: true },
    session_timeout: "0s",
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

function deferred<T>() {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>(next => {
    resolve = next;
  });
  return { promise, resolve };
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

  it("Should adopt the saved canonical duration and clear the acknowledged draft", async () => {
    const canonical = {
      ...envelope,
      config: { ...envelope.config, session_timeout: "4h0m0s" },
    };
    vi.mocked(updateSettingsGeneral).mockImplementation(async () => {
      vi.mocked(getSettingsGeneral).mockResolvedValue(canonical);
      return settingsRestartRequiredMutationFixture;
    });
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useSettingsGeneralPage(), { wrapper });
    await waitFor(() => expect(result.current.draft).toBeTruthy());

    act(() => {
      result.current.setDraft({ ...envelope.config, session_timeout: "14400s" });
    });
    expect(result.current.isDirty).toBe(true);
    act(() => result.current.handleSave());

    await waitFor(() => {
      expect(result.current.isSaving).toBe(false);
      expect(result.current.draft?.session_timeout).toBe("4h0m0s");
      expect(result.current.isDirty).toBe(false);
    });
    expect(updateSettingsGeneral).toHaveBeenCalledExactlyOnceWith({
      config: { ...envelope.config, session_timeout: "14400s" },
    });
    expect(result.current.lastAppliedLabel).toContain("restart required");
  });

  it.each(["ws_alpha", "ws_beta"])(
    "Should retain a newer draft in %s when an earlier save finishes",
    async workspace => {
      const pending = deferred<SettingsMutationResult>();
      vi.mocked(updateSettingsGeneral).mockReturnValue(pending.promise);
      const { wrapper } = createWrapper();
      const { result, rerender } = renderHook(() => useSettingsGeneralPage(), { wrapper });
      await waitFor(() => expect(result.current.draft).toBeTruthy());

      act(() => {
        result.current.setDraft({ ...envelope.config, session_timeout: "14400s" });
      });
      act(() => result.current.handleSave());
      await waitFor(() => expect(updateSettingsGeneral).toHaveBeenCalledTimes(1));

      act(() => {
        workspaceState.activeWorkspaceId = workspace;
        rerender();
      });
      act(() => {
        result.current.setDraft({ ...envelope.config, session_timeout: "3600s" });
      });
      vi.mocked(getSettingsGeneral).mockResolvedValue({
        ...envelope,
        config: { ...envelope.config, session_timeout: "4h0m0s" },
      });
      await act(async () => pending.resolve(settingsRestartRequiredMutationFixture));

      await waitFor(() => {
        expect(result.current.isSaving).toBe(false);
        expect(result.current.envelope?.config.session_timeout).toBe("4h0m0s");
      });
      expect(result.current.draft?.session_timeout).toBe("3600s");
      expect(result.current.isDirty).toBe(true);
    }
  );

  it("Should retain the dirty draft when the save fails", async () => {
    vi.mocked(updateSettingsGeneral).mockRejectedValue(new Error("Couldn't save General"));
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useSettingsGeneralPage(), { wrapper });
    await waitFor(() => expect(result.current.draft).toBeTruthy());

    act(() => {
      result.current.setDraft({ ...envelope.config, session_timeout: "14400s" });
    });
    act(() => result.current.handleSave());

    await waitFor(() => expect(result.current.saveError).toBe("Couldn't save General"));
    expect(result.current.draft?.session_timeout).toBe("14400s");
    expect(result.current.isDirty).toBe(true);
    expect(result.current.lastAppliedLabel).toBeNull();
  });

  it("Should send all requested targets to the apply endpoint and expose the daemon's answer", async () => {
    vi.mocked(applySettingsUpdate).mockResolvedValue({
      targets: ["runtime", "app"],
      status: "accepted",
      operation_id: "op-7f3a2c",
      message: "Started the runtime update.",
      holder: null,
    });
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useSettingsGeneralPage(), { wrapper });

    await waitFor(() => expect(result.current.update.data).toBeTruthy());

    act(() => {
      result.current.updateActions.apply(["runtime", "app"]);
    });

    await waitFor(() => {
      expect(applySettingsUpdate).toHaveBeenCalledWith({ targets: ["runtime", "app"] });
      expect(result.current.updateActions.result).toMatchObject({
        status: "accepted",
        operation_id: "op-7f3a2c",
      });
    });
  });

  it("Should keep a blocked apply answer verbatim instead of reporting success", async () => {
    vi.mocked(applySettingsUpdate).mockResolvedValue({
      targets: ["runtime"],
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
      result.current.updateActions.apply(["runtime"]);
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
