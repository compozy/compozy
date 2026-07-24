---
provider: manual
pr:
round: 8
round_created_at: 2026-07-24T19:59:29Z
status: resolved
file: internal/core/worktree/review_isolation.go
line: 518
severity: medium
author: claude-code
provider_ref:
---

# Issue 009: Three-way merge path drops file mode changes

## Review Comment

`mergeSourcePathThreeWay` uses `theirsMode` only when creating a file that does
not already exist in the source:

```go
if !oursExists {
    if err := os.WriteFile(oursAbs, nil, theirsMode.Perm()); err != nil {
```

For an existing file, `git merge-file` writes content in place and preserves
*ours'* mode. Verified: merging a 0755 "theirs" into a 0644 "ours" leaves 0644.

Failure: a review issue is "make scripts/setup.sh executable". The batch chmods
it, and some other path in the same batch drifted, so the whole batch takes the
merge path. With `AutoCommit`, `setup.sh` is non-drifted so it is in
`comparePaths`, and `validateStagedReviewIndex` sees `100644` in the source vs
`100755` in the workspace → "staged source entries differ from isolated review
results" → full rollback, and the batch is parked for a change that had no
conflict at all. Without `AutoCommit` there is no such check, so the mode change
is silently lost and the review issue is reported resolved while the file stays
non-executable.

Fix: after a clean merge, apply `theirsMode.Perm()` to `oursAbs` — at minimum
propagate the executable bit. Add a test covering an executable-bit-only change
that rides along in a merged batch.

## Triage

- Decision: `VALID`
- Root cause: `mergeSourcePathThreeWay` applies `theirsMode` only when creating a
  file that does not already exist in the source (line 522). For an existing file
  `git merge-file` rewrites content in place and preserves *ours'* mode, so a
  mode-only change carried by the batch (e.g. `chmod +x scripts/setup.sh`) is lost.
  With `AutoCommit` the non-drifted executable path fails
  `validateStagedReviewIndex` (`100644` source vs `100755` workspace) and the whole
  batch is rolled back and parked for a change that never conflicted; without
  `AutoCommit` the mode change is silently dropped while the issue is reported
  resolved.
- Fix: after a clean (non-conflicted) merge, propagate `theirsMode.Perm()` to
  `oursAbs` via `os.Chmod`, so the batch's mode change (including the executable
  bit) lands in the source. On a conflict the path is rolled back anyway, so mode
  is only applied on a clean merge.
- Verified the merge path is genuinely taken when a sibling path drifts with
  overlapping context (strict `git apply` fails with "patch does not apply", git
  2.50.1), which is exactly what the new test reproduces.
- Test: new `TestReviewIsolationPropagatesExecutableBitThroughMerge` rides an
  exec-bit-only change alongside a drifted sibling file under `AutoCommit` and
  asserts the source file ends up `100755` and committed.
- Notes: change constrained to `review_isolation.go` and its test file. The chmod
  addition pushed `mergeSourcePathThreeWay` over the gocyclo limit, so the batch-deletion
  branch was extracted into `mergeDeletedSourcePath` (behavior-preserving) to keep lint at
  0 issues.
- Verification: `make fmt`/`make lint` clean; `go test -race ./internal/core/worktree/`
  passes (36 tests, incl. the new exec-bit-through-merge test); `go build ./...` clean.
  The 24 `make verify` failures are the pre-existing environmental `rundb: schema too new
  (db=4 binary=3)` skew in `internal/core/plan` / `internal/daemon`, unrelated to this
  change. See issue_002 for the full note.
