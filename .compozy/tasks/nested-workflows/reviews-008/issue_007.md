---
provider: manual
pr:
round: 8
round_created_at: 2026-07-24T19:59:29Z
status: resolved
file: internal/core/taskgroups/readiness.go
line: 57
severity: medium
author: claude-code
provider_ref:
---

# Issue 007: unmetTransitivePaths enumerates every simple path

## Review Comment

`unmetTransitivePaths` walks `incoming` edges with only an ancestor-cycle guard
(line 69) — no memoization, no visited set, no bound. It materializes *every*
simple path, and each recursion allocates `slices.Clone(edges)`,
`slices.Clone(ids)`, and a fresh copy of the `ancestors` map (lines 72-85).

Time and memory are therefore exponential in graph density. For a plan where TG-k
depends on every TG-j (j<k) — a shape LLM-generated plans produce when they add
redundant "explicit" transitive edges — the recorded path count is
`2^(N-1) - N`: N=20 gives ~524k paths, N=25 gives ~16.7M.

This runs on a daemon request path: `projectTaskGroupReadiness`
(`internal/daemon/transport_mappers.go:170`) calls it once per child from
`query_service.go:167` and `task_transport_service.go:146`, multiplying by N, and
then copies every path into the API response. A 20-group dense plan wedges the
daemon for seconds per list call; ~25 groups exhausts memory. `_task_groups.md` is
user/agent-authored with no node or edge cap.

The enumeration is also pure waste for the main consumer:
`unmetDependencyBlockers` (`internal/core/taskgroups/set_validation.go:105`)
immediately flattens all paths into a *set*.

Fix: compute the unmet-ancestor set with a BFS over reverse edges (O(V+E)) for
`Eligible` and for blockers, and derive at most one representative path per
blocker for display. Add a test with a dense 25-group plan asserting the call
completes in bounded time.

## Triage

- Decision: `VALID`
- Root cause: `unmetTransitivePaths` did a DFS that materialized *every* simple
  reverse path, cloning `edges`, `ids`, and the `ancestors` map at each step
  (readiness.go:72-85). For a plan where TG-k depends on every TG-j (j<k) the
  recorded-path count is `2^(N-1)-N`, so the daemon list path (`transport_mappers.go`
  → `EvaluateReadiness` per child) is exponential in graph density. `ParsePlan`
  imposes no node/edge cap and accepts redundant transitive edges (only cycles and
  duplicate edges are rejected), so the blowup is reachable from user/agent-authored
  `_task_groups.md`, not just hand-built plans.
- Fix approach: replaced the DFS with a breadth-first walk over the reverse
  (`incoming`) edges — O(V+E). Each reachable node is visited once; a single
  representative shortest path is reconstructed via parent links for every
  incomplete ancestor reached at depth >= 2 (direct prerequisites stay in
  `DirectUnmet`). This preserves the two observable contracts exactly:
  - `Eligible = len(direct)==0 && len(transitive)==0` — when `direct` is empty all
    depth-1 nodes are complete, so "any incomplete ancestor reachable at depth >= 2"
    ⟺ "any incomplete reachable ancestor", matching the old verdict.
  - `unmetDependencyBlockers` (set_validation.go) unions every incomplete ID across
    paths; the BFS still surfaces every incomplete reverse-reachable ancestor
    (direct via `DirectUnmet`, transitive as a path endpoint), so the blocker set is
    identical. The redundant duplicate paths it drops were already flattened into
    that set, i.e. pure waste.
  The `DependencyPath` shape (TaskGroupIDs deepest-ancestor→direct-prereq; Edges
  connecting consecutive pairs, excluding the final edge into the selected group) is
  byte-for-byte preserved — verified against `TestReadiness/UT-016` and daemon
  `IT-055` (`transport_service_test.go`), both of which pass unchanged.
- Tests: added `TestReadiness/UT-038` (diamond → one representative path per
  blocker, blocker set intact) and `UT-039` (fully-connected 25-group plan
  completes in bounded time — the exact shape that previously enumerated ~16.7M
  paths; the test finishing is the regression guard).
- Notes: consumers (`tasks_run_wizard_status.go`, `task_group_completion.go`,
  `daemon/transport_mappers.go`, `daemon_commands.go`) either union path IDs into a
  set or render one line per path, so fewer/deduplicated paths only shrink display
  output; none lose information.
