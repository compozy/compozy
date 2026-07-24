---
provider: manual
pr:
round: 8
round_created_at: 2026-07-24T19:59:29Z
status: resolved
file: internal/core/worktree/review_isolation.go
line: 557
severity: high
author: claude-code
provider_ref:
---

# Issue 003: merge-file binary failure misreported as a hunk conflict

## Review Comment

`runGitMergeFile` classifies the result as a conflict whenever the exit code is
non-zero and stderr is empty:

```go
if errors.As(err, &exitErr) && exitErr.ExitCode() > 0 && strings.TrimSpace(stderr.String()) == "" {
    return true, nil
}
```

The comment above it states the premise directly — "-q suppresses conflict
warnings, so a non-zero exit with empty stderr is a" conflict — but that premise
is wrong. `git merge-file` returns the conflict count (1..127) on success and
**255 on error**, and `-q` suppresses the error text too, so both cases present
as "non-zero exit, empty stderr".

Verified on this machine (git 2.50.1):

```
BINARY exit=255 stderr=[]        # error, misclassified as conflict
TEXTCONFLICT exit=1 stderr=[]    # real conflict
```

Failure: batches A and B make non-overlapping edits to a shared text file, so B
takes the 3-way path. `mergeIsolatedBatch` merges *every* path in the batch,
including a binary one B touched (a `.png`, a golden fixture) that no other batch
changed. merge-file exits 255, is classified as conflicted, `applyIsolatedPatch`
rolls the whole batch back via `restoreSourcePathContents` and returns
`ErrOverlappingReviewEdits on <binary>`. B's completed work is discarded and
parked with a cause that never happened, so the operator debugs a phantom
conflict.

Fix: treat only `1 <= ExitCode() <= 127` as a conflict and any other non-zero code
as a hard error. Optionally drop `-q` and inspect stderr for the real diagnostic
("Cannot merge binary files"). Add a test merging binary inputs and asserting an
error rather than a conflict.

## Triage

- Decision: `VALID`
- Root cause: `runGitMergeFile` (line 557) treats *any* non-zero exit with empty
  stderr as a conflict. But `git merge-file` returns the conflict count (1..127) on
  a successful-but-conflicted merge and 255 on a hard error, and `-q` suppresses the
  error text too — so a binary-file merge error (exit 255, empty stderr) is
  misclassified as a mergeable conflict. That surfaces `ErrOverlappingReviewEdits`
  on a path no other batch touched and discards a completed batch as a phantom
  conflict.
- Verified on this machine (git 2.50.1): text conflict → exit 1, stderr empty;
  binary error → exit 255, stderr empty *with* `-q`, and
  `error: Cannot merge binary files: <path>` *without* `-q`.
- Fix: classify strictly by exit code — only `1 <= ExitCode() <= 127` is a
  conflict; every other non-zero code is a hard error. Drop `-q` so the genuine
  diagnostic (e.g. "Cannot merge binary files") reaches the wrapped error; dropping
  `-q` adds no stderr noise on ordinary text conflicts. Corrected the misleading
  doc comment stating the wrong premise.
- Test: new subtest in `TestRunGitMergeFileMergesAndDetectsConflicts` merges binary
  inputs and asserts an error (not a conflict).
- Notes: change constrained to `review_isolation.go` and its test file.
- Verification: `make fmt`/`make lint` clean; `go test -race ./internal/core/worktree/`
  passes (36 tests, incl. the new binary-merge subtest); `go build ./...` clean. The 24
  `make verify` failures are the pre-existing environmental `rundb: schema too new
  (db=4 binary=3)` skew in `internal/core/plan` / `internal/daemon`, unrelated to this
  change. See issue_002 for the full note.
