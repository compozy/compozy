// Suite: Settings → Computers local execution and load balancing mutations (US-014, US-015)
// Invariant: Turning off local execution opens a confirmation dialog when local sessions are active,
//            writes PATCH /api/computers/local for run_agents, load_balancing, and local weight,
//            and writes PATCH /api/computers/{id} for per-computer load balancing weights.
// Boundary IN: useLocalComputerSettings hook over computers-api adapters.
// Boundary OUT: Backend daemon config persistence (covered by Go UT-100, UT-101, IT-090).

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, renderHook, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import { afterEach, describe, expect, it, vi } from "vitest";

import { apiClient } from "@/lib/api-client";

import * as computersApi from "../../adapters/computers-api";
import { useLocalComputerSettings } from "../use-local-computer-settings";

function createWrapper(queryClient: QueryClient) {
  return ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  );
}

describe("useLocalComputerSettings (US-014, US-015)", () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("opens confirmation when local active sessions exist and patches /api/computers/local on confirm", async () => {
    vi.spyOn(apiClient, "GET").mockResolvedValue({
      data: {
        sessions: [],
        page: { total: 2, has_more: false, next_cursor: null },
      },
      error: undefined,
      response: new Response(null, { status: 200 }),
    } as never);
    const patchLocalSpy = vi.spyOn(computersApi, "patchLocalComputer").mockResolvedValue({
      run_agents: false,
      load_balancing: false,
      load_balancing_weight: 50,
    });

    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    const { result } = renderHook(
      () => useLocalComputerSettings({ signedIn: true, localRunAgents: true }),
      { wrapper: createWrapper(queryClient) }
    );

    await waitFor(() => {
      expect(result.current.activeLocalSessions).toBe(2);
    });

    act(() => {
      result.current.handleRequestToggleRunAgents(false);
    });
    expect(result.current.disableAgentsOpen).toBe(true);
    expect(patchLocalSpy).not.toHaveBeenCalled();

    act(() => {
      result.current.handleConfirmDisableAgents();
    });
    await waitFor(() => {
      expect(patchLocalSpy).toHaveBeenCalledWith({ run_agents: false });
    });
    await waitFor(() => {
      expect(result.current.disableAgentsOpen).toBe(false);
    });
  });

  it("patches load_balancing and weights on /api/computers/local and /api/computers/{id}", async () => {
    vi.spyOn(apiClient, "GET").mockResolvedValue({
      data: {
        sessions: [],
        page: { total: 0, has_more: false, next_cursor: null },
      },
      error: undefined,
      response: new Response(null, { status: 200 }),
    } as never);
    const patchLocalSpy = vi.spyOn(computersApi, "patchLocalComputer").mockResolvedValue({
      run_agents: true,
      load_balancing: true,
      load_balancing_weight: 100,
    });
    const patchComputerSpy = vi.spyOn(computersApi, "patchComputer").mockResolvedValue({
      id: "cmp_devbox",
      account_id: "acc_1",
      name: "devbox",
      enabled: true,
      status: "connected",
      plan_state: "ok",
      version: "0.8.0",
      platform: "linux_x86_64",
      route_in_use: null,
      routes: [],
      scopes: [],
      diagnosis: null,
      protocol: { min: 1, max: 1 },
      load_balancing_weight: 25,
      added_at: "2026-11-10T09:00:00Z",
      last_connected_at: "2026-11-24T12:00:00Z",
    });

    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    const { result } = renderHook(
      () => useLocalComputerSettings({ signedIn: true, localRunAgents: true }),
      { wrapper: createWrapper(queryClient) }
    );

    act(() => {
      result.current.handleToggleLoadBalancing(true);
      result.current.handleChangeLocalWeight(100);
      result.current.handleChangeComputerWeight("cmp_devbox", 25);
    });

    await waitFor(() => {
      expect(patchLocalSpy).toHaveBeenCalledWith({ load_balancing: true });
      expect(patchLocalSpy).toHaveBeenCalledWith({ load_balancing_weight: 100 });
      expect(patchComputerSpy).toHaveBeenCalledWith("cmp_devbox", { load_balancing_weight: 25 });
    });
  });
});
