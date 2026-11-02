# Independent review: package cache and Loop refresh

Reviewed the uncommitted performance diff against `de5ab2f4d` on 2026-11-02. This review changed no production or canonical test source. Only this report and temporary diagnostics under `.cache/performance/review-cache-loops/` were written. No daemon, browser, production state, full suite or gate was started.

## Result

**No outstanding actionable defect found in the reviewed diff.** The concrete pre-existing initial-fetch freshness limitation L1 was repaired by the owning worker after this review, and its disposition is independently verified below. The original reproduction and baseline control are retained as historical evidence.

## L1: initial-fetch invalidation loss — resolved and independently verified

Original pre-fix location: `web/src/systems/loops/lib/loop-live-refresh-coordinator.ts:89`, `:96`, `:100`. The resolved implementation is `refreshLoopQuery` in the same file, starting at line 55.

Before the repair, the coordinator treated completion of `invalidateQueries` as completion of an authoritative read initiated after the accepted event. TanStack Query 5.104.5 instead reuses an already-running fetch when the query has no data, including when `cancelRefetch` is true. Its older response then clears `isInvalidated`. With no subsequent frame, `pending` was empty and the coordinator returned to idle without a read that included the event.

Minimal reproduction with actual QueryClient, QueryObserver, XState store, coordinator source and canonical key-factory source:

1. Subscribe to a query with no cached data. Its query function captures server revision 0 and waits on an explicit promise barrier.
2. Advance the synthetic server to revision 1 and request a `run` refresh.
3. Let the coordinator microtask run, then resolve the original revision-0 read.
4. Observe one read, cached revision 0, `isInvalidated: false`, coordinator phase `idle`.

| Case | Reads after settlement | Cached revision | Invalidated | Result |
| --- | --- | --- | --- | --- |
| New coordinator, initial read with no data | 1 | 0 | false | No trailing read for revision 1 |
| Previous direct `invalidateQueries` control | 1 | 0 | false | Same pre-existing limitation |
| New coordinator, existing read with cached data | 2 | 1 | false | Converges correctly |
| Disabled observer with an initial prefetch, then immediate reenable | 1 | 0 | false | No immediate reread after reenable |

The diagnostic uses `staleTime: Infinity` to isolate invalidation from unrelated freshness triggers. Product Loop queries use a five-second stale time and some live reads poll, so polling, later frames or another stale-query trigger can recover. Terminal reads disable polling; no blanket immediate-convergence guarantee is warranted. The disabled-observer case also has an explicit in-flight prefetch; a disabled query without a pending fetch remains invalidated normally.

This was **not classified as a regression introduced by the optimization**, because the baseline control reproduced it. Root elected to address it in the current scope. The invariant is: an accepted event must not be considered reconciled solely by a fetch started before that event. The canonical suite is `web/src/systems/loops/hooks/__tests__/use-loop-stream.test.tsx`; the owner added initial-read cases there rather than creating another suite.

Command, executed through the required serialized runner:

```sh
python3 .cache/performance/run-bounded.py bun .cache/performance/review-cache-loops/repro.ts
```

Successful raw receipt: `.cache/performance/review-cache-loops/repro-results.txt` (Bun 1.4.0, four cases, exit 0). The coordinator and key factories are read and transpiled directly without changing them; only their imports are supplied explicitly to avoid unrelated Vite-only assets. QueryClient/QueryObserver and XState are the installed real libraries. Earlier receipts preserve module-resolution/Vite `import.meta.glob` harness failures; those did not execute the cases and are not validation evidence.

### Resolution and independent recheck

The owner's `refreshLoopQuery` now captures already-fetching queries for each affected prefix, awaits their promises with `Promise.allSettled`, and then invalidates. Refetch stays active-only. This covers rejected initial reads and preserves disabled/inactive queries as invalidated without forcing network work. Pending accepted work still survives stream disposal; closed coordinators still reject later refresh requests.

The independent recheck executed the current coordinator source with the real libraries and explicit promise barriers. **All eight conditions passed**:

| Condition | Observed behavior after repair |
| --- | --- |
| Active observer, initial fetch without data | Two reads; current revision 1; invalidation cleared only after the trailing read |
| Disabled observer with in-flight prefetch | One read while disabled, `isInvalidated: true`; enabling causes the second read and revision 1 |
| Disabled observer without an in-flight read | Zero reads while disabled, remains invalidated; enabling performs one current read |
| Disposal before the scheduled microtask | Accepted event still reconciles through a second read |
| Disposal while waiting for the initial read | Accepted event still reconciles through a second read |
| Cleanup removes the last observer | No forced background read; the completed old response remains invalidated, and resubscription reads revision 1 |
| Initial read rejects | Failure does not erase the obligation; the trailing read returns revision 1 |
| A1→B→A2 and late refresh from closed A1 | Current A data converges to revision 2; the old A1 refresh starts no additional read and changes no data |

The original direct-invalidation control still returns revision 0, proving the negative control remains effective. The owner separately proved four negative cases before the fix and **28/28 canonical hook tests** after it, with the existing burst still at three reads and zero cancellations.

Independent command and successful receipt:

```sh
python3 .cache/performance/run-bounded.py bash -c 'bun .cache/performance/review-cache-loops/repro.ts && bun .cache/performance/review-cache-loops/recheck.ts'
```

Receipt: `.cache/performance/review-cache-loops/recheck-results.txt`, Bun 1.4.0, exit 0; it records the reviewed source SHA-256. Owner receipts: `.cache/performance/loop-loops/stream-initial-negative.txt` and `stream-initial-after.txt`. The review itself made no production or canonical test edits.

## Cache and filesnap review

Reviewed `internal/marketplace/pluginsource/cache.go`, `cache_check.go`, `cache_sweep.go`, `cache_test.go`; `internal/filesnap/filesnap.go`, `filesnap_test.go` and Darwin/Linux/other change-token helpers; and the relevant `internal/fileutil/nofollow*` implementations and `marketplace/service_cache.go` consumer.

No new corruption, race, symlink, digest-identity or eviction defect was found:

- Every `Check` opens the cache root and final regular file through the existing anchored, non-following directory API before considering a cache hit. Missing files, changed permissions and replacement symlinks are not bypassed by the snapshot map. Unix uses `openat` with `O_NOFOLLOW`; Windows rejects reparse points through the existing native handle path.
- A remembered verification is keyed by the **expected SHA-256 digest**, not only inode metadata. A hardlink containing a different digest cannot borrow another digest's verification.
- `FromFile` samples the held descriptor without consuming the reader. Darwin/Linux comparison includes device, inode and ctime in addition to size, mode and mtime; restoring size/mtime does not alone authorize reuse. The helpers are unchanged from baseline.
- Successful hashing populates the map only if before/after descriptor snapshots match. Other platforms or unavailable native metadata leave held-file snapshots uncacheable, so checks continue to hash. Existing `FromPath`/`FromInfo` fingerprint fallback behavior is preserved.
- Cache read locking spans check/open and coordinates against publication/sweep. Snapshot map accesses use `verifiedMu`; sweep obtains the cache write lock before clearing the map. No reversed lock order was found. Concurrent readers can duplicate a cold hash, but cannot publish unverified bytes through this path.
- The map holds at most 1,024 metadata entries and no archive contents or open handles. Sweep clears it under the cache lock. External removal still fails the next anchored open. Installation `Open` independently hashes bytes on every invocation; it never consults the availability snapshot cache.

The existing canonical tests cover same-size/restored-mtime corruption, repaired blobs, digest mismatch through hardlinks, cancellation, deletion/replacement, symlinks, permission errors, concurrent corrupt/symlink replacement, concurrent puts/checks, pinned eviction and held-reader identity/offset. Reviewed owner receipts show `TestPackageCache` passing under race (2.281 s), filesnap selected suites passing under race (1.941 s), and acquisition integration passing under race (1.457 s). Those are owner-run receipts in `.cache/performance/extension-marketplace/focused-race.txt` and `integration-race.txt`; this read-only review did not repeat them.

Limits: native runtime evidence is Darwin/arm64. Linux and Windows helpers were statically inspected, not executed here. This review does not establish integrity guarantees for a privileged adversary or an unusual filesystem that can preserve all supposedly authoritative native identity/change metadata while changing bytes. As before, availability is a point-in-time observation, and installation remains the fresh-hash boundary.

## Loop cleanup and scope review

Reviewed the new coordinator, `use-loop-stream.ts`, `loop-stream-lifecycle-store.ts`, query keys/options, and the existing hook tests.

- The pending set contains at most four scope labels. Each round drains it; events received during a round request one follow-up. Request/catalog extras are retained in the union. When a run refresh includes goal-turn events, the `runReads` prefix covers the goal-turn descendant key, so suppressing a redundant exact key does not omit it.
- Cleanup closes event admission while retaining already accepted pending work. The scheduled and in-flight state transitions both preserve the final reconciliation; a closed store rejects later refresh requests. With inactive/disabled observers and no pending query fetch, invalidation leaves data stale for the next enabled observer without forcing a disabled read.
- `handleFrame` compares the exact active subscription object at `use-loop-stream.ts:148`, not only workspace/run strings. Each opening creates a new object and increments generation. An old A callback remains rejected after A→B→A; returning to equal IDs cannot reauthorize that object. Cleanup also removes listeners where supported, nulls callbacks and closes the source. Previously accepted A work can still invalidate A keys after disposal, intentionally; it does not apply old frame payloads to B.
- Existing tests exercise cached-data bursts, scheduled/in-flight disposal, stream error, failed reads with a pending follow-up, listener cleanup, and a late A callback after A→B. The subsequent independent recheck exercises actual coordinator instances through A1→B→A2 and rejects a late refresh from closed A1. The hook's frame-identity fence is statically reviewed; this review did not mount an additional React test or run a browser.

Subscription-local coordinators can still cancel or duplicate reads over shared aggregate keys across simultaneous subscriptions or rapid replacement; the owning report already limits its coordination claim accordingly. No new cross-subscription scheduler was proposed by this review.
