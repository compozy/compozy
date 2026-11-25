import type { QueryClient } from "@tanstack/react-query";

/** Reserved computer identifier for the local daemon (ADR-004). */
export const LOCAL_COMPUTER_ID = "local" as const;

/** Reserved Run on target identifier for load-balanced Auto placement (US-015). */
export const AUTO_COMPUTER_ID = "auto" as const;

export type ComputerId = typeof LOCAL_COMPUTER_ID | (string & {});

export function normalizeComputerId(computerId?: string | null): ComputerId {
  if (typeof computerId !== "string") return LOCAL_COMPUTER_ID;
  const trimmed = computerId.trim();
  return trimmed === "" ? LOCAL_COMPUTER_ID : trimmed;
}

export function isAutoComputerId(computerId?: string | null): boolean {
  return normalizeComputerId(computerId) === AUTO_COMPUTER_ID;
}

export function isRemoteComputerId(computerId?: string | null): boolean {
  const normalized = normalizeComputerId(computerId);
  return normalized !== LOCAL_COMPUTER_ID && normalized !== AUTO_COMPUTER_ID;
}

/**
 * Prepends the mandatory `["computer", computerId]` scope tuple to a TanStack Query key (ADR-004).
 */
export function computerQueryKey<const T extends readonly unknown[]>(
  computerId: string | null | undefined,
  ...segments: T
): readonly ["computer", ComputerId, ...T] {
  return ["computer", normalizeComputerId(computerId), ...segments] as const;
}

/** True when a query key targets a remote computer (`computerId !== "local"`). */
export function isRemoteComputerQueryKey(queryKey: readonly unknown[]): boolean {
  return (
    queryKey.length >= 2 &&
    queryKey[0] === "computer" &&
    typeof queryKey[1] === "string" &&
    queryKey[1] !== "" &&
    queryKey[1] !== LOCAL_COMPUTER_ID
  );
}

/** Extracts the computer ID from a computer-scoped query key, or `null` when unscoped. */
export function queryKeyComputerId(queryKey: readonly unknown[]): ComputerId | null {
  if (queryKey.length >= 2 && queryKey[0] === "computer" && typeof queryKey[1] === "string") {
    return normalizeComputerId(queryKey[1]);
  }
  return null;
}

/** True when a query key targets `["account", "status"]`. */
export function isAccountStatusQueryKey(queryKey: readonly unknown[]): boolean {
  return queryKey.length === 2 && queryKey[0] === "account" && queryKey[1] === "status";
}

const accountScopeEvictionListeners = new Set<() => void>();

/**
 * Registers a store or client cache cleanup callback invoked whenever the signed-in account
 * signs out or switches (Rule 23, AST-010). Returns an unsubscribe function.
 */
export function onAccountScopeEvicted(listener: () => void): () => void {
  accountScopeEvictionListeners.add(listener);
  return () => {
    accountScopeEvictionListeners.delete(listener);
  };
}

function notifyAccountScopeEvicted(): void {
  for (const listener of [...accountScopeEvictionListeners]) {
    listener();
  }
}

export interface EvictRemoteComputerQueriesOptions {
  includeAccount?: boolean;
  includeComputers?: boolean;
  /**
   * When true alongside `includeAccount`, keeps `["account", "status"]` intact while cancelling
   * and removing all other account-scoped queries (`devices`, `approvals`, `tokens`).
   */
  preserveAccountStatus?: boolean;
}

function matchesEvictedScope(
  queryKey: readonly unknown[],
  options: EvictRemoteComputerQueriesOptions
): boolean {
  if (isRemoteComputerQueryKey(queryKey)) return true;
  if (options.includeComputers && queryKey[0] === "computers") return true;
  if (options.includeAccount && queryKey[0] === "account") {
    if (options.preserveAccountStatus && isAccountStatusQueryKey(queryKey)) {
      return false;
    }
    return true;
  }
  return false;
}

function cancelAndRemoveQueries(
  queryClient: QueryClient,
  predicate: (query: { queryKey: readonly unknown[] }) => boolean
): void {
  // Cancel in-flight requests synchronously before removal so a late response from the
  // previous account cannot repopulate the cache after eviction (Rule 23, AST-010).
  void queryClient.cancelQueries({ predicate });
  for (const query of queryClient.getQueryCache().findAll({ predicate })) {
    if (query.getObserversCount() > 0) {
      queryClient.setQueryData(query.queryKey, undefined);
    }
  }
  queryClient.removeQueries({ predicate });
}

/**
 * Cancels and evicts every remote `["computer", id, ...]` query from the cache (Rule 23, US-001.EC-4).
 * Optionally evicts controller-level `["computers", ...]` and `["account", ...]` caches too.
 */
export function evictRemoteComputerQueries(
  queryClient: QueryClient,
  options: EvictRemoteComputerQueriesOptions = {}
): void {
  cancelAndRemoveQueries(queryClient, query => matchesEvictedScope(query.queryKey, options));
}

export interface EvictAccountScopedStateOptions<TStatus = unknown> {
  /**
   * Newly confirmed `/api/account` status to seed into `["account", "status"]` after
   * cancelling and removing all prior account- and computer-scoped queries.
   */
  seedStatus?: TStatus;
  /**
   * Keeps the already-confirmed `["account", "status"]` entry while evicting all other
   * account-scoped (`devices`, `approvals`, `tokens`), computer-registry (`["computers", ...]`),
   * and remote-computer (`["computer", id, ...]`) queries and stores.
   */
  preserveAccountStatus?: boolean;
}

/**
 * Evicts every account-scoped and remote-computer-scoped cache and store in the SPA on
 * sign-out, account deletion, gate block, or account switch (Rule 23, US-001.EC-4, AST-010).
 *
 * Local `["computer", "local", ...]` queries are preserved.
 */
export function evictAccountScopedState<TStatus = unknown>(
  queryClient: QueryClient,
  options: EvictAccountScopedStateOptions<TStatus> = {}
): void {
  const statusKey = ["account", "status"] as const;
  const keepStatus = options.preserveAccountStatus === true;
  const hasSeed = options.seedStatus !== undefined;

  if (keepStatus) {
    cancelAndRemoveQueries(queryClient, query =>
      matchesEvictedScope(query.queryKey, {
        includeAccount: true,
        includeComputers: true,
        preserveAccountStatus: true,
      })
    );
  } else if (hasSeed) {
    cancelAndRemoveQueries(queryClient, query =>
      matchesEvictedScope(query.queryKey, {
        includeAccount: true,
        includeComputers: true,
      })
    );
    queryClient.setQueryData(statusKey, options.seedStatus);
  } else {
    cancelAndRemoveQueries(queryClient, query =>
      matchesEvictedScope(query.queryKey, {
        includeAccount: true,
        includeComputers: true,
        preserveAccountStatus: true,
      })
    );
    const statusQuery = queryClient.getQueryCache().find({ queryKey: statusKey, exact: true });
    if (statusQuery && statusQuery.getObserversCount() > 0) {
      // Wipe previous account's status immediately and trigger a fresh authoritative read.
      void queryClient.resetQueries({ queryKey: statusKey, exact: true });
    } else {
      cancelAndRemoveQueries(queryClient, query => isAccountStatusQueryKey(query.queryKey));
    }
  }

  notifyAccountScopeEvicted();
}
