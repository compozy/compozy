---
provider: manual
pr:
round: 8
round_created_at: 2026-07-24T19:59:29Z
status: resolved
file: internal/core/worktree/review_isolation.go
line: 631
severity: high
author: claude-code
provider_ref:
---

# Issue 002: git add -f -A commits gitignored .compozy artifacts

## Review Comment

`commitReviewPatch` stages with `git add -f -A --` (line 631) against the *source*
root. `-f` overrides `.gitignore`, and `.gitignore:59` ignores `.compozy/**`.

The leak path is end-to-end: `createReviewWorkspace` force-adds `artifactRel`
(line 239) so `.compozy/tasks/<name>/**` becomes *tracked inside the isolated
worktree*. Gitignore only suppresses untracked files, so the agent's edit to
`reviews-NNN/issue_001.md` is picked up by `git add -A` (line 354) and lands in
`paths`. `commitReviewPatch` then force-adds that path in the source, where it is
ignored, and `git commit --only` commits it. Afterwards `git ls-files -- .compozy`
lists it permanently. Every auto-commit batch leaks, because
`FinalizeIssueStatuses` always rewrites the issue files.

This already happened on this branch: commit `2ddc29b refactor: untrack committed
review-round issue files` was cleanup for exactly this.

`-f` is load-bearing only in that dropping it makes `git add` exit 1
("paths are ignored ... use -f"), so the fix is to exclude artifact paths from the
staged set, not to drop `-f`. Line 239's `-f` is legitimately intentional.

Same root cause, other affected sites — fix as one policy:

- line 1029: second `add -f -A` site; must filter identically or the expected
  index diverges from the staged one.
- line 354: the mirror-image gap. `Apply` uses `git add -A` with **no** `-f`, so a
  *new* file the agent writes under the artifact dir (e.g. `memory/context.md`) is
  untracked+ignored, never staged, never reaches the source, and is destroyed by
  `Cleanup`'s `git worktree remove --force`. No first-party writer creates such a
  file today, which is why this half is currently latent.

Fix: decide one artifact-path policy and apply it symmetrically at 239 / 354 /
631 / 1029. Add a test asserting a review commit on a repo that ignores
`.compozy/**` leaves `git ls-files -- .compozy` empty.

## Triage

- Decision: `VALID`
- Root cause: `commitReviewPatch` (line 631) force-stages the batch delta into the
  *source* with `git add -f -A`, and `-f` overrides `.gitignore`. Because the seed
  commit force-adds `artifactRel` inside the isolated worktree (line 239), the
  agent's edits to `.compozy/tasks/<name>/**` are tracked there, land in `paths`
  via `git add -A` (line 354), and are then force-committed into the source where
  they are ignored — leaking workflow artifacts into history on every auto-commit.
- Confirmed the architecture: `execution.go:562-563` points each job's
  `WorkspaceRoot`/`ReviewsDir` at the isolated worktree, and `FinalizeIssueStatuses`
  writes issue files there; `Apply`'s patch is what carries them to the source
  working tree. So artifacts must still reach the source *working tree* (on disk,
  where the workflow reads resolved statuses and memory) but must never be *staged
  or committed*.
- Fix (one policy applied symmetrically at 239 / 354 / 631 / 1029):
  1. Store `artifactRel` on `ReviewIsolation`.
  2. Line 354: after `git add -A`, force-stage the artifact tree in the worktree
     (`git add -f -A -- artifactRel`) so *new* ignored artifact files (e.g. memory
     notes) are captured in the delta and mirrored to the source working tree — the
     latent mirror-image gap the review calls out.
  3. Lines 631 / 1029: derive `committablePaths` = `paths` minus everything under
     `artifactRel`, and stage/commit/validate only those. The full `paths`/`patch`
     still drive the working-tree apply/merge/rollback, so artifacts reach the
     source working tree but never the index. When `committablePaths` is empty
     (artifact-only batch, e.g. all issues invalid) the commit is skipped entirely.
- Test: `TestReviewIsolationCommitsExactIntegratedBatch` now ignores `.compozy/**`
  and asserts `git ls-files -- .compozy` stays empty while the code fix is
  committed and the resolved issue is present on disk; a new
  `TestReviewIsolationMirrorsNewArtifactFileWithoutCommitting` covers a fresh
  artifact file reaching the source working tree without being committed.
- Notes: change constrained to `review_isolation.go` and its test file per scope.
- Verification: `make fmt`/`make lint` clean (0 issues); `go test -race ./internal/core/worktree/`
  passes (36 tests); adjacent `./internal/core/run/executor/` + `./internal/core/reviews/`
  pass (214); `go build ./...` clean. The 24 failures in `make verify` are all in
  `internal/core/plan` / `internal/daemon` with the pre-existing environmental cause
  `rundb: schema too new (db=4 binary=3)` — a shared `~/.compozy/runs/*/run.db` written
  by a newer binary; unrelated to this worktree-package change (no `plan`/`daemon`/`rundb`
  code touched). Documented and proceeding per cy-fix-reviews.
