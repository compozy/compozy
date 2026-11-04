import { queryOptions } from "@tanstack/react-query";

import type { ProfileScopeParams } from "@/systems/profiles";

import { sessionKeys } from "./query-keys";

export type SessionCatalogStreamStatus = "disabled" | "connecting" | "live" | "stale";

/** The catalog stream for one profile scope; it always spans every workspace. */
export function sessionCatalogStreamURL(scope: ProfileScopeParams): string {
  const params = new URLSearchParams({ all_workspaces: "true" });
  if ("all_profiles" in scope) params.set("all_profiles", "true");
  else params.set("profile", scope.profile);
  return `/api/sessions/catalog-stream?${params.toString()}`;
}

/**
 * The status the catalog stream last published for `streamUrl`. The stream writes this
 * entry; nothing fetches it. A missing, reset or never-written entry reads as
 * `disabled`, so a poll it covers falls back to its interval rather than going silent.
 */
export function sessionCatalogStreamStatusOptions(streamUrl: string) {
  return queryOptions({
    queryKey: sessionKeys.catalogStreamStatus(streamUrl),
    queryFn: (): SessionCatalogStreamStatus => "disabled",
    enabled: false,
    staleTime: Infinity,
    gcTime: Infinity,
  });
}

/** A failed covered read keeps a bounded retry: no stream wake may arrive to repair it. */
export const CATALOG_STREAM_COVERED_ERROR_REFETCH_INTERVAL_MS = 30_000;

/**
 * Cadence of a poll the catalog stream covers: none while that stream is live for the
 * read's scope and the last read succeeded (its wakes re-read the poll's source), a
 * bounded retry after a failure, and the fallback interval otherwise. Only reads whose
 * every change the catalog stream wakes may use it; task, loop, terminal-input and
 * cross-profile notification reads keep their own polls.
 */
export function catalogStreamCoveredRefetchInterval(
  streamCovered: boolean,
  failed: boolean,
  fallbackMs: number
): number | false {
  if (!streamCovered) return fallbackMs;
  return failed ? CATALOG_STREAM_COVERED_ERROR_REFETCH_INTERVAL_MS : false;
}
