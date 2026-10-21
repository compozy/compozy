// Suite: Provider usage data hooks
// Invariant: One profile-scoped envelope read feeds the view model; popover open sends only a
// background refresh for rows older than 60 s; operator actions are the only explicit
// refreshes; an unreachable daemon keeps the cached view model; every stream frame re-reads.
// Boundary IN: hooks + adapter over a stubbed `fetch` and a fake event source.
// Boundary OUT: rendering (components suite), daemon refresh semantics (Go service suite).
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, renderHook, waitFor } from "@testing-library/react";
import { createElement, type ReactNode } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import type { StreamEventSource } from "@/lib/ticketed-event-source";
import { fetchRequest } from "@/test/fetch-test-utils";

import {
  getProviderUsage,
  ProviderUsageApiError,
  refreshProviderUsage,
} from "../../adapters/provider-usage-api";
import { providerUsageKeys } from "../../lib/query-keys";
import {
  codexOkTarget,
  defaultUsageEnvelope,
  usageEnvelope,
  usageTarget,
} from "../../mocks/fixtures";
import { useProviderUsage } from "../use-provider-usage";
import { useProviderUsageStream } from "../use-provider-usage-stream";
import {
  useProviderUsageActions,
  useProviderUsageOpenRefresh,
} from "../use-refresh-provider-usage";

function jsonResponse(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

function setup() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  const wrapper = ({ children }: { children: ReactNode }) =>
    createElement(QueryClientProvider, { client: queryClient }, children);
  return { queryClient, wrapper };
}

async function requestBody(callIndex: number): Promise<unknown> {
  return fetchRequest(callIndex).clone().json();
}

function refreshCalls(): number[] {
  return vi
    .mocked(globalThis.fetch)
    .mock.calls.map(([input], index) => ({ input, index }))
    .filter(({ input }) => input instanceof Request && input.url.endsWith("/refresh"))
    .map(({ index }) => index);
}

describe("provider usage hooks", () => {
  beforeEach(() => {
    vi.stubGlobal("fetch", vi.fn());
  });
  afterEach(() => {
    vi.unstubAllGlobals();
    vi.restoreAllMocks();
  });

  describe("adapter", () => {
    it("Should read the profile-scoped envelope", async () => {
      vi.mocked(globalThis.fetch).mockResolvedValue(jsonResponse(defaultUsageEnvelope()));
      const envelope = await getProviderUsage("work");
      expect(envelope.revision).toBe(14);
      expect(new URL(fetchRequest().url).search).toBe("?profile=work");
    });

    it("Should surface flat and profile error codes", async () => {
      vi.mocked(globalThis.fetch).mockResolvedValueOnce(
        jsonResponse(
          { error: "provider usage is turned off", code: "provider_usage_disabled" },
          409
        )
      );
      await expect(refreshProviderUsage({ intent: "explicit" })).rejects.toMatchObject({
        status: 409,
        code: "provider_usage_disabled",
        message: "provider usage is turned off",
      });
      vi.mocked(globalThis.fetch).mockResolvedValueOnce(
        jsonResponse(
          { error: { code: "profile_not_found", message: 'profile "nope" not found' } },
          404
        )
      );
      await expect(getProviderUsage("nope")).rejects.toMatchObject({
        status: 404,
        code: "profile_not_found",
        message: 'profile "nope" not found',
      });
    });

    it("Should report an unreachable daemon as status 0", async () => {
      vi.mocked(globalThis.fetch).mockRejectedValue(new TypeError("Failed to fetch"));
      const error = await getProviderUsage("default").catch((caught: unknown) => caught);
      expect(error).toBeInstanceOf(ProviderUsageApiError);
      expect((error as ProviderUsageApiError).unreachable).toBe(true);
    });
  });

  it("Should keep cached rows and flag the daemon unavailable after a network error (UT-062)", async () => {
    const { queryClient, wrapper } = setup();
    vi.mocked(globalThis.fetch).mockResolvedValueOnce(jsonResponse(defaultUsageEnvelope()));
    const { result } = renderHook(() => useProviderUsage("default"), { wrapper });
    await waitFor(() => expect(result.current.viewModel?.rows).toHaveLength(2));

    vi.mocked(globalThis.fetch).mockRejectedValueOnce(new TypeError("Failed to fetch"));
    await act(() =>
      queryClient.refetchQueries({ queryKey: providerUsageKeys.envelope("default") })
    );
    await waitFor(() => expect(result.current.daemonUnavailable).toBe(true));
    expect(result.current.viewModel?.rows).toHaveLength(2);
    expect(result.current.errorMessage).toBeNull();
  });

  it("Should send one background refresh for rows older than 60 s on open (UT-054)", async () => {
    const { queryClient, wrapper } = setup();
    const now = Date.now();
    const fresh = { ...codexOkTarget(now), measured_at: new Date(now - 30_000).toISOString() };
    const old = usageTarget({
      provider_id: "claude",
      provider_name: "Claude",
      measured_at: new Date(now - 90_000).toISOString(),
    });
    const envelope = usageEnvelope([old, fresh], {}, now);
    queryClient.setQueryData(providerUsageKeys.envelope("default"), envelope);
    vi.mocked(globalThis.fetch).mockResolvedValue(jsonResponse(envelope));

    const { result } = renderHook(() => useProviderUsageOpenRefresh("default"), { wrapper });
    act(() => result.current());
    await waitFor(() => expect(refreshCalls()).toHaveLength(1));
    expect(await requestBody(refreshCalls()[0] ?? -1)).toEqual({
      profile: "default",
      intent: "background",
      provider_ids: ["claude"],
      max_age_seconds: 60,
    });
  });

  it("Should send nothing on open when every row is fresh", () => {
    const { queryClient, wrapper } = setup();
    const now = Date.now();
    queryClient.setQueryData(
      providerUsageKeys.envelope("default"),
      usageEnvelope([{ ...codexOkTarget(now), measured_at: new Date(now).toISOString() }], {}, now)
    );
    const { result } = renderHook(() => useProviderUsageOpenRefresh("default"), { wrapper });
    act(() => result.current());
    expect(globalThis.fetch).not.toHaveBeenCalled();
  });

  it("Should leave consent collection to the daemon and explicitly refresh on retry (UT-063)", async () => {
    const { wrapper } = setup();
    vi.mocked(globalThis.fetch).mockImplementation(() =>
      Promise.resolve(jsonResponse(defaultUsageEnvelope()))
    );
    const allowKeychain = vi.fn().mockResolvedValue(undefined);
    const { result } = renderHook(() => useProviderUsageActions("default", { allowKeychain }), {
      wrapper,
    });

    act(() => result.current.refreshAll());
    await waitFor(() => expect(refreshCalls()).toHaveLength(1));
    expect(await requestBody(refreshCalls()[0] ?? -1)).toEqual({
      profile: "default",
      intent: "explicit",
    });

    act(() => result.current.runAction("claude", "allow_keychain"));
    await waitFor(() => expect(allowKeychain).toHaveBeenCalledWith("claude"));
    await Promise.resolve();
    expect(refreshCalls()).toHaveLength(1);
    act(() => result.current.runAction("claude", "retry"));
    await waitFor(() => expect(refreshCalls()).toHaveLength(2));
    expect(await requestBody(refreshCalls()[1] ?? -1)).toEqual({
      profile: "default",
      intent: "explicit",
      provider_ids: ["claude"],
    });
  });

  it("Should not refresh when saving Keychain consent fails", async () => {
    const { wrapper } = setup();
    const allowKeychain = vi.fn().mockRejectedValue(new Error("apply failed"));
    const { result } = renderHook(() => useProviderUsageActions("default", { allowKeychain }), {
      wrapper,
    });
    act(() => result.current.runAction("claude", "allow_keychain"));
    await waitFor(() => expect(allowKeychain).toHaveBeenCalled());
    await Promise.resolve();
    expect(refreshCalls()).toHaveLength(0);
  });

  it("Should open settings for a configure action without refreshing", async () => {
    const { wrapper } = setup();
    const configure = vi.fn();
    const { result } = renderHook(() => useProviderUsageActions("default", { configure }), {
      wrapper,
    });
    act(() => result.current.runAction("zai", "configure"));
    expect(configure).toHaveBeenCalledWith("zai");
    await Promise.resolve();
    expect(refreshCalls()).toHaveLength(0);
  });

  describe("useProviderUsageStream", () => {
    function fakeSource() {
      const listeners = new Map<string, Set<EventListener>>();
      const source: StreamEventSource & { emit: (type: string) => void; closed: boolean } = {
        closed: false,
        onmessage: null,
        onerror: null,
        onopen: null,
        addEventListener: (type, listener) => {
          const set = listeners.get(type) ?? new Set();
          set.add(listener as EventListener);
          listeners.set(type, set);
        },
        removeEventListener: (type, listener) => {
          listeners.get(type)?.delete(listener as EventListener);
        },
        close: () => {
          source.closed = true;
        },
        emit: type => {
          const data = JSON.stringify({ revision: 15, boot_id: "b_1" });
          for (const listener of listeners.get(type) ?? []) {
            listener(new MessageEvent(type, { data }));
          }
        },
      };
      return source;
    }

    it("Should re-read the envelope on ready and on every change frame", async () => {
      const { queryClient, wrapper } = setup();
      const invalidate = vi.spyOn(queryClient, "invalidateQueries").mockResolvedValue(undefined);
      const source = fakeSource();
      const factory = vi.fn(() => source);
      const { unmount } = renderHook(
        () => useProviderUsageStream("default", { eventSourceFactory: factory }),
        { wrapper }
      );
      expect(factory).toHaveBeenCalledWith("/api/provider-usage/stream?profile=default");

      source.emit("ready");
      source.emit("provider_usage_changed");
      expect(invalidate).toHaveBeenCalledTimes(2);
      expect(invalidate).toHaveBeenCalledWith({
        queryKey: providerUsageKeys.envelope("default"),
        exact: true,
      });

      unmount();
      expect(source.closed).toBe(true);
      source.emit("provider_usage_changed");
      expect(invalidate).toHaveBeenCalledTimes(2);
    });
  });
});
