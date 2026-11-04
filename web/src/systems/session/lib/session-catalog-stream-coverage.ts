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

/**
 * Safety cadence of a covered read while the catalog stream is live, matching the session
 * stream's: the stream's wakes re-read the source as changes happen, and this bounded
 * read repairs a failed read or a change whose daemon publish was lost.
 */
export const CATALOG_STREAM_SAFETY_REFETCH_INTERVAL_MS = 30_000;

/**
 * Cadence of a poll the catalog stream covers: the safety cadence while that stream is
 * live for the read's scope, the read's own fallback interval otherwise. Only reads whose
 * changes the catalog stream wakes may use it; task, loop, terminal-input and
 * cross-profile notification reads keep their own polls.
 */
export function catalogStreamCoveredRefetchInterval(
  streamCovered: boolean,
  fallbackMs: number
): number {
  return streamCovered
    ? Math.max(fallbackMs, CATALOG_STREAM_SAFETY_REFETCH_INTERVAL_MS)
    : fallbackMs;
}
