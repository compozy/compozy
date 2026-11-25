// Suite: Remote query cache eviction on account sign-out or switch (IT-026 web adapter)
// Invariant: Signing out or switching accounts evicts every ["computer", id, ...] entry where
//            id !== "local", while preserving local ["computer", "local", ...] queries.
// Boundary IN: useAccountRemoteEviction + /api/account and /api/account/stream adapters.
// Boundary OUT: Daemon keyring and cloud device isolation (covered by Go IT-026).

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, renderHook, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import { afterEach, describe, expect, it, vi } from "vitest";

import { apiClientFor } from "@/lib/api-client";
import { computerQueryKey, LOCAL_COMPUTER_ID } from "@/lib/computer-scope";
import * as accountApi from "@/systems/account/adapters/account-api";
import { useAccountSettingsPage } from "@/systems/account/hooks/use-account-settings-page";
import { accountKeys } from "@/systems/account/lib/query-keys";
import { createSessionCreateStore, SessionCreateProvider } from "@/systems/session";
import {
  isSessionStreamLive,
  retainSessionStreamLiveness,
} from "@/systems/session/lib/session-stream-liveness";

import { computerRegistryKeys } from "../../lib/query-keys";
import { useAccountRemoteEviction } from "../use-account-eviction";

vi.mock("@tanstack/react-router", async importOriginal => {
  const actual = await importOriginal<typeof import("@tanstack/react-router")>();
  return {
    ...actual,
    useRouter: () => ({
      navigate: vi.fn(),
      state: { location: { pathname: "/settings/account", href: "/settings/account" } },
    }),
  };
});

function makeAccountStatus(
  overrides: Partial<accountApi.AccountStatusPayload> = {}
): accountApi.AccountStatusPayload {
  return {
    gate: "off",
    grace: null,
    state: "signed_in",
    source: "session",
    signed_out_reason: null,
    pre_account_install: false,
    installation_linked: false,
    token_name: null,
    signed_in: true,
    email: "a@example.test",
    name: "Account A",
    provider: "github",
    device: { id: "dev_a", name: "MacBook A", admin: true },
    marketing_opt_in: false,
    plan: "pro",
    subscription: null,
    remote_computers_used: 1,
    remote_computers_limit: 10,
    cloud_routes_used: 0,
    cloud_routes_limit: 10,
    payment_issue: false,
    recent_sign_ins: [],
    ...overrides,
  };
}

describe("useAccountRemoteEviction (IT-026 web adapter)", () => {
  afterEach(() => {
    vi.unstubAllGlobals();
    vi.restoreAllMocks();
  });

  it("IT-026: Should evict all ['computer', 'C1', ...] queries on account switch and sign-out while keeping local cache", async () => {
    let streamHandlers: accountApi.AccountStreamHandlers | null = null;
    let currentAccount = {
      signed_in: true,
      state: "signed_in",
      gate: "off",
      pre_account_install: false,
      email: "a@example.test",
      plan: "free",
    } as unknown as accountApi.AccountStatusPayload;

    vi.spyOn(accountApi, "fetchAccountStatus").mockImplementation(async () => currentAccount);
    vi.spyOn(accountApi, "openAccountStream").mockImplementation(handlers => {
      streamHandlers = handlers;
      return () => {
        streamHandlers = null;
      };
    });

    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    });
    queryClient.setQueryData(computerQueryKey(LOCAL_COMPUTER_ID, "sessions"), ["local-sess"]);
    queryClient.setQueryData(computerQueryKey("C1", "sessions"), ["remote-c1-sess"]);
    queryClient.setQueryData(computerQueryKey("C1", "workspaces"), ["remote-c1-ws"]);

    const wrapper = ({ children }: { children: ReactNode }) => (
      <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
    );

    renderHook(() => useAccountRemoteEviction(), { wrapper });

    await waitFor(() => {
      expect(queryClient.getQueryData(accountKeys.status())).toBeDefined();
      expect(streamHandlers).not.toBeNull();
    });
    expect(queryClient.getQueryData(computerQueryKey("C1", "sessions"))).toEqual([
      "remote-c1-sess",
    ]);

    // Simulate daemon account stream delivering sign-out / account switch with evict_remote: true.
    currentAccount = {
      signed_in: true,
      state: "signed_in",
      gate: "off",
      pre_account_install: false,
      email: "b@example.test",
      plan: "free",
    } as unknown as accountApi.AccountStatusPayload;

    act(() => {
      streamHandlers?.onAccountChanged({ evict_remote: true });
    });

    await waitFor(() => {
      expect(queryClient.getQueryData(computerQueryKey("C1", "sessions"))).toBeUndefined();
      expect(queryClient.getQueryData(computerQueryKey("C1", "workspaces"))).toBeUndefined();
    });
    expect(
      queryClient
        .getQueryCache()
        .findAll()
        .some(query => query.queryKey[0] === "computer" && query.queryKey[1] === "C1")
    ).toBe(false);
    expect(queryClient.getQueryData(computerQueryKey(LOCAL_COMPUTER_ID, "sessions"))).toEqual([
      "local-sess",
    ]);
  });

  it("Should not open /api/account/stream while signed out", async () => {
    vi.spyOn(accountApi, "fetchAccountStatus").mockResolvedValue({
      signed_in: false,
      state: "signed_out",
      gate: "off",
      pre_account_install: false,
      plan: "free",
    } as unknown as accountApi.AccountStatusPayload);
    const openStreamSpy = vi.spyOn(accountApi, "openAccountStream").mockReturnValue(() => {});

    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    });
    const wrapper = ({ children }: { children: ReactNode }) => (
      <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
    );

    const { result } = renderHook(() => useAccountRemoteEviction(), { wrapper });

    await waitFor(() => {
      expect(queryClient.getQueryData(accountKeys.status())).toBeDefined();
    });
    expect(result.current.signedIn).toBe(false);
    expect(openStreamSpy).not.toHaveBeenCalled();
  });

  // Regression test for AST-010: switching accounts via SSE must cancel in-flight account queries,
  // remove every account- and computer-scoped cache entry, and clear remote SPA stores while
  // keeping local computer caches intact.
  it("AST-010: Should cancel and remove account/computer queries and clear remote SPA stores on account switch", async () => {
    let streamHandlers: accountApi.AccountStreamHandlers | null = null;
    let currentAccount = makeAccountStatus({
      email: "a@example.test",
      name: "Account A",
      device: { id: "dev_a", name: "MacBook A", admin: true },
    });

    vi.spyOn(accountApi, "fetchAccountStatus").mockImplementation(async () => currentAccount);
    vi.spyOn(accountApi, "openAccountStream").mockImplementation(handlers => {
      streamHandlers = handlers;
      return () => {
        streamHandlers = null;
      };
    });

    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    });
    queryClient.setQueryData(computerQueryKey(LOCAL_COMPUTER_ID, "sessions"), ["local-sess"]);
    queryClient.setQueryData(computerQueryKey("C1", "sessions"), ["remote-c1-sess"]);
    queryClient.setQueryData(computerRegistryKeys.list(), { computers: [{ id: "C1" }] });
    queryClient.setQueryData(computerRegistryKeys.available(), { computers: [{ id: "C2" }] });
    queryClient.setQueryData(accountKeys.devices(), {
      devices: [{ id: "dev_a", name: "Account A Mac" }],
    });
    queryClient.setQueryData(accountKeys.approvals(), {
      approvals: [{ id: "apr_a", device_name: "Account A Phone" }],
    });
    queryClient.setQueryData(accountKeys.tokens(), {
      tokens: [{ id: "atk_a", name: "Account A CI" }],
    });

    // Populate SPA stores and remote API client cache for Account A.
    const clientBefore = apiClientFor("C1");
    const releaseRemoteStream = retainSessionStreamLiveness("sess-remote", "C1");
    const releaseLocalStream = retainSessionStreamLiveness("sess-local", LOCAL_COMPUTER_ID);
    const sessionCreateStore = createSessionCreateStore();
    sessionCreateStore.trigger.computerSelected({ computerId: "C1" });

    // Start an in-flight account devices fetch for Account A so eviction must abort it.
    let inFlightSignal: AbortSignal | undefined;
    let resolveInFlight: ((value: unknown) => void) | null = null;
    const inFlightFetch = queryClient
      .fetchQuery({
        queryKey: accountKeys.devices(),
        queryFn: ({ signal }) => {
          inFlightSignal = signal;
          return new Promise(resolve => {
            resolveInFlight = resolve;
          });
        },
      })
      .catch(() => undefined);

    const wrapper = ({ children }: { children: ReactNode }) => (
      <QueryClientProvider client={queryClient}>
        <SessionCreateProvider store={sessionCreateStore}>{children}</SessionCreateProvider>
      </QueryClientProvider>
    );

    try {
      renderHook(() => useAccountRemoteEviction(), { wrapper });

      await waitFor(() => {
        expect(queryClient.getQueryData(accountKeys.status())).toEqual(currentAccount);
        expect(streamHandlers).not.toBeNull();
      });

      currentAccount = makeAccountStatus({
        email: "b@example.test",
        name: "Account B",
        device: { id: "dev_b", name: "MacBook B", admin: true },
      });

      act(() => {
        streamHandlers?.onAccountChanged({ evict_remote: true });
      });

      // Even if the cancelled in-flight request resolves late, A's devices must not land in cache.
      resolveInFlight?.({ devices: [{ id: "dev_a_late", name: "Late Account A Device" }] });
      await inFlightFetch;

      await waitFor(() => {
        expect(queryClient.getQueryData(accountKeys.status())).toEqual(currentAccount);
      });

      expect(inFlightSignal?.aborted).toBe(true);
      expect(queryClient.getQueryData(accountKeys.devices())).toBeUndefined();
      expect(queryClient.getQueryData(accountKeys.approvals())).toBeUndefined();
      expect(queryClient.getQueryData(accountKeys.tokens())).toBeUndefined();
      expect(queryClient.getQueryData(computerRegistryKeys.list())).toBeUndefined();
      expect(queryClient.getQueryData(computerRegistryKeys.available())).toBeUndefined();
      expect(queryClient.getQueryData(computerQueryKey("C1", "sessions"))).toBeUndefined();
      expect(
        queryClient
          .getQueryCache()
          .findAll({ queryKey: accountKeys.all })
          .map(query => query.queryKey)
      ).toEqual([accountKeys.status()]);

      // Remote SPA stores and API client cache are cleared; local state survives.
      expect(apiClientFor("C1")).not.toBe(clientBefore);
      expect(isSessionStreamLive("sess-remote", "C1")).toBe(false);
      expect(isSessionStreamLive("sess-local", LOCAL_COMPUTER_ID)).toBe(true);
      expect(sessionCreateStore.getSnapshot().context.draft.computerId).toBe(LOCAL_COMPUTER_ID);
      expect(queryClient.getQueryData(computerQueryKey(LOCAL_COMPUTER_ID, "sessions"))).toEqual([
        "local-sess",
      ]);
    } finally {
      releaseRemoteStream();
      releaseLocalStream();
    }
  });

  // Regression test for AST-010: signing out as Account A in Account settings and returning as
  // Account B before GC must never expose A's cached devices, approvals, tokens, or profile data.
  it("AST-010: Should evict Account A metadata on sign-out so Account B never sees A's rows when fetches fail", async () => {
    const accountAStatus = makeAccountStatus({
      email: "a@example.test",
      name: "Account A",
      device: { id: "dev_a", name: "Account A Laptop", admin: true },
      recent_sign_ins: [
        {
          device_id: "dev_a2",
          name: "Account A Phone",
          kind: "browser",
          location: "Lisbon",
          at: "2026-11-24T10:00:00Z",
        },
      ],
    });
    const accountBStatus = makeAccountStatus({
      email: "b@example.test",
      name: "Account B",
      device: { id: "dev_b", name: "Account B Laptop", admin: true },
      recent_sign_ins: [],
    });

    let activeStatus = accountAStatus;
    let failAccountReads = false;

    vi.spyOn(accountApi, "fetchAccountStatus").mockImplementation(async () => activeStatus);
    vi.spyOn(accountApi, "fetchAccountDevices").mockImplementation(async () => {
      if (failAccountReads) throw new Error("cloud temporarily unavailable");
      return {
        devices: [
          {
            id: "dev_a",
            name: "Account A Laptop",
            kind: "desktop",
            admin: true,
            this_device: true,
            last_seen_at: "2026-11-24T10:00:00Z",
          },
        ],
      };
    });
    vi.spyOn(accountApi, "fetchAccountApprovals").mockImplementation(async () => {
      if (failAccountReads) throw new Error("cloud temporarily unavailable");
      return {
        approvals: [
          {
            id: "apr_a",
            computer_id: "C1",
            device_name: "Account A Phone",
            key_fingerprint: "fp_a",
            state: "pending",
            applied_result: null,
            can_decide: true,
            created_at: "2026-11-24T10:00:00Z",
          },
        ],
      };
    });
    vi.spyOn(accountApi, "fetchAccountTokens").mockImplementation(async () => {
      if (failAccountReads) throw new Error("cloud temporarily unavailable");
      return {
        tokens: [
          {
            id: "atk_a",
            name: "Account A CI Token",
            last_used_at: null,
          },
        ],
      };
    });
    vi.spyOn(accountApi, "logoutAccount").mockResolvedValue({ ok: true } as never);
    vi.spyOn(accountApi, "openAccountStream").mockReturnValue(() => undefined);
    vi.stubGlobal(
      "fetch",
      vi.fn(async () => Response.json({ computers: [], enabled: false }))
    );

    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false, gcTime: 60_000 } },
    });
    const wrapper = ({ children }: { children: ReactNode }) => (
      <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
    );

    // 1. Open Account settings as Account A and confirm A's metadata is loaded.
    const viewA = renderHook(() => useAccountSettingsPage(), { wrapper });
    await waitFor(() => {
      expect(viewA.result.current.devices).toHaveLength(1);
      expect(viewA.result.current.approvals).toHaveLength(1);
      expect(viewA.result.current.tokens).toHaveLength(1);
    });

    // 2. Sign out as Account A.
    act(() => {
      viewA.result.current.handleConfirmSignOut();
    });

    await waitFor(() => {
      expect(viewA.result.current.account?.signedIn).toBe(false);
    });

    // Account A's email/name/device/notices and sub-resource caches must be completely scrubbed.
    const signedOutStatus = queryClient.getQueryData<accountApi.AccountStatusPayload>(
      accountKeys.status()
    );
    expect(signedOutStatus?.email).toBe("");
    expect(signedOutStatus?.device).toBeNull();
    expect(signedOutStatus?.recent_sign_ins).toEqual([]);
    expect(queryClient.getQueryData(accountKeys.devices())).toBeUndefined();
    expect(queryClient.getQueryData(accountKeys.approvals())).toBeUndefined();
    expect(queryClient.getQueryData(accountKeys.tokens())).toBeUndefined();

    viewA.unmount();

    // 3. Sign in as Account B and reopen Account settings while B's metadata requests fail.
    activeStatus = accountBStatus;
    failAccountReads = true;
    queryClient.setQueryData(accountKeys.status(), accountBStatus);

    const viewB = renderHook(() => useAccountSettingsPage(), { wrapper });
    await waitFor(() => {
      expect(viewB.result.current.account?.email).toBe("b@example.test");
    });

    expect(viewB.result.current.devices).toEqual([]);
    expect(viewB.result.current.approvals).toEqual([]);
    expect(viewB.result.current.tokens).toEqual([]);
  });

  // Regression test for AST-010: when account status changes directly from Account A to Account B
  // without an SSE event, the identity-change effect must remove A's account sub-queries and keep
  // only the newly confirmed status for Account B.
  it("AST-010: Should evict previous account sub-queries when account identity changes directly", async () => {
    const accountA = makeAccountStatus({ email: "a@example.test" });
    const accountB = makeAccountStatus({ email: "b@example.test" });

    vi.spyOn(accountApi, "fetchAccountStatus").mockResolvedValue(accountA);
    vi.spyOn(accountApi, "openAccountStream").mockReturnValue(() => undefined);

    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    });
    queryClient.setQueryData(accountKeys.devices(), { devices: [{ id: "dev_a" }] });
    queryClient.setQueryData(accountKeys.approvals(), { approvals: [{ id: "apr_a" }] });
    queryClient.setQueryData(accountKeys.tokens(), { tokens: [{ id: "atk_a" }] });
    queryClient.setQueryData(computerRegistryKeys.list(), { computers: [{ id: "C1" }] });
    queryClient.setQueryData(computerQueryKey("C1", "sessions"), ["remote-c1"]);
    queryClient.setQueryData(computerQueryKey(LOCAL_COMPUTER_ID, "sessions"), ["local-sess"]);

    const wrapper = ({ children }: { children: ReactNode }) => (
      <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
    );

    renderHook(() => useAccountRemoteEviction(), { wrapper });

    await waitFor(() => {
      expect(queryClient.getQueryData(accountKeys.status())).toEqual(accountA);
    });

    act(() => {
      queryClient.setQueryData(accountKeys.status(), accountB);
    });

    await waitFor(() => {
      expect(queryClient.getQueryData(accountKeys.devices())).toBeUndefined();
      expect(queryClient.getQueryData(accountKeys.approvals())).toBeUndefined();
      expect(queryClient.getQueryData(accountKeys.tokens())).toBeUndefined();
      expect(queryClient.getQueryData(computerRegistryKeys.list())).toBeUndefined();
      expect(queryClient.getQueryData(computerQueryKey("C1", "sessions"))).toBeUndefined();
    });

    expect(queryClient.getQueryData(accountKeys.status())).toEqual(accountB);
    expect(queryClient.getQueryData(computerQueryKey(LOCAL_COMPUTER_ID, "sessions"))).toEqual([
      "local-sess",
    ]);
  });
});
