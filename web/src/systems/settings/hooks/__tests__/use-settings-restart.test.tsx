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
    listSettingsApplyRecords: vi.fn(),
    triggerSettingsRestart: vi.fn(),
  };
});

vi.mock("@/systems/status/adapters/daemon-api", async importOriginal => {
  const actual = await importOriginal<typeof import("@/systems/status/adapters/daemon-api")>();
  return { ...actual, fetchStatus: vi.fn() };
});

import { fetchStatus, statusKeys } from "@/systems/status";
import { statusFixture } from "@/systems/status/mocks";

import {
  getSettingsRestartStatus,
  listSettingsApplyRecords,
  SettingsApiError,
  triggerSettingsRestart,
} from "../../adapters/settings-api";
import { settingsKeys } from "../../lib/query-keys";
import { settingsRestartPresentation } from "../../lib/restart-presentation";
import { settingsApplyRecordsFixture } from "../../mocks/fixtures";
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
  vi.mocked(fetchStatus).mockRejectedValue(new Error("Runtime status unavailable"));
  vi.mocked(listSettingsApplyRecords).mockResolvedValue({ entries: [] });
});

afterEach(() => {
  vi.restoreAllMocks();
});

describe("useSettingsRestart", () => {
  it("keeps live restart fields current and preserves the shared status response", async () => {
    vi.mocked(fetchStatus).mockResolvedValue(statusFixture);
    const { wrapper, queryClient } = createWrapper();
    const { result } = renderHook(() => useSettingsRestart(), { wrapper });
    await waitFor(() => expect(result.current.activeSessionCount).toBe(3));
    expect(result.current.isRestartRequired).toBe(false);

    const busyStatus = {
      ...statusFixture,
      daemon: { ...statusFixture.daemon, active_sessions: 120 },
    };
    act(() => queryClient.setQueryData(statusKeys.current(), busyStatus));
    await waitFor(() => expect(result.current.activeSessionCount).toBe(120));
    expect(result.current.isRestartRequired).toBe(false);

    const pendingRestartStatus = {
      ...busyStatus,
      config: { ...statusFixture.config, restart_required: true },
    };
    act(() => queryClient.setQueryData(statusKeys.current(), pendingRestartStatus));
    await waitFor(() => expect(result.current.isRestartRequired).toBe(true));
    expect(result.current.activeSessionCount).toBe(120);
    expect(queryClient.getQueryData(statusKeys.current())).toEqual(pendingRestartStatus);
  });

  it("Should discover external restart requirements and snooze only the observed candidate", async () => {
    vi.mocked(fetchStatus).mockResolvedValue(statusFixture);
    vi.mocked(getSettingsRestartStatus).mockResolvedValue({
      operation_id: "op_previous",
      status: "ready",
      old_pid: 1000,
      old_socket_path: "/tmp/compozy.sock",
      old_started_at: "2026-04-17T10:00:00Z",
      active_session_count: 0,
      started_at: "2026-04-17T10:05:00Z",
      updated_at: "2026-04-17T10:05:05Z",
    });
    settingsRestartStore.trigger.restartOperationStarted({ operationId: "op_previous" });
    const { wrapper, queryClient } = createWrapper();
    const { result } = renderHook(() => useSettingsRestart(), { wrapper });
    await waitFor(() => expect(result.current.status).toBe("ready"));
    expect(result.current.isRestartRequired).toBe(false);

    vi.mocked(fetchStatus).mockResolvedValue({
      ...statusFixture,
      config: { ...statusFixture.config, restart_required: true, apply_state: "pending_restart" },
    });
    vi.mocked(listSettingsApplyRecords).mockResolvedValue({
      entries: [{ ...settingsApplyRecordsFixture.entries[0], id: "external-candidate-one" }],
    });
    await act(async () => {
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: statusKeys.current() }),
        queryClient.invalidateQueries({ queryKey: settingsKeys.applyRoot() }),
      ]);
    });
    await waitFor(() => expect(result.current.isRestartRequired).toBe(true));
    act(() => result.current.dismiss());
    expect(result.current.isNoticeSnoozed).toBe(true);

    vi.mocked(listSettingsApplyRecords).mockResolvedValue({
      entries: [{ ...settingsApplyRecordsFixture.entries[0], id: "external-candidate-two" }],
    });
    await act(async () => {
      await queryClient.invalidateQueries({ queryKey: settingsKeys.applyRoot() });
    });
    await waitFor(() => expect(result.current.isNoticeSnoozed).toBe(false));
    expect(result.current.isRestartRequired).toBe(true);

    vi.mocked(fetchStatus).mockResolvedValue(statusFixture);
    await act(async () => {
      await queryClient.invalidateQueries({ queryKey: statusKeys.current() });
    });
    await waitFor(() => expect(result.current.isRestartRequired).toBe(false));
  });

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
    expect(settingsRestartStore.getSnapshot().context).toMatchObject({
      operationId: "op_refresh",
      mutationGeneration: 1,
      snoozedMutationGeneration: null,
      snoozedApplyRecordId: null,
      lastMutation: { section: "general", restartRequired: true },
    });
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

  // Invariant: the daemon is unreachable while it restarts; that outage keeps the operation
  // polling as "Restarting…" until the replacement reports a terminal status.
  it("Should keep the restart operation while the restarting daemon is unreachable", async () => {
    settingsRestartStore.trigger.restartOperationStarted({ operationId: "op_outage" });
    const stopping = {
      operation_id: "op_outage",
      status: "stopping" as const,
      old_pid: 1000,
      old_socket_path: "/tmp/compozy.sock",
      old_started_at: "2026-04-17T10:00:00Z",
      active_session_count: 0,
      started_at: "2026-04-17T10:05:00Z",
      updated_at: "2026-04-17T10:05:01Z",
    };
    vi.mocked(getSettingsRestartStatus).mockResolvedValue(stopping);
    const { wrapper, queryClient } = createWrapper();
    const { result } = renderHook(() => useSettingsRestart(), { wrapper });
    await waitFor(() => expect(result.current.status).toBe("stopping"));

    vi.mocked(getSettingsRestartStatus).mockRejectedValue(new TypeError("Failed to fetch"));
    await act(async () => {
      await queryClient.refetchQueries({ queryKey: settingsKeys.restartStatus("op_outage") });
    });
    await waitFor(() => expect(result.current.statusQueryError).toBeTruthy());
    expect(result.current.operationId).toBe("op_outage");
    expect(result.current.status).toBe("stopping");
    expect(result.current.isOutageProlonged).toBe(false);

    // The poll interval, not a manual refetch, carries the notice to the replacement's status.
    vi.mocked(getSettingsRestartStatus).mockResolvedValue({
      ...stopping,
      status: "ready",
      new_pid: 2000,
      updated_at: "2026-04-17T10:05:09Z",
    });
    await waitFor(() => expect(result.current.status).toBe("ready"), { timeout: 5_000 });
    expect(result.current.operationId).toBe("op_outage");
  });

  // Invariant: guidance for a prolonged outage counts from the restart's start, so a page
  // refreshed mid-outage (no successful status read yet) still shows it.
  it("Should show prolonged-outage guidance after a refresh during a long restart", async () => {
    settingsRestartStore.trigger.restartOperationStarted({
      operationId: "op_long",
      startedAt: Date.now() - 60_000,
    });
    vi.mocked(getSettingsRestartStatus).mockRejectedValue(new TypeError("Failed to fetch"));
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useSettingsRestart(), { wrapper });

    await waitFor(() => expect(result.current.statusQueryError).toBeTruthy(), { timeout: 5_000 });
    expect(result.current.operationId).toBe("op_long");
    expect(result.current.isOutageProlonged).toBe(true);
  });

  // Invariant: a restart persisted before the start time existed is upgraded on rehydration, so
  // its prolonged-outage guidance still appears and polling continues.
  it("Should time a restart rehydrated from the previous persisted version", async () => {
    window.sessionStorage.setItem(
      settingsRestartStorageKey,
      JSON.stringify({
        context: {
          operationId: "op_upgraded",
          lastMutation: null,
          mutationGeneration: 1,
          snoozedMutationGeneration: null,
          snoozedApplyRecordId: null,
        },
        version: 1,
      })
    );
    const rehydratedAt = Date.now() - 60_000;
    const now = vi.spyOn(Date, "now").mockReturnValue(rehydratedAt);
    await act(async () => {
      await rehydrateStore(settingsRestartStore);
    });
    now.mockRestore();
    expect(settingsRestartStore.getSnapshot().context.operationStartedAt).toBe(rehydratedAt);

    vi.mocked(getSettingsRestartStatus).mockRejectedValue(new TypeError("Failed to fetch"));
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useSettingsRestart(), { wrapper });

    await waitFor(() => expect(result.current.isOutageProlonged).toBe(true), { timeout: 5_000 });
    const polls = vi.mocked(getSettingsRestartStatus).mock.calls.length;
    await waitFor(
      () => expect(vi.mocked(getSettingsRestartStatus).mock.calls.length).toBeGreaterThan(polls),
      { timeout: 5_000 }
    );
    expect(result.current.operationId).toBe("op_upgraded");
  });

  it("Should fall back to the restart action when the status endpoint returns an HTTP error", async () => {
    settingsRestartStore.trigger.settingsMutationRecorded({
      mutation: {
        section: "general",
        restartRequired: true,
        warnings: [],
        completedAt: "2026-04-17T10:05:00Z",
      },
    });
    settingsRestartStore.trigger.restartOperationStarted({ operationId: "op_broken" });
    vi.mocked(getSettingsRestartStatus).mockRejectedValue(
      new SettingsApiError("Failed to load restart status", 500)
    );
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useSettingsRestart(), { wrapper });

    await waitFor(() => expect(result.current.statusQueryError).toBeTruthy(), { timeout: 5_000 });
    expect(result.current.operationId).toBeNull();
    expect(result.current.isNoticeSnoozed).toBe(false);
    const presentation = settingsRestartPresentation({
      isVisible: result.current.isRestartRequired && !result.current.isNoticeSnoozed,
      isRestartRequired: result.current.isRestartRequired,
      operationId: result.current.operationId,
      status: result.current.status,
      activeSessionCount: result.current.activeSessionCount,
      isTriggerPending: result.current.isTriggerPending,
      isOutageProlonged: result.current.isOutageProlonged,
      trigger: () => undefined,
      dismiss: () => undefined,
    });
    expect(presentation).toMatchObject({ phase: "required", triggerLabel: "Restart CompozyOS" });
  });

  it("Should reset the restart singleton to its exact initial snapshot", () => {
    settingsRestartStore.trigger.settingsMutationRecorded({
      mutation: {
        section: "roles",
        restartRequired: true,
        warnings: [],
        completedAt: "2026-04-17T10:05:00Z",
      },
    });
    settingsRestartStore.trigger.restartOperationStarted({ operationId: "op_test" });

    resetSettingsRestartStore();

    expect(settingsRestartStore.getSnapshot().context).toEqual({
      operationId: null,
      operationStartedAt: null,
      lastMutation: null,
      mutationGeneration: 0,
      snoozedMutationGeneration: null,
      snoozedApplyRecordId: null,
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
          section: "roles",
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
