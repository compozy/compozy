# Independent review: usage read fence and worktree invalidation

Reviewed in `/Users/pedronauck/Dev/compozy/_worktrees/responsiveness-audit-20261101` on 2026-11-02 against the working diff from baseline `de5ab2f4d`. The reviewer changed only this report. Implementation and canonical test ownership remain with `usage_dashboard` and `worktree_workspace`.

## Outcome

One confirmed P2 correctness regression was found in the provider-usage read fence and is now closed after owner remediation and focused race validation. The publication-generation guard and canonical ABA regression were inspected. No additional actionable defect was found in the reviewed worktree frontend diff.

## UW-001 — An old credential mismatch can clear a newer same-identity publication

**Severity:** P2. **Disposition:** closed on 2026-11-02 — confirmed independently, owner remediation inspected and focused race validation passed.

**Location:** `internal/providerusage/read_fence.go`, `credentialReadChecks` and `applyCredentialReadCheck`; `internal/providerusage/flight.go`, `collectOnce`; `internal/providerusage/service.go`, `targetState` and `Envelope`. The requested `service_reads.go` path does not exist in this checkout.

Moving credential I/O outside the manager mutex initially captured only the target pointer, credential epoch and identity. Those values do not distinguish a later successful publication for the same identity. The following schedule reproduced the regression without a credential reset:

1. Seed a valid usage snapshot for credential A.
2. Change the local credential file to B. Start `Envelope`, let its identity probe read B, and pause before applying the resulting mismatch against captured A.
3. Restore A, advance the fixture clock beyond refresh max-age, and complete a real local HTTP-backed `Refresh`. It publishes fresh A usage while preserving the target pointer, credential epoch and identity.
4. Resume the old B mismatch. The original guards all pass, clearing the new A snapshot and starting unnecessary recollection.

The observable result was `ok` with two usage windows becoming `refreshing` with zero windows. The old read could therefore erase successfully refreshed usage and produce redundant provider work. No credential disclosure was observed.

**Independent reproduction:** a temporary Go test overlay outside the checkout extended the existing service fixture without editing repository source/tests. It used only a synthetic credential file and the existing local `httptest` collector. A second probe gate prevented accidental background recollection from masking the cleared snapshot. All compilation and execution passed through `python3 .cache/performance/run-bounded.py`; the nested focused command was:

```text
go test -overlay <temporary-overlay.json> ./internal/providerusage -run '^TestReviewOlderReadPreservesSameIdentityPublication$' -count=1
```

Session `22384` completed before the production fix. Its exact diagnostic was:

```text
--- FAIL: TestReviewOlderReadPreservesSameIdentityPublication (0.00s)
    service_test.go:1848: older mismatch cleared fresh same-identity publication: before=ok after=refreshing before_windows=2 after_windows=0
FAIL
FAIL    github.com/compozy/compozy/internal/providerusage    0.359s
REVIEW_PROBE_EXIT 1
```

The outer diagnostic Python wrapper exited zero after printing the nested test result; `REVIEW_PROBE_EXIT 1` is the failed test status. The temporary overlay was removed on completion.

**Owner remediation reviewed:** capture a monotonic per-target `validationSeq` alongside pointer/epoch/identity and reject both a stale mismatch and a stale read-fence cache write after a newer verified publication. `collectOnce` advances this sequence only after successful ownership validation; `retryFence` does not advance it for an unverified retained error publication. Using a sequence avoids timestamp collisions with the injected clock and avoids coupling freshness to unrelated profile revisions.

**Canonical invariant and suite:** an older credential read must not invalidate a later verified usage publication, including A → B → A with unchanged final identity and credential epoch. Owner: providerusage manager. Suite: `internal/providerusage/service_test.go`, `TestUsageService/Should_preserve_a_newer_credential_publication_over_an_older_pending_read_mismatch`. The owner added the publication case to the existing identity/epoch table, without `ClearCredentials`, and retained the gate that exposes a cleared snapshot before recollection.

**Closing evidence (owner execution, inspected by reviewer):** `.cache/performance/usage-dashboard/providerusage-race.log` records `TestUsageService` passing in 1.420 s with `CGO_ENABLED=1 go test -race -parallel=2 ./internal/providerusage -run '^TestUsageService$' -count=1 -timeout=120s`, executed through the shared bounded runner. The new same-identity publication case is included. `fence-aba-before.log` records the canonical case failing before repair (0.442 s, `want ok, got refreshing`), and `fence-missing-validation.log` records the same failure (0.367 s) when only the `validationSeq` guard is removed. The negative control shows the case exercises the new fence rather than passing through unrelated behavior. `test-conventions.log` passed. This closes UW-001; no duplicate reviewer suite or live credential check was run.

## Provider-usage review coverage

Inspected `read_fence.go`, `service.go`, `flight.go`, `publication.go`, target resolution/catalog copying, `identity.go`, and relevant existing/new `service_test.go` cases.

- Credential filesystem/probe work runs outside the mutex; state capture and guarded application run under the mutex. Captured target routes and collector registry ownership were traced rather than assumed mutable.
- Pointer identity fences target removal/replacement; enabled/stopped checks fence lifecycle teardown; epoch fences credential reset and target configuration changes; identity fences a changed publication owner. UW-001 supplies the missing same-identity verified-publication dimension.
- Existing collection and publication fences continue to check target/epoch before publishing. The new generation does not bypass reconciliation or authorize an unverified error publication.
- New canonical cases cover an unrelated profile read while native credential evidence is paused, reset, removal, newer identity, newer epoch, and now newer same-identity publication. Existing privacy coverage remains `Should omit secrets identities and response bodies from envelopes and logs`.
- No identity/secret field was added to a DTO, log or error. The new read-check values remain internal. Credential identities retain process-random HMAC digests and redacted format/log methods; the test fixture uses synthetic data only.

## Worktree frontend review coverage

Inspected `hooks/use-worktree-catalog-stream.ts`, `hooks/use-worktree-stream.ts`, `lib/worktree-reread.ts`, the related `use-worktree-exit.ts` invalidation change, both canonical hook suites, query-key/options factories, both connection stores, and ticketed event-source listener disposal.

No actionable introduced defect was found in these paths:

- **Burst and trailing:** the shared helper is the previous detailed-stream implementation extracted intact. Each request resets the 150 ms quiet timer; only the first request arms the 1 s ceiling. Firing clears both timers before the reread, allowing the next event to start another window. Existing detailed-stream coverage exercises two ceiling periods and a later trailing event.
- **Catalog completeness:** pending workspace IDs form a set. A flush invalidates each affected workspace prefix and the aggregate prefix once. Canonical keys confirm that a workspace prefix includes its list, detail, status and exit reads; removing redundant detail invalidation does not lose a read model.
- **Scope:** frame validation and authorized-workspace membership run before either the immediate failure handoff or pending-set mutation. A scope replacement closes/removes old listeners, flushes only its captured pending scope, and creates a new connection with the new authorized set. Sorted/deduplicated scope keys avoid reconnecting on reordering.
- **Disconnect and unmount:** catalog error and disposal flush pending work and cancel both timers. Detailed-stream disposal does the same. Its error callback continues to mark stale while the existing bounded timer remains armed; this is unchanged behavior, not an introduced loss of pending work. Named listeners are removed by the original references, and the ticketed source removes/clears its listeners on close.
- **Per-worktree invalidation:** the workspace list now uses `exact: true`, alongside already exact detail/status/exit invalidations. Current option factories use those exact keys, and sibling worktree details no longer refetch. The materialization-failure handoff is cache-only and disabled for fetching.
- **Status generation:** old connection status callbacks are ignored by the existing store generation fence. Immediate materialization failure text is still written before the delayed reread.

**Owner-provided focused evidence, read by the reviewer:** `.cache/performance/worktree-workspace/stream-after.log` records bounded root Turbo with pinned Bun 1.4.0, three canonical files and 28 passing tests: catalog stream 18, detailed stream 6, exit 4. The command was `bunx turbo run test --filter=./web --only -- src/systems/workspace/hooks/__tests__/use-worktree-catalog-stream.test.tsx src/systems/workspace/hooks/__tests__/use-worktree-stream.test.tsx src/systems/workspace/hooks/__tests__/use-worktree-exit.test.tsx --maxWorkers=1`. Actual QueryObserver query-function counts were `[1,1,1]` for 100 catalog frames and `[1,1,1,1,0]` for a detailed scope wake with a sibling observer. These receipts are owner execution evidence, not a separate reviewer rerun or browser-latency proof.

## Limits

This is a targeted working-tree review with one independently executed adversarial repro, not an exhaustive concurrency proof. No full suite, gate, daemon, browser, production credential source, Keychain, real provider endpoint, or production state was used. Frontend conclusions combine source/test review with the owner's focused receipt; native browser scheduling and reconnect over a real network were not independently exercised. Final delivery gates and broader runtime QA remain with the root. No additional production/test edits were made by this reviewer.
