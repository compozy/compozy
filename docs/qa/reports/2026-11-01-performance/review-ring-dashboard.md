# Independent review: terminal ring and dashboard live refresh

Reviewed the working-tree diff against `de5ab2f4d`, the complete three production
files, their canonical suites, terminal replay consumers, and the installed
TanStack Query / XState Store implementations. This review did not modify
production code or tests.

## Finding

### RD-001 · P2 — Preserve the pending reread until the leading request settles

Disposition: resolved after the owning agent's correction, canonical regression
tests, and an independent post-fix diagnostic. Original evidence is retained
below; the verified correction is recorded in the post-fix section.

Original location: `web/src/systems/dashboard/hooks/home-live-refresh-store.ts:76`
(the immediate invalidation and state reset at lines 73–82).

`scopeDeactivated` invokes the pending invalidation immediately, then clears
`pending` and advances the generation. That invalidation is not necessarily a
new read: TanStack skips disabled observers, and it reuses an in-flight initial
fetch when the query has no data. The original leading request can subsequently
publish its older snapshot and clear `isInvalidated`; its settlement is then
ignored by the generation fence, so the pending lifecycle change is lost.

Confirmed regression with the real installed `QueryClient`, `QueryObserver`, and
`homeLiveRefreshLogic`, using a deferred query function and no network:

1. Seed an observed aggregate at revision 0 and start the leading read on a
   lifecycle event at revision 1.
2. Receive a second lifecycle event at revision 2, then disable the aggregate
   observer before deactivating the stream scope.
3. Resolve the outstanding reads.

| Path | Requests | Final cached revision | `isInvalidated` |
| --- | ---: | ---: | --- |
| Previous immediate per-event invalidation control | 2 | 2 | false |
| Current coalescing logic | 1 | 1 | false |

A second diagnostic with an unseeded, still-active observer also loses the
pending update on close: the initial request is reused and completes at revision
0, with one request and `isInvalidated: false`. This second edge is additional
coverage for the new close guarantee; the seeded/disabled case above is the
before/after regression.

The existing close test retains seeded, active observers outside the hook, so
its immediate cleanup invalidation can cancel/refetch successfully and does not
expose this branch. Keep the pending reread or dirty state until the leading
request can no longer overwrite it. Extend the existing
`web/src/systems/dashboard/__tests__/use-home-live.test.tsx` suite for disabled
and initially empty observers at close, preserving current-generation isolation
and callbacks bound to the correct QueryClient.

The diagnostic ran via `python3 .cache/performance/run-bounded.py bun -` and
completed successfully. Its compact output was:

```json
{"mode":"baseline","seed":true,"disableOnClose":true,"requests":2,"value":2,"invalidated":false}
{"mode":"current","seed":true,"disableOnClose":true,"requests":1,"value":1,"invalidated":false}
{"mode":"current","seed":false,"disableOnClose":false,"requests":1,"value":0,"invalidated":false}
```

The control executes the previous per-event `invalidateQueries` path; this was
not a full baseline browser run.

### RD-001 post-fix verification

The corrected store retains the generation with pending work when its scope is
retired. On that generation's settlement, it consumes the obligation exactly
once and invalidates through the callback captured by the original request.
That path preserves the active generation's `refreshing` and `pending` state.
Scope activation preserves any other retired obligations, and cleanup no longer
invalidates before the leading read settles.

The owning agent added the two cases to the existing hook suite. Both failed
before the fix: the disabled observer incorrectly became fresh, and the empty
observer never started the trailing request. Receipt:
`.cache/performance/usage-dashboard/home-close-before.log`.

The final existing case `Should preserve retired refreshes across profile and
QueryClient changes` leaves both the old A and replacement A dirty, across an
A→B→A transition with distinct QueryClients. Settling the old client starts only
its own trailing reads; the replacement still executes its pending read when
its own leading requests settle. Both clients converge to revision 4, with six
requests and zero cancellations each. The final owner-run dashboard group
passed 63 tests in nine files, including these cases; receipt:
`.cache/performance/usage-dashboard/dashboard-final-test.log` (Turbo execution
`be9ec2b5ea38159e`).

The reviewer independently reran the original QueryClient diagnostic after the
fix, through the bounded runner with Bun 1.4.0:

```json
{"seed":true,"disableOnClose":true,"beforeReactivation":{"requests":1,"value":1,"invalidated":true},"final":{"requests":2,"value":2,"invalidated":false},"retiredGenerations":[]}
{"seed":false,"disableOnClose":false,"beforeReactivation":{"requests":2,"value":2,"invalidated":false},"final":{"requests":2,"value":2,"invalidated":false},"retiredGenerations":[]}
```

Thus disabled queries retain the pending invalidation and refresh on activation;
active initially empty queries complete the trailing read. No new defect was
found in the corrected state transitions. Post-fix SHA-256 values:

```text
ebbd5c661600b5de66225376bbcc79e59ff2dc53abfb6c9f26c3fc4de8ad89fb  web/src/systems/dashboard/hooks/home-live-refresh-store.ts
372a7a3e93170fed2f4f4f8a78e74578a3067211af9d97a747edfb2b624a609e  web/src/systems/dashboard/hooks/use-home-live.ts
3bd9b7dcb83a1b805693ef7dc43c59daf21a210bb32d77d4c661ac0a8ef9ca51  web/src/systems/dashboard/__tests__/use-home-live.test.tsx
```

## No other defect found

- `internal/terminal/ring.go`: append copies caller bytes under the write lock;
  replay strings and snapshots are independently copied under the read lock.
  Evicted segment slots are cleared, oversized retained data is cloned after
  trimming, absolute offsets/marker gaps follow the prior logic, and the bounded
  newline search covers exactly the prior 256-byte window. No new ownership,
  UTF-8 trimming, snapshot mutation, or read/write race defect was found.
- Dashboard: generation checks fence old completions through A→B→A transitions;
  the per-effect `active` guard rejects queued old activity frames; query keys
  remain scoped; trailing callbacks retain their originating QueryClient. No
  separate defect was found in those paths.

## Scope and limits

No full suite, browser, daemon, benchmark, or new race run was launched by this
reviewer. Ring/race receipts and the owner's post-fix dashboard suite receipts
were inspected, not claimed as new reviewer executions. The independent
diagnostics exercise the real store/QueryClient; the complete UI sequence was
not replayed. Root integration and current-head CI remain separate evidence.

Initially reviewed production SHA-256 values:

```text
fcfdfbe343852580dfde241b8cabc8783b8bc55b678f212dd29b2425fee66a84  internal/terminal/ring.go
7929398d51b3c8c992e9d1855cdd92e28816a52af1f18eb30d5bad0dac627e0f  web/src/systems/dashboard/hooks/home-live-refresh-store.ts
0ec7e4a0de96455c98fee4db3f1d510f5e0a30b31d6ec9e6a3cd947aafe89d46  web/src/systems/dashboard/hooks/use-home-live.ts
```
