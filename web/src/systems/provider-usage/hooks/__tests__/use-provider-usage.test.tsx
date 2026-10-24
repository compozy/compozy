// Suite: Provider usage data hooks
// Invariant: One profile-scoped envelope read feeds the view model; popover open sends only a
// background refresh for rows older than 60 s; operator actions are the only explicit
// refreshes; an unreachable daemon keeps the cached view model; every stream frame re-reads.
// Explicit refresh feedback follows unfinished collections through those reads, within its original profile/lifetime.
// Boundary IN: hooks + adapter over a stubbed `fetch` and a fake event source.
// Boundary OUT: rendering (components suite), daemon refresh semantics (Go service suite).
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, renderHook, waitFor } from "@testing-library/react";
import { createElement, type ReactNode } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { toast } from "@compozy/ui";

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
  statusTargets,
  usageEnvelope,
  usageTarget,
} from "../../mocks/fixtures";
import { useProviderUsage } from "../use-provider-usage";
import { useProviderUsageStream } from "../use-provider-usage-stream";
import {
  useProviderUsageActions,
  useProviderUsageOpenRefresh,
  useRefreshProviderUsage,
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
    vi.spyOn(toast, "error").mockReturnValue("usage-error");
    vi.spyOn(toast, "warning").mockReturnValue("usage-warning");
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

  // Invariant: loading is "no envelope yet", not "a fetch is in flight": a read held back until its
  // profile resolves is still waiting, and it settles once the first envelope arrives.
  it("Should be loading until the first envelope arrives, whether or not the read has started", async () => {
    const { wrapper } = setup();
    vi.mocked(globalThis.fetch).mockResolvedValue(jsonResponse(defaultUsageEnvelope()));

    const held = renderHook(() => useProviderUsage(""), { wrapper });
    expect(held.result.current).toMatchObject({ loading: true, viewModel: null });
    expect(globalThis.fetch).not.toHaveBeenCalled();

    const { result } = renderHook(() => useProviderUsage("default"), { wrapper });
    expect(result.current.loading).toBe(true);
    await waitFor(() => expect(result.current.loading).toBe(false));
    expect(result.current.viewModel?.rows).toHaveLength(2);
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

  it("Should refresh the displayed profile and row after switching between non-default profiles", async () => {
    const { wrapper } = setup();
    vi.mocked(globalThis.fetch).mockImplementation(async input => {
      const request = input as Request;
      const profile =
        request.method === "POST"
          ? (await request.clone().json()).profile
          : new URL(request.url).searchParams.get("profile");
      return jsonResponse(
        usageEnvelope(
          [usageTarget({ provider_id: "claude", provider_name: `Claude ${profile}` })],
          { profile_id: profile }
        )
      );
    });
    const { result, rerender } = renderHook(
      ({ profile }) => ({
        usage: useProviderUsage(profile),
        actions: useProviderUsageActions(profile),
      }),
      { wrapper, initialProps: { profile: "work" } }
    );
    for (const profile of ["work", "personal"]) {
      rerender({ profile });
      await waitFor(() => expect(result.current.usage.viewModel?.profileId).toBe(profile));
      expect(result.current.usage.viewModel?.rows[0]?.target.provider_name).toBe(
        `Claude ${profile}`
      );
      act(() => result.current.actions.runAction("claude", "refresh"));
      await waitFor(() => expect(refreshCalls()).toHaveLength(profile === "work" ? 1 : 2));
      expect(await requestBody(refreshCalls().at(-1) ?? -1)).toEqual({
        profile,
        intent: "explicit",
        provider_ids: ["claude"],
      });
      await waitFor(() => expect(result.current.actions.pendingProviderIds.size).toBe(0));
    }
  });

  it("Should keep a pending refresh bound to its original profile after switching", async () => {
    const { queryClient, wrapper } = setup();
    let respond!: (response: Response) => void;
    const pending = new Promise<Response>(resolve => {
      respond = resolve;
    });
    vi.mocked(globalThis.fetch).mockImplementation(input => {
      const request = input as Request;
      if (request.method === "POST") return pending;
      const profile = new URL(request.url).searchParams.get("profile") ?? "";
      return Promise.resolve(jsonResponse(usageEnvelope([], { profile_id: profile })));
    });
    const invalidate = vi.spyOn(queryClient, "invalidateQueries");
    const { result, rerender } = renderHook(
      ({ profile }) => ({
        usage: useProviderUsage(profile),
        actions: useProviderUsageActions(profile),
      }),
      { wrapper, initialProps: { profile: "work" } }
    );
    await waitFor(() => expect(result.current.usage.viewModel?.profileId).toBe("work"));
    act(() => result.current.actions.runAction("claude", "refresh"));
    await waitFor(() => expect(refreshCalls()).toHaveLength(1));
    expect(await requestBody(refreshCalls()[0] ?? -1)).toEqual({
      profile: "work",
      intent: "explicit",
      provider_ids: ["claude"],
    });
    rerender({ profile: "personal" });
    await waitFor(() => expect(result.current.usage.viewModel?.profileId).toBe("personal"));
    await act(async () => {
      respond(jsonResponse(usageEnvelope([statusTargets().needsConsent], { profile_id: "work" })));
      await pending;
    });
    await waitFor(() =>
      expect(invalidate).toHaveBeenCalledWith({
        queryKey: providerUsageKeys.envelope("work"),
        exact: true,
      })
    );
    expect(invalidate).not.toHaveBeenCalledWith({
      queryKey: providerUsageKeys.envelope("personal"),
      exact: true,
    });
    expect(result.current.usage.viewModel?.profileId).toBe("personal");
    expect(result.current.usage.viewModel?.rows).toEqual([]);
    expect(toast.warning).toHaveBeenCalledWith("Claude usage still needs attention (work)", {
      description: "Needs Keychain access",
    });
  });

  it.each(["needs_consent", "signed_out", "error", "stale", "rate_limited"] as const)(
    "Should explain an explicit refresh that returns %s without warning about unrequested rows",
    async status => {
      const { wrapper } = setup();
      const message = "Run Claude Code to refresh your login";
      vi.mocked(globalThis.fetch).mockResolvedValue(
        jsonResponse(
          usageEnvelope([
            usageTarget({
              provider_id: "claude",
              provider_name: "Claude Code",
              status,
              error: { code: "auth_expired", message, action: "sign_in", retry_after: null },
            }),
            statusTargets().codexAuthExpired,
          ])
        )
      );
      const { result } = renderHook(() => useProviderUsageActions("work"), { wrapper });
      act(() => result.current.runAction("claude", "refresh"));
      await waitFor(() =>
        expect(toast.warning).toHaveBeenCalledWith(
          "Claude Code usage still needs attention (work)",
          {
            description: message,
          }
        )
      );
      expect(toast.warning).toHaveBeenCalledTimes(1);
      expect(toast.error).not.toHaveBeenCalled();
    }
  );

  it("Should keep background collection outcomes silent", async () => {
    const { queryClient, wrapper } = setup();
    const envelope = usageEnvelope([statusTargets().keychainTokenExpired]);
    queryClient.setQueryData(providerUsageKeys.envelope("work"), envelope);
    vi.mocked(globalThis.fetch).mockResolvedValue(jsonResponse(envelope));
    const invalidate = vi.spyOn(queryClient, "invalidateQueries").mockResolvedValue(undefined);
    const { result } = renderHook(() => useProviderUsageOpenRefresh("work"), { wrapper });
    act(() => result.current());
    await waitFor(() => expect(invalidate).toHaveBeenCalled());
    expect(toast.warning).not.toHaveBeenCalled();
  });

  it.each([
    ["explicit", "failure"],
    ["explicit", "success"],
    ["background", "failure"],
  ] as const)(
    "Should observe delayed %s refresh %s through the profile cache",
    async (intent, outcome) => {
      const { queryClient, wrapper } = setup();
      const initial = usageEnvelope([{ ...statusTargets().needsConsent, refreshing: true }], {
        profile_id: "work",
        revision: 20,
      });
      const key = providerUsageKeys.envelope("work");
      queryClient.setQueryData(
        key,
        usageEnvelope([statusTargets().needsConsent], { profile_id: "work", revision: 19 })
      );
      vi.mocked(globalThis.fetch).mockResolvedValue(jsonResponse(initial));
      const { result } = renderHook(() => useRefreshProviderUsage("work"), { wrapper });
      act(() => result.current.mutate({ intent, providerIds: ["claude"] }));
      await waitFor(() => expect(result.current.isSuccess).toBe(true));
      expect(toast.warning).not.toHaveBeenCalled();
      const final = usageEnvelope(
        [
          outcome === "failure"
            ? statusTargets().keychainDenied
            : usageTarget({ provider_id: "claude", provider_name: "Claude" }),
          statusTargets().codexAuthExpired,
        ],
        { profile_id: "work", revision: 21 }
      );
      act(() => {
        queryClient.setQueryData(providerUsageKeys.envelope("personal"), final);
        queryClient.setQueryData(key, { ...final, revision: 19 });
      });
      expect(toast.warning).not.toHaveBeenCalled();
      act(() => queryClient.setQueryData(key, final));
      if (intent === "explicit" && outcome === "failure") {
        expect(toast.warning).toHaveBeenCalledWith("Claude usage still needs attention (work)", {
          description: "Keychain access denied",
        });
      }
      const warningCount = intent === "explicit" && outcome === "failure" ? 1 : 0;
      expect(toast.warning).toHaveBeenCalledTimes(warningCount);
      act(() => queryClient.setQueryData(key, { ...final, revision: 22 }));
      expect(toast.warning).toHaveBeenCalledTimes(warningCount);
      expect(globalThis.fetch).toHaveBeenCalledTimes(1);
    }
  );

  it("Should summarize one slow header refresh after every requested collection finishes", async () => {
    const { queryClient, wrapper } = setup();
    const initial = usageEnvelope(
      [
        { ...statusTargets().needsConsent, refreshing: true },
        { ...statusTargets().codexAuthExpired, refreshing: true },
      ],
      { profile_id: "work", revision: 20 }
    );
    vi.mocked(globalThis.fetch).mockResolvedValue(jsonResponse(initial));
    const { result } = renderHook(() => useRefreshProviderUsage("work"), { wrapper });
    act(() => result.current.mutate({ intent: "explicit" }));
    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    act(() =>
      queryClient.setQueryData(providerUsageKeys.envelope("work"), {
        ...initial,
        revision: 21,
        targets: [statusTargets().keychainDenied, initial.targets[1]],
      })
    );
    expect(toast.warning).not.toHaveBeenCalled();
    act(() =>
      queryClient.setQueryData(providerUsageKeys.envelope("work"), {
        ...initial,
        revision: 22,
        targets: [statusTargets().keychainDenied, statusTargets().codexAuthExpired],
      })
    );
    expect(toast.warning).toHaveBeenCalledTimes(1);
    expect(toast.warning).toHaveBeenCalledWith("2 providers still need attention (work)", {
      description: "Claude: Keychain access denied · Codex: Run Codex to refresh your login",
    });
  });

  it.each([
    ["switch", "before"],
    ["switch", "after"],
    ["unmount", "before"],
    ["unmount", "after"],
  ] as const)(
    "Should stop delayed feedback on %s when the HTTP response arrives %s cleanup",
    async (cleanup, timing) => {
      const { queryClient, wrapper } = setup();
      const initial = usageEnvelope([{ ...statusTargets().needsConsent, refreshing: true }], {
        profile_id: "work",
        revision: 20,
      });
      let respond!: (response: Response) => void;
      const pending = new Promise<Response>(resolve => {
        respond = resolve;
      });
      vi.mocked(globalThis.fetch).mockReturnValue(pending);
      const invalidate = vi.spyOn(queryClient, "invalidateQueries").mockResolvedValue(undefined);
      const { result, rerender, unmount } = renderHook(
        ({ profile }) => useRefreshProviderUsage(profile),
        { wrapper, initialProps: { profile: "work" } }
      );
      act(() => result.current.mutate({ intent: "explicit", providerIds: ["claude"] }));
      await waitFor(() => expect(refreshCalls()).toHaveLength(1));
      if (timing === "before") {
        act(() => respond(jsonResponse(initial)));
        await waitFor(() => expect(result.current.isSuccess).toBe(true));
      }
      if (cleanup === "switch") {
        rerender({ profile: "personal" });
        rerender({ profile: "work" });
      } else unmount();
      if (timing === "after") act(() => respond(jsonResponse(initial)));
      await waitFor(() => expect(invalidate).toHaveBeenCalled());
      act(() =>
        queryClient.setQueryData(
          providerUsageKeys.envelope("work"),
          usageEnvelope([statusTargets().keychainDenied], { profile_id: "work", revision: 21 })
        )
      );
      expect(toast.warning).not.toHaveBeenCalled();
    }
  );

  it("Should summarize header refresh failures in one bounded warning", async () => {
    const { wrapper } = setup();
    const targets = Array.from({ length: 8 }, (_, index) => ({
      ...statusTargets().needsConsent,
      provider_id: `provider-${index}`,
      provider_name: `Provider ${index}`,
    }));
    vi.mocked(globalThis.fetch).mockResolvedValue(jsonResponse(usageEnvelope(targets)));
    const { result } = renderHook(() => useProviderUsageActions("work"), { wrapper });
    act(() => result.current.refreshAll());
    await waitFor(() =>
      expect(toast.warning).toHaveBeenCalledWith("8 providers still need attention (work)", {
        description:
          "Provider 0: Needs Keychain access · Provider 1: Needs Keychain access · Provider 2: Needs Keychain access · 5 more. Open Usage for details.",
      })
    );
    expect(toast.warning).toHaveBeenCalledTimes(1);
  });

  it.each(["all", "retry", "refresh"] as const)(
    "Should report a failed explicit %s even when the cache re-read succeeds",
    async action => {
      const { queryClient, wrapper } = setup();
      const envelope = defaultUsageEnvelope();
      vi.mocked(globalThis.fetch).mockImplementation(input => {
        const failed = (input as Request).method === "POST";
        return Promise.resolve(
          jsonResponse(
            failed ? { error: "Refresh refused", code: "unavailable" } : envelope,
            failed ? 503 : 200
          )
        );
      });
      const { result } = renderHook(
        () => ({
          usage: useProviderUsage("default"),
          actions: useProviderUsageActions("default"),
        }),
        { wrapper }
      );
      await waitFor(() => expect(result.current.usage.viewModel?.rows).toHaveLength(2));
      act(() => {
        if (action === "all") result.current.actions.refreshAll();
        else result.current.actions.runAction("codex", action);
      });
      await waitFor(() =>
        expect(toast.error).toHaveBeenCalledWith("Couldn't refresh provider usage (default)", {
          description: "Refresh refused",
        })
      );
      await waitFor(() => expect(result.current.actions.refreshing).toBe(false));
      expect(toast.error).toHaveBeenCalledTimes(1);
      await waitFor(() => {
        const reads = vi
          .mocked(globalThis.fetch)
          .mock.calls.filter(([input]) => (input as Request).method === "GET");
        expect(reads.length).toBeGreaterThanOrEqual(2);
        expect(queryClient.isFetching()).toBe(0);
      });
      expect(result.current.usage.viewModel?.rows).toHaveLength(2);
      expect(result.current.usage.errorMessage).toBeNull();
    }
  );

  it("Should keep background refresh failures silent", async () => {
    const { queryClient, wrapper } = setup();
    queryClient.setQueryData(providerUsageKeys.envelope("default"), defaultUsageEnvelope());
    vi.mocked(globalThis.fetch).mockRejectedValue(new TypeError("Failed to fetch"));
    const invalidate = vi.spyOn(queryClient, "invalidateQueries").mockResolvedValue(undefined);
    const { result } = renderHook(() => useProviderUsageOpenRefresh("default"), { wrapper });
    act(() => result.current());
    await waitFor(() => expect(invalidate).toHaveBeenCalled());
    expect(toast.error).not.toHaveBeenCalled();
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
