---
status: resolved
file: internal/store/rundb/run_db_test.go
line: 485
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc58ixss,comment:PRRC_kwDORy7nkc654MFa
---

# Issue 007: _⚠️ Potential issue_ | _🟠 Major_
## Review Comment

_⚠️ Potential issue_ | _🟠 Major_

**This concurrency test can pass without ever creating contention.**

At Line 459, `db.now` is invoked after `UpsertIntegrity` has already taken `integrityMu`, so only one goroutine can ever signal `nowEntered` at a time. Then `waitForConcurrentIntegrityRead` just returns on timeout at Line 538, which lets the test fall back to sequential execution and still pass. As written, this does not prove the merge path is safe under concurrent callers.

As per coding guidelines, "Prefer root-cause fixes in failing tests over workarounds that mask the real issue".


Also applies to: 529-540

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/store/rundb/run_db_test.go` around lines 457 - 485, The test is
ineffective because db.now is called after UpsertIntegrity has already taken
integrityMu, so only one goroutine ever signals nowEntered; fix this by moving
the timestamp acquisition out of the integrityMu critical section: in
UpsertIntegrity, call db.now() (the now hook) before acquiring integrityMu so
multiple goroutines can enter and block on the test's releaseNow/nowEntered
signals; ensure any state that depends on the timestamp is used after the lock
is acquired and that you keep integrityMu protection for shared state updates.
```

</details>

<!-- fingerprinting:phantom:medusa:grasshopper:e40d238e-ad07-4f04-8bea-f476de16b781 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `valid`
- Root cause: the current concurrency test waits for both goroutines to hit `db.now`, but `db.now` runs after `integrityMu` is acquired, so the wait helper can time out and still let the test pass without ever proving a second caller contended with the first.
- Plan: strengthen the test to deterministically start a second `UpsertIntegrity` call while the first call is blocked inside the critical section, and keep timestamp acquisition under the mutex so `UpdatedAt` cannot move backwards because of pre-lock timestamps.
- Resolution: rewrote `TestRunDBUpsertIntegritySerializesConcurrentMerges` so the first update blocks inside `db.now` while holding `integrityMu`, then launches a second `UpsertIntegrity` caller before releasing the first. The obsolete timeout helper was removed. The production path in `run_db.go` was intentionally left unchanged so timestamp capture remains serialized with the merge.
- Regression coverage: the updated test now proves that a second concurrent caller is present during the first merge and that the final persisted integrity state contains both reasons.
- Verification: `go test ./internal/store/rundb -run 'Test(RunDBCloseContext|RunDBUpsertIntegritySerializesConcurrentMerges)$' -count=1` passed. `make verify` then passed with `2544` tests and `2` skipped helper-process tests.
