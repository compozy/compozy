---
provider: manual
pr:
round: 8
round_created_at: 2026-07-24T19:59:29Z
status: resolved
file: internal/core/worktree/review_isolation.go
line: 1004
severity: medium
author: claude-code
provider_ref:
---

# Issue 008: Byte-exact index compare aborts on benign stat refresh

## Review Comment

`requireUnchangedGitIndex` compares the raw `.git/index` bytes:

```go
if current.path != expected.path || !bytes.Equal(current.content, expected.content) {
    return gitIndexBackup{}, errors.New("source git index changed since review isolation began")
}
```

Git rewrites the index whenever it refreshes its stat cache, with semantically
identical entries. Verified: after `touch`ing tracked files, a plain `git status`
changes the index file's checksum while `git ls-files --stage` stays
byte-identical.

Failure: the user has VS Code or lazygit open on the source repo — their git
integration polls `git status` — during a long concurrent review run with
`AutoCommit`. The index is rewritten with no semantic change, so `Apply` fails at
line 385 with "source git index changed since review isolation began", and every
remaining auto-commit batch fails the same way. All work is stranded in the
private worktrees, and the trigger is invisible to the user.

The package already has the correct semantic comparator: `gitIndexesMatch`
(line 1096), used by `reconcileSourceIndex`.

Fix: gate this check on `gitIndexesMatch` and keep the byte copy only as the
rollback payload for `restoreGitIndexCAS`. Add a test that refreshes the stat
cache between capture and validate and asserts the batch still applies.

## Triage

- Decision: `VALID`
- Root cause: `requireUnchangedGitIndex` (line 1004) compares the raw `.git/index`
  bytes. Git rewrites the index whenever it refreshes its stat cache
  (semantically identical entries), so a benign `git status` from an editor's git
  integration during a concurrent auto-commit run changes the byte content and
  aborts every remaining batch with "source git index changed since review
  isolation began", stranding work in the private worktrees.
- Verified on this machine: after `touch`ing tracked files and
  `git update-index --refresh`, the index bytes change while `git ls-files --stage`
  stays byte-identical.
- Fix: gate the check on the package's existing semantic comparator
  `gitIndexesMatch` (line 1096, already used by `reconcileSourceIndex`) instead of
  `bytes.Equal`. Keep returning the freshly captured byte snapshot as the rollback
  payload for `restoreGitIndexCAS`, so rollback semantics are unchanged.
- Test: new `TestRequireUnchangedGitIndexToleratesStatRefresh` refreshes the stat
  cache between capture and validate and asserts no error while the returned
  payload carries the current bytes.
- Notes: change constrained to `review_isolation.go` and its test file.
- Verification: `make fmt`/`make lint` clean; `go test -race ./internal/core/worktree/`
  passes (36 tests, incl. the new stat-refresh tolerance test); `go build ./...` clean.
  The 24 `make verify` failures are the pre-existing environmental `rundb: schema too new
  (db=4 binary=3)` skew in `internal/core/plan` / `internal/daemon`, unrelated to this
  change. See issue_002 for the full note.
