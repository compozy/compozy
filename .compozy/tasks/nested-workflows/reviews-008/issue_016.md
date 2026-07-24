---
provider: manual
pr:
round: 8
round_created_at: 2026-07-24T19:59:29Z
status: resolved
file: internal/core/taskgroups/completion.go
line: 109
severity: low
author: claude-code
provider_ref:
---

# Issue 016: Completion heading scan includes YAML frontmatter

## Review Comment

`rewriteCompletion` runs `completionHeadingPattern` — a `(?m)` line-anchored
regex — over the whole file, while `parseMarkdownTaskGroups`
(`internal/core/taskgroups/plan.go:293`) only scans the post-frontmatter `body`
returned by `frontmatter.Parse`. The two disagree on any heading-shaped line
inside the frontmatter.

Failure: an edge rationale authored as a YAML block scalar (`rationale: |`) whose
continuation line begins `## [ ] TG-002 — needs schema`. `ParsePlan` accepts the
plan, because that line never reaches the body and there is exactly one body
heading. But `rewriteCompletion` sees `len(selected) == 2` and returns
`ErrCompletionConflict` with "must contain exactly one compatible task group
heading", pointing at `body.TG-002`, which is fine.

`MarkComplete` for TG-002 then fails permanently, and `HydrateCompletion` aborts
the entire hydration batch with a misleading diagnostic — the `headingMatches == 0`
skip path at line 281 does not apply, because the count is 2, not 0.

Fix: split the frontmatter first and match against the body only, offsetting the
match indices back into `content` before flipping the checkbox byte, so both
readers agree on what counts as a heading.

## Triage

- Decision: `VALID`
- Root cause: `rewriteCompletion` (`completion.go:109`) ran
  `completionHeadingPattern.FindAllSubmatchIndex` over the entire file, while
  `parseMarkdownTaskGroups` (`plan.go:293`) only scans the post-frontmatter
  `body` from `frontmatter.Parse`. Any heading-shaped line inside the
  frontmatter is therefore counted by the completion rewriter but invisible to
  the plan validator, so the two readers disagree on the heading count.
- Reachability: the issue's literal example (a `rationale: |` block-scalar
  continuation line) does not actually reproduce — YAML block-scalar content is
  indented, so `(?m)^## ` never anchors to it. The defect is nonetheless real
  and reachable via a column-0 heading-shaped **YAML comment**, e.g. a line
  `## [ ] TG-001 — …` in the frontmatter. YAML treats a leading `#` as a
  comment (parses cleanly, invisible to the body), yet the regex matches it.
  Confirmed with two failing regression tests: `ParsePlan` accepts the plan,
  but `RewriteCompletion`/`HydrateCompletion` returned `ErrCompletionConflict`
  because `len(selected) == 2`. Because the count is 2 (not 0), the
  `headingMatches == 0` skip path at `completion.go` did not apply, so
  `HydrateCompletion` aborted the whole batch — matching the reported failure.
- Fix: split the frontmatter first and scan only the body. Added
  `completionBodyOffset`, which mirrors `frontmatter.splitContent` to locate the
  byte just after the closing `---` delimiter, then run the heading regex on
  `content[bodyStart:]` and offset the matched checkbox index back into
  `content` before flipping the byte. When there is no complete frontmatter
  block the offset is 0 (whole-content scan preserved) so `ParsePlan` still
  surfaces the real error. The returned `headingMatches` count is now
  body-only, so hydration again distinguishes an absent heading (0, skip) from a
  genuinely ambiguous one (>1, abort).
- Tests: added `TestCompletion/Should ignore a heading-shaped line inside YAML
  frontmatter` and `TestHydrateCompletion/Should hydrate despite a heading-shaped
  frontmatter comment` (plus a shared `withFrontmatterHeadingDecoy` helper). Both
  fail before the fix and pass after; full `taskgroups` package passes under
  `-race`.
- Verification: `internal/core/taskgroups` = 90/90 under `-race`; `golangci-lint
  run ./internal/core/taskgroups/...` = 0 issues; `go build ./...` = exit 0.
  `make verify` reported 25 test failures, all outside this batch's scope and
  environmental/pre-existing, not caused by this change:
  - 24 failures in `internal/core/plan` and `internal/daemon` are identical
    `rundb: schema too new (db=4 binary=3)`. These tests open the shared global
    run journal at fixed `~/.compozy/runs/<name>/run.db` paths (not `t.TempDir()`).
    A newer sibling-worktree binary already migrated those DBs to
    `schema_migrations` version 4, while this checkout's `internal/store/rundb`
    only defines migrations up to version 3, so it correctly refuses to open
    them. Confirmed by inspecting the stale DB (`MAX(version)=4`) vs this
    checkout's migrations (max `version: 3`).
  - 1 failure, `internal/core/subprocess/TestShutdownEscalatesFromSIGTERMToSIGKILL`
    ("timed out waiting for SIGTERM marker"), is a load-induced flake in a
    real-process signal-escalation test; it passes in isolation
    (`go test -run TestShutdownEscalatesFromSIGTERMToSIGKILL -count=1` = pass).
