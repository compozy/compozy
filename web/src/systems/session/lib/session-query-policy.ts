/*
 * Cache policy shared by the session read models. Session detail + transcript
 * are the hot return path for `/agents/:name/sessions/:id`: kept inactive for
 * 30 minutes so tab restores and cross-route returns render from cache
 * immediately. Detail uses bounded live polling; transcript freshness is
 * driven by its SSE tail and explicit recovery reads.
 */

export const SESSION_TRANSCRIPT_STALE_TIME_MS = 10_000;
const SESSION_WARM_CACHE_GC_TIME_MS = 30 * 60 * 1_000;

export const SESSION_WARM_CACHE_POLICY = {
  gcTime: SESSION_WARM_CACHE_GC_TIME_MS,
} as const;

/** True when `queryKey` starts with every element of `scope`, element by element. */
export function queryKeyHasScope(queryKey: readonly unknown[], scope: readonly unknown[]): boolean {
  return (
    scope.length <= queryKey.length && scope.every((element, index) => queryKey[index] === element)
  );
}
