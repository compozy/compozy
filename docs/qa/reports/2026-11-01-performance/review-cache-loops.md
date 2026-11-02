# Independent review: package cache and Loop/Home refresh

## Final shared coordinator review — 2026-11-02

**No concrete outstanding defect found in the frozen shared coordinator or its Loop/Home integrations.** This pass waited for both owners' freezes, verified their source hashes, read the installed TanStack Query 5.104.5 implementation, and inspected the owners' canonical tests, negative controls and receipts. It changed only this report. It did not execute tests, benchmarks, gates, a daemon or a browser.

The earlier read-only assessment of the Loop-only coordinator `ef96bb09cbdd175d8392c12e9f53a72b8a200d5e35b758a6ed5dbc31511e41b2` and suite `41a81b16d3ee754e0cea52505ff7328c260f657166e52a433538d045bf205d4a` is **historical**. That revision removed the shared read barrier but had no bounded same-query recovery. The shared helper and Home migration below supersede that implementation and its review verdict.

### Reviewed freeze

| File | Verified SHA-256 |
| --- | --- |
| `web/src/lib/query-live-refresh-coordinator.ts` | `6a2a174355c4303d4f4a2133fb3b6963fc6a4a2da7180821df15ce342d3b74dd` |
| `web/src/systems/loops/lib/loop-live-refresh-coordinator.ts` | `cd5c2f9b98605c5ffe8eae66e754666f1b696bfa76bf7f3fde6f2c5dc5bbf26f` |
| `web/src/systems/loops/lib/query-options.ts` | `81e17cbab464fd903a477f0e048047dd93137052641206c8472e135e1b516f03` |
| `web/src/systems/loops/hooks/__tests__/use-loop-stream.test.tsx` | `3b40471a6794d7ad0390f3b2ff2e0533b17cebb1a35eccbe69c56ec83c437bd4` |
| `web/src/systems/dashboard/hooks/home-live-refresh-store.ts` | `bb7b67a44d8032c3c90e9af43c26cbd874e0dfa22f9317ee446a9ea7e317d76a` |
| `web/src/systems/dashboard/hooks/use-home-live.ts` | `99d8937cb4cf4c71b5d86b4e049b5da0187adfbd35a7139314fc1bea42d2fdcf` |
| `web/src/systems/dashboard/lib/query-options.ts` | `eaf167a2f9aa2171c2807697223f828fbbb895789bc4c48f9913603ffb57ed59` |
| `web/src/systems/dashboard/lib/query-policy.ts` | `066bba419504370ef840aba72b0d9ff3c2a3dd5088e96b313b60b48366dab739` |
| `web/src/systems/dashboard/__tests__/use-home-live.test.tsx` | `6d2fdefab2c6c19561cd3f16150beda661f66fe069749f65392a69f583c98723` |
| `web/e2e/scripts/benchmark-home-live.ts` | `416823f8965a4d64f6412847e1649e2483e52c28bf892160e1eaddf4e60a91f8` |

Home's owner freeze receipt is `.cache/performance/usage-dashboard/pr1141-home-freeze.json`. The Loop L1 and stalled-read receipts also record the shared-helper and facade hashes above. Source hashes matched the announced freezes when this review concluded.

### Contracts and ordering

- **Query identity and scope:** `refresh(queryKey)` selects with TanStack's existing partial-key matching, then tracks actual `Query` objects. Trailing invalidation selects that same object, preserving per-query custom hashes and avoiding accidental lookup through a default hash. Workspace/run/profile key shapes are unchanged. The Loop scope union remains microtask-batched; `runReads` includes the Goal-turn descendants. Home retains its existing overview/dashboard/inbox roots.
- **Independent progress:** the persistent map belongs to one coordinator invocation, created per Loop subscription or Home effect/QueryClient. Busy queries receive one pending bit; settled siblings remain eligible immediately. Home no longer awaits one `Promise.all` before accepting the next lifecycle refresh. No global coordination registry or cross-scope response cache was introduced.
- **L1 and pending consumption:** an initial, inactive or static pre-event fetch retains its trailing obligation. The drain waits with `allSettled`, clears the pending bit immediately before the fresh invalidation, and checks it again afterward. Thus wakes already accepted before that fresh read are consumed together, while wakes accepted during it request another read. Default active-only refetch leaves disabled/static queries invalidated without forcing a request. Read failure does not skip the pending drain, and `finally` releases the local entry.
- **Bounded restart:** only the first pending wake arms a timer while an eligible read is running. Later frames do not renew it. Loop supplies its existing 15-second live-read interval; Home supplies its existing 30-second overview interval. At expiry, the callback checks the captured promise is still current, the follow-up remains owed, and the query is fetching, active, cached and nonstatic. Cancellation uses `revert: true` and releases the existing drain; it does not start a competing refresh queue. A timer for a replaced promise cannot cancel the replacement.
- **Timer cleanup and settlement:** the drain clears the timer after the initial wait, after each trailing invalidation, and in `finally`. Timeout execution clears its own handle before checking eligibility. The synchronous reset/invalidate sequence consumes wakes before the new read; later callbacks cannot interleave between the final loop check and map deletion. Accepted work can outlive source closure, but a settled record leaves no coordinator timer behind.
- **Disposal and ABA:** Loop retains its exact active-subscription-object fence and closed-store admission. Home retains an effect-local `active` flag in addition to the URL/lens scope gate; returning A→B→A cannot reauthorize a saved A1 handler. Each retired coordinator captures its original QueryClient. Its eventual settlement does not mutate a replacement effect's pending registry. Closing the source intentionally does not discard an already accepted freshness obligation.
- **Home activity and pulse:** schema validation, scoped activity keys, duplicate suppression and the 30-entry cache limit are unchanged. Ordinary events still admit overview refresh at most once per minute; task lifecycle events still bypass that throttle and update its timestamp. The pure `query-policy.ts` move keeps one 30-second constant, consumed by query options, the live hook and the benchmark, with no compatibility alias or changed value.

### Owner-run evidence inspected

| Evidence | Observed result | Receipt |
| --- | --- | --- |
| Loop deadline negative control on the prior implementation | Active and disposed pending-cached cases fail; five boundary controls pass | `loop-loops/same-query-deadline-negative.txt` |
| Loop canonical hook suite after extraction | **39/39 pass**, including first-dirty timing, no timer renewal, promise replacement during trailing refresh, disabled/initial/static reads and timer cleanup | `loop-loops/same-query-deadline-after.txt` |
| Existing L1/replacement/cleanup diagnostic | **8/8 pass**, with unchanged assertions and loader updated for the extracted helper | `loop-loops/same-query-l1-recheck.txt` |
| Loop delayed-I/O fixture | **100 frames → 3 reads / 0 cancellations / revision 100**; a stalled pending cached read restarts at the 15-second dirty deadline | `loop-loops/same-query-stall-after.txt` |
| Loop/helper formatting and lint | Pass | `loop-loops/same-query-format.txt`, `same-query-lint.txt` |
| Home independent-sibling negative control | Suspended overview leaves request counts `1/1/1/1`, failing required `1/2/2/2` | `usage-dashboard/pr1141-home-stall-before.log` |
| Home canonical hook and dashboard suites | **33/33 pass**: 16 live-hook cases and 17 dashboard cases, including independent overview variants and saved retired A1 handler after A→B→A | `usage-dashboard/pr1141-home-live-final.log` |
| Home 100-event fixture, five samples | Baseline **300 reads / 297 cancellations**; shared-helper integration **6 reads / 0 cancellations**, all three final revisions 100 in every sample | `usage-dashboard/pr1141-home-benchmark-before.log`, `pr1141-home-benchmark-after-final.log` |

Receipt paths in this table are relative to `.cache/performance/`. The transient Home benchmark import failure in `pr1141-home-benchmark-after.log` did not run the fixture and is not accepted evidence; the final pure-policy import produced the passing receipt above. The benchmark uses the real helper and installed QueryClient/QueryObserver with controlled I/O, not mocked coordinator behavior. The counts establish bounded request amplification, not browser responsiveness or endpoint latency.

### Limits

The timer bounds when an eligible pending read can be restarted, subject to event-loop scheduling; it does not guarantee that the replacement network request completes within 15 or 30 seconds. Initial, disabled and static reads deliberately retain their settlement obligations without forced timer cancellation. A permanently unresolved one can retain its own local record, but it cannot hold healthy siblings behind a shared barrier.

Coordination is local to each subscription/effect. Existing cache operations or simultaneous subscriptions can still replace the same underlying Query; this review does not claim global request deduplication. Timer identity prevents the old timeout from canceling a different promise. The tests use actual Query clients and controlled asynchronous I/O; this review did not add an independent execution, real-browser/provider test, or current-head CI verdict. Root owns integrated delivery validation. The package-cache/filesnap findings below were not re-executed in this final helper pass.

## Historical audit before the shared helper

The following sections preserve the earlier audit and its original receipts. Loop implementation descriptions and prior independently executed diagnostics below are historical, not assertions that the current shared-helper implementation has the former whole-round structure.

Reviewed the uncommitted performance diff against `de5ab2f4d` on 2026-11-02. That earlier review changed no production or canonical test source. Only this report and temporary diagnostics under `.cache/performance/review-cache-loops/` were written. No daemon, browser, production state, full suite or gate was started.

## Historical result

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

These observations describe the initial implementation. The per-query follow-up below
supersedes its global read-completion barrier while preserving scope and cleanup contracts.

- The pending set contains at most four scope labels. Each round drains it; events received during a round request one follow-up. Request/catalog extras are retained in the union. When a run refresh includes goal-turn events, the `runReads` prefix covers the goal-turn descendant key, so suppressing a redundant exact key does not omit it.
- Cleanup closes event admission while retaining already accepted pending work. The scheduled and in-flight state transitions both preserve the final reconciliation; a closed store rejects later refresh requests. With inactive/disabled observers and no pending query fetch, invalidation leaves data stale for the next enabled observer without forcing a disabled read.
- `handleFrame` compares the exact active subscription object at `use-loop-stream.ts:148`, not only workspace/run strings. Each opening creates a new object and increments generation. An old A callback remains rejected after A→B→A; returning to equal IDs cannot reauthorize that object. Cleanup also removes listeners where supported, nulls callbacks and closes the source. Previously accepted A work can still invalidate A keys after disposal, intentionally; it does not apply old frame payloads to B.
- Existing tests exercise cached-data bursts, scheduled/in-flight disposal, stream error, failed reads with a pending follow-up, listener cleanup, and a late A callback after A→B. The subsequent independent recheck exercises actual coordinator instances through A1→B→A2 and rejects a late refresh from closed A1. The hook's frame-identity fence is statically reviewed; this review did not mount an additional React test or run a browser.

Subscription-local coordinators can still cancel or duplicate reads over shared aggregate keys across simultaneous subscriptions or rapid replacement; the owning report already limits its coordination claim accordingly. No new cross-subscription scheduler was proposed by this review.

## Per-query Loop refresh follow-up — 2026-11-02

An independent read-only review checked the final coordinator SHA-256
`ef96bb09cbdd175d8392c12e9f53a72b8a200d5e35b758a6ed5dbc31511e41b2` and canonical
suite SHA-256 `41a81b16d3ee754e0cea52505ff7328c260f657166e52a433538d045bf205d4a`.
No concrete defect was found in scope selection, custom hashing, initial/disabled-read
freshness, pending consumption, error settlement or disposal. One local registry entry
and pending bit belong to each existing Query; an indefinitely pending request retains
only its own accepted obligation and cannot block healthy queries.

The reviewer inspected the owner's 32/32 canonical cases, unchanged 8/8 independent
controls, failing-before mixed-query controls and preserved 100-event / 3-read /
zero-cancellation fixture. This follow-up was source review and receipt inspection,
not another test, benchmark or browser run. Cross-subscription coordination and a
network response deadline remain outside the implementation's claims.
