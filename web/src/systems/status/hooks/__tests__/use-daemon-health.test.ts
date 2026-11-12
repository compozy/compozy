import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, renderHook, waitFor } from "@testing-library/react";
import { createElement, type ReactNode } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { runtimeIdentityFixture, statusFixture } from "../../mocks/fixtures";
import { daemonRecovery } from "../../lib/daemon-readiness";
import { statusKeys } from "../../lib/query-keys";
import type { StatusPayload } from "../../types";
import { deriveDaemonConnectionStatus, useDaemonHealth } from "../use-daemon-health";
import { useDaemonStatus } from "../use-daemon-status";
import { useStatus } from "../use-status";
import { useDaemonConnectionStatus } from "../use-daemon-connection-status";
import { useRuntimeIdentity } from "../use-runtime-identity";
import { useDaemonRecovery } from "../use-daemon-recovery";
import { useRuntimeReconnectRefresh } from "../use-runtime-reconnect-refresh";

vi.mock("../../adapters/daemon-api", () => ({
  fetchStatus: vi.fn(),
  fetchRuntimeIdentity: vi.fn(),
}));

import { fetchRuntimeIdentity, fetchStatus } from "../../adapters/daemon-api";

function createWrapper(queryClient?: QueryClient) {
  const client =
    queryClient ??
    new QueryClient({
      defaultOptions: {
        queries: { retry: false },
      },
    });
  return ({ children }: { children: ReactNode }) =>
    createElement(QueryClientProvider, { client }, children);
}

describe("deriveDaemonConnectionStatus", () => {
  it("Should map the shared status query lifecycle to a connection status", () => {
    expect(
      deriveDaemonConnectionStatus({
        isPending: true,
        isFetching: true,
        isSuccess: false,
        isError: false,
      })
    ).toBe("connecting");
    expect(
      deriveDaemonConnectionStatus({
        data: statusFixture.health,
        isPending: false,
        isFetching: false,
        isSuccess: true,
        isError: false,
      })
    ).toBe("connected");
    expect(
      deriveDaemonConnectionStatus({
        isPending: false,
        isFetching: false,
        isSuccess: false,
        isError: true,
      })
    ).toBe("error");
    expect(
      deriveDaemonConnectionStatus({
        isPending: false,
        isFetching: false,
        isSuccess: false,
        isError: false,
      })
    ).toBe("disconnected");
  });
});

describe("daemon status query projections", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("Should share one status request across envelope, daemon, and health consumers", async () => {
    vi.mocked(fetchStatus).mockResolvedValue(statusFixture);

    const { result } = renderHook(
      () => ({
        envelope: useStatus(),
        daemon: useDaemonStatus(),
        health: useDaemonHealth(),
      }),
      { wrapper: createWrapper() }
    );

    await waitFor(() => {
      expect(result.current.health.connectionStatus).toBe("connected");
    });

    expect(result.current.envelope.data).toEqual(statusFixture);
    expect(result.current.daemon.data).toEqual(statusFixture.daemon);
    expect(result.current.health.health).toEqual(statusFixture.health);
    expect(fetchStatus).toHaveBeenCalledTimes(1);
  });

  it("Should keep cached daemon data visible without fetching while disabled", () => {
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    queryClient.setQueryData(statusKeys.current(), statusFixture);

    const { result } = renderHook(() => useDaemonStatus({ enabled: false }), {
      wrapper: createWrapper(queryClient),
    });

    expect(result.current.data).toEqual(statusFixture.daemon);
    expect(fetchStatus).not.toHaveBeenCalled();
  });

  it("Should share bounded identity reads and retain refresh and failure behavior", async () => {
    vi.useFakeTimers();
    const restartedIdentity = {
      ...runtimeIdentityFixture,
      daemon: {
        ...runtimeIdentityFixture.daemon,
        pid: 5252,
        started_at: "2026-04-18T18:00:00Z",
        user_home_dir: "/workspace/new-home",
      },
    };
    vi.mocked(fetchRuntimeIdentity)
      .mockResolvedValueOnce(runtimeIdentityFixture)
      .mockResolvedValueOnce(restartedIdentity)
      .mockRejectedValue(new Error("Connection closed"));

    const { result, unmount } = renderHook(
      () => ({
        firstConnection: useDaemonConnectionStatus(),
        secondConnection: useDaemonConnectionStatus(),
        identity: useRuntimeIdentity(),
        diagnostics: useStatus({ enabled: false }),
      }),
      { wrapper: createWrapper() }
    );

    try {
      expect(result.current.firstConnection).toBe("connecting");
      await act(() => vi.advanceTimersByTimeAsync(1));
      expect(result.current.firstConnection).toBe("connected");
      expect(result.current.secondConnection).toBe("connected");
      expect(result.current.identity.data).toEqual(runtimeIdentityFixture);
      expect(fetchRuntimeIdentity).toHaveBeenCalledTimes(1);

      await act(() => vi.advanceTimersByTimeAsync(10_000));
      expect(result.current.identity.data).toEqual(restartedIdentity);
      expect(fetchRuntimeIdentity).toHaveBeenCalledTimes(2);

      await act(() => vi.advanceTimersByTimeAsync(11_100));
      expect(result.current.firstConnection).toBe("error");
      expect(result.current.secondConnection).toBe("error");
      expect(result.current.identity.data).toEqual(restartedIdentity);
      expect(fetchStatus).not.toHaveBeenCalled();
    } finally {
      unmount();
      vi.useRealTimers();
    }
  });

  it("Should report connecting while the first shared status request is pending", () => {
    vi.mocked(fetchStatus).mockReturnValue(new Promise(() => undefined));

    const { result } = renderHook(() => useDaemonHealth(), {
      wrapper: createWrapper(),
    });

    expect(result.current.connectionStatus).toBe("connecting");
    expect(result.current.isInitialLoading).toBe(true);
  });

  it("Should report an error after the shared status request fails", async () => {
    // The status projection owns the failed retry outcome, not wall-clock waiting.
    vi.useFakeTimers();
    vi.mocked(fetchStatus).mockRejectedValue(new Error("Network error"));

    const { result, unmount } = renderHook(() => useDaemonHealth(), {
      wrapper: createWrapper(),
    });

    try {
      await act(() => vi.advanceTimersByTimeAsync(1_100));
      expect(result.current.connectionStatus).toBe("error");
      expect(result.current.health).toBeUndefined();
      expect(fetchStatus).toHaveBeenCalledTimes(2);
    } finally {
      unmount();
      vi.useRealTimers();
    }
  });
});

/** A status answer from a runtime that reports its lifecycle phase. */
function statusInPhase(phase: string, step?: string): StatusPayload {
  const readiness = { phase, step, since: "2026-11-12T00:00:00Z" };
  const daemon = { ...statusFixture.daemon, readiness };
  // `readiness` joins the generated contract with the next codegen; the cast holds on both sides.
  return { ...statusFixture, daemon } as StatusPayload;
}

// Invariant: after a restart the shell says sessions are resuming exactly while the runtime is in
// its serving phase, names the step it knows, and returns to normal when the runtime is ready,
// without a page reload. Runtimes that report no phase never show it.
describe("daemon recovery after a restart", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("Should name the recovery step it knows and fall back for the rest", () => {
    const resuming = statusInPhase("serving", "resumeBootInterruptedSessions").daemon;
    expect(daemonRecovery(resuming)).toEqual({ label: "Resuming interrupted sessions" });
    const unnamed = statusInPhase("serving", "bootNewStep").daemon;
    expect(daemonRecovery(unnamed)).toEqual({ label: "Resuming your sessions" });
    expect(daemonRecovery(statusInPhase("serving").daemon)).toEqual({
      label: "Resuming your sessions",
    });
  });

  it.each(["starting", "ready", "stopping"])("Should show no recovery in the %s phase", phase => {
    expect(daemonRecovery(statusInPhase(phase, "bootFinalize").daemon)).toBeNull();
  });

  it("Should show no recovery for a runtime that reports no lifecycle phase", () => {
    expect(daemonRecovery(statusFixture.daemon)).toBeNull();
    expect(daemonRecovery(undefined)).toBeNull();
  });

  it("Should clear the recovery once the runtime is ready, without remounting", async () => {
    vi.mocked(fetchStatus).mockResolvedValue(statusInPhase("serving", "recoverBootMonitors"));
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    const { result, unmount } = renderHook(() => useDaemonRecovery(), {
      wrapper: createWrapper(queryClient),
    });

    try {
      await waitFor(() => {
        expect(result.current).toEqual({ label: "Restoring monitors" });
      });
      vi.mocked(fetchStatus).mockResolvedValue(statusInPhase("ready"));
      await act(() => queryClient.invalidateQueries({ queryKey: statusKeys.current() }));
      await waitFor(() => {
        expect(result.current).toBeNull();
      });
    } finally {
      unmount();
    }
  });
});

// Invariant: a runtime restart on the same origin refreshes the page's server state in place.
// Cached reads go stale when, and only when, a different runtime process answers.
describe("runtime reconnect refresh", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("Should mark cached reads stale only when another runtime process answers", async () => {
    vi.mocked(fetchRuntimeIdentity).mockResolvedValue(runtimeIdentityFixture);
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    const cached = ["sessions", "list"];
    queryClient.setQueryData(cached, []);
    const stale = () => queryClient.getQueryState(cached)?.isInvalidated;
    const { unmount } = renderHook(() => useRuntimeReconnectRefresh(), {
      wrapper: createWrapper(queryClient),
    });

    try {
      await waitFor(() => {
        expect(fetchRuntimeIdentity).toHaveBeenCalledTimes(1);
      });
      // The same process answering again is not a restart.
      await act(() => queryClient.refetchQueries({ queryKey: statusKeys.identity() }));
      expect(stale()).toBe(false);

      vi.mocked(fetchRuntimeIdentity).mockResolvedValue({
        ...runtimeIdentityFixture,
        daemon: { ...runtimeIdentityFixture.daemon, pid: 5252 },
      });
      await act(() => queryClient.refetchQueries({ queryKey: statusKeys.identity() }));
      await waitFor(() => {
        expect(stale()).toBe(true);
      });
    } finally {
      unmount();
    }
  });
});
