import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { renderHook, waitFor } from "@testing-library/react";
import { createElement, type ReactNode } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { statusFixture } from "../../mocks/fixtures";
import { deriveDaemonConnectionStatus, useDaemonHealth } from "../use-daemon-health";
import { useDaemonStatus } from "../use-daemon-status";
import { useStatus } from "../use-status";

vi.mock("../../adapters/daemon-api", () => ({
  fetchStatus: vi.fn(),
}));

import { fetchStatus } from "../../adapters/daemon-api";

function createWrapper() {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false },
    },
  });
  return ({ children }: { children: ReactNode }) =>
    createElement(QueryClientProvider, { client: queryClient }, children);
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

  it("Should report connecting while the first shared status request is pending", () => {
    vi.mocked(fetchStatus).mockReturnValue(new Promise(() => undefined));

    const { result } = renderHook(() => useDaemonHealth(), {
      wrapper: createWrapper(),
    });

    expect(result.current.connectionStatus).toBe("connecting");
    expect(result.current.isInitialLoading).toBe(true);
  });

  it("Should report an error after the shared status request fails", async () => {
    vi.mocked(fetchStatus).mockRejectedValue(new Error("Network error"));

    const { result } = renderHook(() => useDaemonHealth(), {
      wrapper: createWrapper(),
    });

    await waitFor(
      () => {
        expect(result.current.connectionStatus).toBe("error");
      },
      { timeout: 3_000 }
    );

    expect(result.current.health).toBeUndefined();
    expect(fetchStatus).toHaveBeenCalledTimes(2);
  });
});
