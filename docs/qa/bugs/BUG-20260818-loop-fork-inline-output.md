# BUG-20260818-loop-fork-inline-output: Fork rejects generations with inline outputs

- **Status:** fixed
- **Impact (user-side):** Blocks-Completion
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Bruno
- **Journey Step:** J-replay-loop-history, fork from a settled generation
- **Scenarios:** LP-time-travel-fork; LP-web-fork-dialog
- **Found:** 2026-08-18 · **Report:** docs/qa/reports/2026-08-18-graph-eng.md
- **Origin:** Task 11 isolated graph-eng QA

## Summary

Forking a completed run failed when the source generation contained an inline JSON output. The store validated every non-empty `output_ref` as a content-addressed blob, even though small outputs are stored inline in the same column.

## Reproduction

1. Run a Loop with an `ask` node and answer it with `{"environment":"staging"}`.
2. Wait for the run to finish.
3. Run `compozy loop fork --generation 1` against the settled run.

**Expected:** A linked child run starts from generation 1 without mutating the source.

**Actual:** Fork failed with `loop: output ref not found: fork seed output "{\"environment\":\"staging\"}"`.

## Fix

- **Root cause:** `validateForkSeedBlobs` checked inline values against `loop_output_blobs` instead of limiting the lookup to valid `sha256:` references.
- **Correction:** Validate blob existence only when `OutputRefLooksContentAddressed` is true; preserve inline values unchanged in the fork seed.
- **Fix batch:** graph-eng final QA remediation
- **Regression test:** `TestGlobalDBLoopTimeTravelShouldCommitOneAtomicOperation` now seeds both a content-addressed blob and inline JSON, while retaining the missing-blob rollback case.

## Verification

- `go test -race ./internal/store/globaldb -run '^TestGlobalDBLoopTimeTravelShouldCommitOneAtomicOperation$' -count=1` — pass.
