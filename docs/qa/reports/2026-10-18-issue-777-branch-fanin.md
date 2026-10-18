# QA Run Report — 2026-10-18 — Branch fan-in

- **Scope:** Clarify branch fan-in and validate an explicit all-conditions guard before downstream work.
- **Cadence tier:** targeted
- **Build:** Installed CompozyOS 0.3.0-beta.35; source baseline `07aba5e6ee52b906b8bf78e847728c71cc003b8e`.
- **Environment:** Isolated daemon/home, local HTTP port and UDS; mechanical transforms only, no providers.
- **Status:** in-progress

## Session Matrix & Results

| Journey / Scenario | Persona | Tour | Status |
|---|---|---|---|
| J-complete-partial-loop / branch fan-in | Bruno (CLI operator, macOS, local network, en-US) | Feature Tour | Pass |
| Explicit AND guard: false and true conditions | Bruno | Feature Tour | Pass |

## Plan

Reproduce the issue graph through `compozy loop run`, read terminal status through CLI/UDS and independently through HTTP. Then run an explicit AND guard with one false input and with both inputs true. Preserve the existing alternative-path join contract; no database reads or host daemon changes.

## Session Debriefs

The issue YAML reproduced unchanged at the graph level: `false_branch` returned `branch:false`,
`false_path` had `branch_skipped`, `true_path` returned `true-path`, and `join` returned `joined`.
The run settled `done` in generation 1. This agrees with the existing coordinator contract:
a shared join stays live when an independent incoming path remains live.

The complete `guarded-delivery` example was published and run twice. With `security_ok=true` and
`approval_ok=false`, `publish` had `branch_skipped` and no `task_run_id`. With both true it returned
`{"published":true}` with an action task run. Both runs settled `done` in generation 1. Fresh HTTP
run-detail reads matched every CLI/UDS generation output exactly.

Local evidence: the `issue777-fanin` bootstrap manifest and `qa/{status,http}-{fanin,guard-false,guard-true}.json`
under this run's isolated lab. The compact observed results above are retained here so public
readers do not depend on private host paths. No providers, external writes, or approvals were used.

Authoring diagnostics during setup: the initial wrapper used lowercase `loop` instead of `Loop`;
the first guard omitted its UDS start binding; CAS publication then required `meta.version` to match
`expected_version`. All were rejected explicitly and corrected through ordinary authoring. They
were setup input errors, not successful runs or bypasses.

No new runtime defect or paper cut was observed in the bounded journey. The documentation gap was
the missing explanation that skipped paths satisfy dependencies without proving execution.

## Verification

- `CGO_ENABLED=1 GOMAXPROCS=2 go test -p 2 -race ./internal/loop -run '^TestCoordinatorRunnerShouldRouteBranchCondition$' -count=1` — pass, including the existing live-join case and all four AND-guard combinations.
- Site content generation and scoped `turbo run typecheck --filter=./packages/site --only` — pass after installing locked worktree dependencies; the sandbox reported cache-write warnings. The initial typecheck dependency graph entered the codegen gate unexpectedly and was canceled; `--only` then ran just the requested typecheck. No broad gate result is claimed.
- The read-only Go test convention checker flags three pre-existing top-level cases in this file, outside the changed branch suite. The new cases follow the existing `Should` subtest convention and run in parallel.
- Full local gates, broad Go tests, browser E2E and desktop builds are excluded by the assignment's machine-resource override. Required PR CI remains the delivery gate.

## Compatibility and Scope

No runtime semantics, schemas, public IDs, configuration, hooks or workspace isolation changed.
The fix documents existing fan-in and a supported AND guard; it does not change existing Loop
execution or require data migration. The live run used the installed beta.35 binary; current source
behavior is covered by the focused coordinator suite. Linux and rendered Web behavior were not
retested locally.

## Final Status

The three CLI/HTTP journey legs pass. PR readiness awaits current-head CI and both external
reviewers. The strict lab audit has only C14 outstanding: it demands local gate evidence, which
the assignment explicitly prohibits in favor of PR CI. Every behavioral evidence check passed;
the audit itself is not reported as passing. The isolated lab was torn down with `clean: true`
in `qa/teardown.json`; no owned processes remain.
