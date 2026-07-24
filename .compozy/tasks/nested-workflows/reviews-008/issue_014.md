---
provider: manual
pr:
round: 8
round_created_at: 2026-07-24T19:59:29Z
status: resolved
file: internal/core/taskgroups/readiness.go
line: 43
severity: low
author: claude-code
provider_ref:
---

# Issue 014: Guard validates plan.Edges but the deref uses Dependencies

## Review Comment

Lines 20-39 validate that every `plan.Edges` endpoint exists in the `taskGroups`
map, clearly intending to make the subsequent map lookups safe. But line 43
dereferences a *different* collection:

```go
for _, dependency := range selected.Dependencies {
    if !taskGroups[dependency.From].Completed {
```

`selected.Dependencies` is never checked by the guard, and `taskGroups` holds
`*TaskGroup`, so an ID present in `Dependencies` but absent from `Edges` yields a
nil pointer and panics — violating the project's no-panic-in-production rule,
inside a daemon request path.

Currently unreachable: plans from `ParsePlan` keep the two collections in sync,
and `internal/daemon/transport_mappers.go:173` hand-builds a `taskgroups.Plan`
that stays consistent only because it appends to both `taskGroup.Dependencies`
and `plan.Edges` in the same loop (lines 189-190). Dropping that
visually-redundant `plan.Edges` append — an easy refactor to make — turns a daemon
list request into a panic.

Note line 74 does the same dereference but is genuinely covered by the guard,
since `edge.From` comes from `plan.Edges`.

Fix: iterate `selected.Dependencies` in the guard as well, or use
`taskGroup, ok := taskGroups[dependency.From]` and return `ErrInvalidPlan` when
`!ok`, so the validated surface matches the dereferenced one.

## Triage

- Decision: `VALID`
- Root cause: the guard (readiness.go:20-39) validates only `plan.Edges` endpoints,
  but the direct-dependency loop dereferences `taskGroups[dependency.From].Completed`
  over `selected.Dependencies` — a collection the guard never checks. `taskGroups`
  holds `*TaskGroup`, so an ID present in `Dependencies` but absent from `Edges`
  yields a nil-pointer panic on a daemon request path, violating the project's
  no-panic-in-production rule. As the reviewer notes, this is currently unreachable
  because `ParsePlan` keeps the two collections in sync and
  `transport_mappers.go:189-190` appends to both together, but the invariant is
  implicit and one dropped `plan.Edges` append turns a list request into a panic.
- Fix approach: adopted the reviewer's second option — the direct loop now does
  `prerequisite, exists := taskGroups[dependency.From]` and returns a typed
  `ErrInvalidPlan` (Field `graph.edges`, message `unknown prerequisite %q`,
  matching the existing edges-guard wording) when the target is missing, so the
  validated surface matches the dereferenced one. Line 74's dereference (now inside
  the rewritten `unmetTransitivePaths` BFS) is genuinely covered because it only
  walks `incoming` edges built from the already-validated `plan.Edges`.
- Tests: added `TestReadiness/UT-040` — a plan whose `TaskGroup.Dependencies`
  points at an absent `TG-999` with no matching edge now returns `ErrInvalidPlan`
  (asserted via `assertDomainError` + `assertIssueContains`) instead of panicking.
- Notes: fixed alongside issue 007 in the same file; the two changes are
  independent.
