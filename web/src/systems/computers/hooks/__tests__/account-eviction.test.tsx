// Suite: Remote query cache eviction on account sign-out or switch (IT-026 web adapter)
// Invariant: Signing out or switching accounts evicts every ["computer", id, ...] entry where
//            id !== "local", while preserving local ["computer", "local", ...] queries.
// Boundary IN: useAccountRemoteEviction + /api/account and /api/account/stream adapters.
// Boundary OUT: Daemon keyring and cloud device isolation (covered by Go IT-026).

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, renderHook, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import { afterEach, describe, expect, it, vi } from "vitest";

import { computerQueryKey, LOCAL_COMPUTER_ID } from "@/lib/computer-scope";
import * as accountApi from "@/systems/account/adapters/account-api";
import { accountKeys } from "@/systems/account/lib/query-keys";

import { useAccountRemoteEviction } from "../use-account-eviction";

describe("useAccountRemoteEviction (IT-026 web adapter)", () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("IT-026: Should evict all ['computer', 'C1', ...] queries on account switch and sign-out while keeping local cache", async () => {
    let streamHandlers: accountApi.AccountStreamHandlers | null = null;
    let currentAccount: accountApi.AccountStatusPayload = {
      signed_in: true,
      state: "signed_in",
      gate: "off",
      pre_account_install: false,
      email: "a@example.test",
      plan: "free",
    };

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
    };

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
});
