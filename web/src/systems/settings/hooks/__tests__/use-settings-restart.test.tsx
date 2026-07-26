import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, renderHook, waitFor } from "@testing-library/react";
import { createElement, type ReactNode } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("../../adapters/settings-api", () => ({
  getSettingsRestartStatus: vi.fn(),
  triggerSettingsRestart: vi.fn(),
}));

import { getSettingsRestartStatus, triggerSettingsRestart } from "../../adapters/settings-api";
import { initialSettingsRestartState } from "../../stores/settings-restart-store";
import {
  resetSettingsRestartStore,
  settingsRestartStorageKey,
  useSettingsRestartStore,
} from "../../stores/use-settings-restart-store";
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
      old_socket_path: "/tmp/agh.sock",
      old_started_at: "2026-04-17T10:00:00Z",
      active_session_count: 3,
      started_at: "2026-04-17T10:05:00Z",
      updated_at: "2026-04-17T10:05:01Z",
    });

    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useSettingsRestart(), { wrapper });

    expect(result.current.operationId).toBeNull();
    expect(result.current.isPolling).toBe(false);

    await act(async () => {
      await result.current.triggerAsync();
    });

    await waitFor(() => {
      expect(result.current.operationId).toBe("op_001");
    });

    await waitFor(() => {
      expect(result.current.status).toBe("stopping");
    });

    expect(result.current.isPolling).toBe(true);
    expect(result.current.isSuccessful).toBe(false);
    expect(result.current.isFailed).toBe(false);
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
      old_socket_path: "/tmp/agh.sock",
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

    expect(result.current.isSuccessful).toBe(true);
    expect(result.current.isPolling).toBe(false);
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
      old_socket_path: "/tmp/agh.sock",
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

    expect(result.current.isFailed).toBe(true);
    expect(result.current.failureReason).toBe("helper spawn failed");
  });

  it("reflects restart-required state from the most recent mutation", () => {
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useSettingsRestart(), { wrapper });

    expect(result.current.isRestartRequired).toBe(false);

    act(() => {
      useSettingsRestartStore.getState().recordMutation({
        section: "general",
        restartRequired: true,
        warnings: [],
        completedAt: new Date().toISOString(),
      });
    });

    expect(result.current.isRestartRequired).toBe(true);
  });

  it("rehydrates a pending restart so polling survives a page refresh", async () => {
    window.sessionStorage.setItem(
      settingsRestartStorageKey,
      JSON.stringify({
        state: {
          operationId: "op_refresh",
          status: "waiting_release",
          activeSessionCount: 2,
          failureReason: undefined,
          mutationGeneration: 1,
          snoozedMutationGeneration: null,
          lastMutation: {
            section: "general",
            restartRequired: true,
            warnings: [],
            completedAt: "2026-04-17T10:05:00Z",
          },
        } satisfies typeof initialSettingsRestartState,
        version: 0,
      })
    );

    vi.mocked(getSettingsRestartStatus).mockResolvedValue({
      operation_id: "op_refresh",
      status: "waiting_release",
      old_pid: 1000,
      old_socket_path: "/tmp/agh.sock",
      old_started_at: "2026-04-17T10:00:00Z",
      active_session_count: 2,
      started_at: "2026-04-17T10:05:00Z",
      updated_at: "2026-04-17T10:05:03Z",
    });

    await act(async () => {
      await useSettingsRestartStore.persist.rehydrate();
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
    expect(result.current.isPolling).toBe(true);
    expect(result.current.activeSessionCount).toBe(2);
  });

  it("snoozes only the current mutation generation while preserving restart truth", () => {
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useSettingsRestart(), { wrapper });

    act(() => {
      useSettingsRestartStore.getState().startRestart({
        operationId: "op_dismiss",
        status: "stopping",
        activeSessionCount: 0,
      });
      useSettingsRestartStore.getState().recordMutation({
        section: "general",
        restartRequired: true,
        warnings: [],
        completedAt: new Date().toISOString(),
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
      useSettingsRestartStore.getState().recordMutation({
        section: "memory",
        restartRequired: true,
        warnings: [],
        completedAt: new Date().toISOString(),
      });
    });

    expect(result.current.isRestartRequired).toBe(true);
    expect(result.current.isNoticeSnoozed).toBe(false);
  });
});
