## TC-PERF-001: Daemon Performance Regression Guard

**Priority:** P1 (High)
**Type:** Performance
**Status:** Not Run
**Estimated Time:** 15 minutes
**Created:** 2026-04-18
**Last Updated:** 2026-04-18
**Automation Target:** Integration
**Automation Status:** Existing
**Automation Command/Spec:**
- `go test ./internal/store/rundb ./internal/daemon -run '^$' -bench 'BenchmarkRunDBListEventsFromCursor|BenchmarkRunManagerListWorkspaceRuns' -benchmem -count=1`
- Optional adjunct when available: `hyperfine --warmup 3 --runs 10 './bin/compozy --help > /dev/null' './bin/compozy --version > /dev/null' './bin/compozy completion bash > /dev/null'`
**Automation Notes:** The benchmark command is the required automation lane. The `hyperfine` timing check is a best-effort adjunct for CLI cold-start-sensitive paths and should be recorded as environment-limited if the tool is unavailable.

### Objective

Detect silent regressions in the performance-sensitive daemon seams fixed by `task_16`, especially event pagination and run listing, while keeping CLI cold-start checks available when the local toolchain supports them.

### Preconditions

- [ ] Benchmark execution is acceptable in the current environment.
- [ ] The task-16 performance ledger or recorded baseline is available for comparison.
- [ ] `hyperfine` is optional; do not fail the core suite if it is missing.

### Test Steps

1. Run the focused daemon benchmark suite listed above.
   **Expected:** The benchmark command exits `0` and produces current `ns/op`, `B/op`, and `allocs/op` data for the guarded hot paths.

2. Compare the benchmark output with the task-16 baseline.
   **Expected:** No material regression is observed without an explicit explanation; any notable slowdown triggers targeted investigation before release sign-off.

3. If `hyperfine` is available, run the CLI timing adjunct.
   **Expected:** `--help`, `--version`, and `completion bash` remain within the expected cold-start envelope relative to the task-16 baseline.

### Edge Cases & Variations

| Variation | Input | Expected Result |
|---|---|---|
| Benchmark-only environment | `hyperfine` missing | Core benchmark lane still runs and records evidence |
| Larger event history | High-event run fixture | Pagination benchmark remains stable enough to avoid obvious regressions |
| CLI cold-start spot check | `hyperfine` available | Help/version/completion timing remains aligned with the perf baseline |

### Related Test Cases

- `TC-FUNC-002`
- `TC-INT-003`

### Traceability

- TechSpec Known Risks: per-run storage and snapshot growth on large workflows.
- Task reference: `task_16`.
- Supporting baseline: `.codex/ledger/2026-04-18-MEMORY-daemon-perf.md`.

### Notes

- This case is not a substitute for `make verify`. It is a targeted regression guard for the perf-sensitive daemon seams that functional tests may miss.
