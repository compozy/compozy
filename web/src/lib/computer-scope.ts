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

/**
 * Evicts every remote `["computer", id, ...]` query from the cache (Rule 23, US-001.EC-4).
 * Optionally evicts controller-level `["computers", ...]` and `["account", ...]` caches too.
 */
export function evictRemoteComputerQueries(
  queryClient: QueryClient,
  options: { includeAccount?: boolean; includeComputers?: boolean } = {}
): void {
  queryClient.removeQueries({
    predicate: query => isRemoteComputerQueryKey(query.queryKey),
  });
  if (options.includeComputers) {
    queryClient.removeQueries({
      predicate: query => query.queryKey[0] === "computers",
    });
  }
  if (options.includeAccount) {
    queryClient.removeQueries({
      predicate: query => query.queryKey[0] === "account",
    });
  }
}
