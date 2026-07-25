---
provider: manual
pr:
round: 9
round_created_at: 2026-07-24T20:14:29Z
status: resolved
file: internal/core/taskgroups/plan.go
line: 348
severity: medium
author: claude-code
provider_ref:
---

# Issue 002: Root groups accept malformed dependency declarations

## Review Comment

The Markdown parser records dependencies only when it sees a
`- Dependencies:` prefix, and `markdownDependencies` silently returns an empty
slice for every unrecognized value or malformed list row. The subsequent
`validateMarkdownTaskGroup` checks title, reference, outcome, and ownership, but
does not record whether the dependency field was present or syntactically
valid.

For a YAML root node with no incoming edges, all of these Markdown bodies
therefore validate as equivalent to the required `- Dependencies: None`:

- no Dependencies section;
- `- Dependencies: garbage`;
- an empty `- Dependencies:` block;
- a block containing only malformed dependency rows.

`validatePlanSurfaces` compares two empty slices at line 461 and accepts the
plan. This defeats the dual-surface contract that every task group must declare
either `None` or the canonical dependency list, allowing corrupted or
hand-edited plans to pass validation and reach readiness/execution.

Track dependency-field presence and parse diagnostics separately from the
parsed edge slice. Require exactly the `None` form or at least one fully matching
dependency row, and reject malformed or unexpected content rather than ignoring
it. Add table-driven tests for the missing, inline-garbage, empty-block, and
malformed-row cases on a zero-incoming-edge group.

## Triage

- Decision: `VALID`
- Notes: `parseMarkdownTaskGroupFields` records only successfully parsed dependency
  rows. It does not preserve whether the `Dependencies` field appeared, and
  `markdownDependencies` discards invalid inline values and malformed block
  rows. For a root task group, each malformed form therefore produces the same
  empty slice as the YAML graph's zero incoming edges, so
  `validatePlanSurfaces` accepts it. The fix will preserve dependency-field
  presence and parse diagnostics independently of parsed edges, require the
  canonical `None` form or a non-empty block of canonical rows, and surface
  malformed content as `body.<TG-ID>.dependencies` validation issues. The
  existing `TestValidatePlan` suite owns the regression cases because the
  invariant is plan-parser validation at the package boundary.
- Verification: the in-scope package passes `go test -race
  ./internal/core/taskgroups` (98 tests), and its exact golangci-lint invocation
  reports zero issues. Full `make verify` remains blocked by unchanged,
  out-of-scope baseline failures: `internal/cli/daemon_commands.go` exceeds the
  configured cyclomatic-complexity limit, and a subsequent run timed out in
  `web/src/systems/runs/components/run-detail-view.test.tsx`, causing five web
  test failures. `git diff --exit-code HEAD` confirms both files are unchanged
  in this batch.
