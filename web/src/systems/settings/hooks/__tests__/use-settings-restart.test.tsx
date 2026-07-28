import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { rehydrateStore } from "@xstate/store/persist";
import { act, renderHook, waitFor } from "@testing-library/react";
import { createElement, type ReactNode } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("../../adapters/settings-api", async importOriginal => {
  const actual = await importOriginal<typeof import("../../adapters/settings-api")>();
  return {
    ...actual,
    getSettingsRestartStatus: vi.fn(),
    triggerSettingsRestart: vi.fn(),
  };
});

import {
  getSettingsRestartStatus,
  SettingsApiError,
  triggerSettingsRestart,
} from "../../adapters/settings-api";
import { resetSettingsRestartStore } from "../../stores/use-settings-restart-store";
import {
  settingsRestartStorageKey,
  settingsRestartStore,
} from "../../stores/settings-restart-store";
import { useSettingsRestart } from "../use-settings-restart";

function createWrapper() {
  const queryClient = new QueryClient({
    defaultOptions: { mutations: { retry: false }, queries: { retry: false } },
  });

  const wrapper = ({ children }: { children: ReactNode }) =>
    createElement(QueryClientProvider, { client: queryClient }, children);

  return { queryClient, wrapper };
}

beforeEach(() => {
  vi.clearAllMocks();
  resetSettingsRestartStore();
});

afterEach(() => {
  vi.restoreAllMocks();
});

describe("useSettingsRestart", () => {
  it("starts the restart operation and exposes polling state", async () => {
    vi.mocked(triggerSettingsRestart).mockResolvedValue({
      operation_id: "op_001",
      status: "pending",
      status_url: "/api/settings/actions/restart/op_001",
      active_session_count: 3,
    });

    vi.mocked(getSettingsRestartStatus).mockResolvedValue({
      operation_id: "op_001",
      status: "stopping",
      old_pid: 1000,
      old_socket_path: "/tmp/compozy.sock",
      old_started_at: "2026-04-17T10:00:00Z",
      active_session_count: 3,
      started_at: "2026-04-17T10:05:00Z",
      updated_at: "2026-04-17T10:05:01Z",
    });

    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useSettingsRestart(), { wrapper });

    expect(result.current.operationId).toBeNull();
    expect(result.current.status).toBeNull();
    expect(result.current).not.toHaveProperty("isPolling");
    expect(result.current).not.toHaveProperty("isSuccessful");
    expect(result.current).not.toHaveProperty("isFailed");

    await act(async () => {
      await result.current.triggerAsync();
    });

    await waitFor(() => {
      expect(result.current.operationId).toBe("op_001");
    });

    await waitFor(() => {
      expect(result.current.status).toBe("stopping");
    });

    expect(result.current.activeSessionCount).toBe(3);
  });

  it("transitions to successful when status reaches ready", async () => {
    vi.mocked(triggerSettingsRestart).mockResolvedValue({
      operation_id: "op_002",
      status: "pending",
      status_url: "/api/settings/actions/restart/op_002",
      active_session_count: 0,
    });

    vi.mocked(getSettingsRestartStatus).mockResolvedValue({
      operation_id: "op_002",
      status: "ready",
      old_pid: 1000,
      old_socket_path: "/tmp/compozy.sock",
      old_started_at: "2026-04-17T10:00:00Z",
      new_pid: 2000,
      active_session_count: 0,
      started_at: "2026-04-17T10:05:00Z",
      updated_at: "2026-04-17T10:05:05Z",
    });

    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useSettingsRestart(), { wrapper });

    await act(async () => {
      await result.current.triggerAsync();
    });

    await waitFor(() => {
      expect(result.current.status).toBe("ready");
    });

    expect(result.current.status).toBe("ready");
  });

  it("captures failure reason when status reaches failed", async () => {
    vi.mocked(triggerSettingsRestart).mockResolvedValue({
      operation_id: "op_003",
      status: "pending",
      status_url: "/api/settings/actions/restart/op_003",
      active_session_count: 1,
    });

    vi.mocked(getSettingsRestartStatus).mockResolvedValue({
      operation_id: "op_003",
      status: "failed",
      old_pid: 1000,
      old_socket_path: "/tmp/compozy.sock",
      old_started_at: "2026-04-17T10:00:00Z",
      active_session_count: 1,
      started_at: "2026-04-17T10:05:00Z",
      updated_at: "2026-04-17T10:05:06Z",
      failure_reason: "helper spawn failed",
    });

    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useSettingsRestart(), { wrapper });

    await act(async () => {
      await result.current.triggerAsync();
    });

    await waitFor(() => {
      expect(result.current.status).toBe("failed");
    });

    expect(result.current.failureReason).toBe("helper spawn failed");
  });

  it("reflects restart-required state from the most recent mutation", () => {
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useSettingsRestart(), { wrapper });

    expect(result.current.isRestartRequired).toBe(false);

    act(() => {
      settingsRestartStore.trigger.settingsMutationRecorded({
        mutation: {
          section: "general",
          restartRequired: true,
          warnings: [],
          completedAt: new Date().toISOString(),
        },
      });
    });

    expect(result.current.isRestartRequired).toBe(true);
  });

  it("rehydrates a pending restart so polling survives a page refresh", async () => {
    window.sessionStorage.setItem(
      settingsRestartStorageKey,
      JSON.stringify({
        context: {
          operationId: "op_refresh",
          mutationGeneration: 1,
          snoozedMutationGeneration: null,
          lastMutation: {
            section: "general",
            restartRequired: true,
            warnings: [],
            completedAt: "2026-04-17T10:05:00Z",
          },
        },
        version: 0,
      })
    );

    vi.mocked(getSettingsRestartStatus).mockResolvedValue({
      operation_id: "op_refresh",
      status: "waiting_release",
      old_pid: 1000,
      old_socket_path: "/tmp/compozy.sock",
      old_started_at: "2026-04-17T10:00:00Z",
      active_session_count: 2,
      started_at: "2026-04-17T10:05:00Z",
      updated_at: "2026-04-17T10:05:03Z",
    });

    await act(async () => {
      await rehydrateStore(settingsRestartStore);
    });

    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useSettingsRestart(), { wrapper });

    await waitFor(() => {
      expect(result.current.operationId).toBe("op_refresh");
    });

    await waitFor(() => {
      expect(result.current.status).toBe("waiting_release");
    });

    expect(result.current.isRestartRequired).toBe(true);
    expect(result.current.activeSessionCount).toBe(2);
  });

  it("Should expose a dismissible restart-required fallback when a persisted operation is gone", async () => {
    settingsRestartStore.trigger.settingsMutationRecorded({
      mutation: {
        section: "general",
        restartRequired: true,
        warnings: [],
        completedAt: "2026-04-17T10:05:00Z",
      },
    });
    settingsRestartStore.trigger.restartOperationStarted({ operationId: "op_missing" });
    vi.mocked(getSettingsRestartStatus).mockRejectedValue(
      new SettingsApiError("Restart operation not found", 404)
    );

    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useSettingsRestart(), { wrapper });

    await waitFor(() => expect(result.current.statusQueryError).toBeTruthy());
    expect(result.current.operationId).toBeNull();
    expect(result.current.isRestartRequired).toBe(true);
    expect(result.current.status).toBeNull();
  });

  it("Should reset the restart singleton to its exact initial snapshot", () => {
    settingsRestartStore.trigger.settingsMutationRecorded({
      mutation: {
        section: "memory",
        restartRequired: true,
        warnings: [],
        completedAt: "2026-04-17T10:05:00Z",
      },
    });
    settingsRestartStore.trigger.restartOperationStarted({ operationId: "op_test" });

    resetSettingsRestartStore();

    expect(settingsRestartStore.getSnapshot().context).toEqual({
      operationId: null,
      lastMutation: null,
      mutationGeneration: 0,
      snoozedMutationGeneration: null,
    });
  });

  it("snoozes only the current mutation generation while preserving restart truth", () => {
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useSettingsRestart(), { wrapper });

    act(() => {
      settingsRestartStore.trigger.restartOperationStarted({
        operationId: "op_dismiss",
      });
      settingsRestartStore.trigger.settingsMutationRecorded({
        mutation: {
          section: "general",
          restartRequired: true,
          warnings: [],
          completedAt: new Date().toISOString(),
        },
      });
    });

    act(() => {
      result.current.dismiss();
    });

    expect(result.current.operationId).toBeNull();
    expect(result.current.status).toBeNull();
    expect(result.current.isRestartRequired).toBe(true);
    expect(result.current.isNoticeSnoozed).toBe(true);

    act(() => {
      settingsRestartStore.trigger.settingsMutationRecorded({
        mutation: {
          section: "memory",
          restartRequired: true,
          warnings: [],
          completedAt: new Date().toISOString(),
        },
      });
    });

    expect(result.current.isRestartRequired).toBe(true);
    expect(result.current.isNoticeSnoozed).toBe(false);
  });

  it("keeps only the restart identity in the persisted store", () => {
    settingsRestartStore.trigger.restartOperationStarted({ operationId: "op_current" });

    const context = settingsRestartStore.getSnapshot().context;
    expect(context.operationId).toBe("op_current");
    expect("status" in context).toBe(false);
    expect("operation" in context).toBe(false);
  });

  it("keeps an active restart lifecycle when the settings route remounts", () => {
    settingsRestartStore.trigger.restartOperationStarted({
      operationId: "op_remount",
    });

    const { wrapper } = createWrapper();
    const first = renderHook(() => useSettingsRestart(), { wrapper });
    first.unmount();

    const second = renderHook(() => useSettingsRestart(), { wrapper });
    expect(second.result.current.operationId).toBe("op_remount");
  });
});
