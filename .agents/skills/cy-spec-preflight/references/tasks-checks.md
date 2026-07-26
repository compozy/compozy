# `_tasks.md` Preflight Checks

Run after `cy-create-tasks` produces a draft, before handing off to execution.

## Table shape

- [ ] Column order matches the canonical `cy-create-tasks` output: `# | Title | Status | Complexity | Dependencies`.
- [ ] Sequential `task_NN` IDs, gap-free.
- [ ] Displayed row numbers are sequential and match individual `task_NN.md` file names.
- [ ] No empty cells. `Dependencies: -` is allowed; blank cells are not.

## Boundary

- [ ] An `## MVP Boundary` section above the table names which numbered tasks are MVP, what is post-MVP, what is out of scope.

## Per-row directives

- [ ] **Dependencies** populated for every row (`task_NN, task_MM` or `-`).
- [ ] **Complexity** rated `low | medium | high | critical`. Critical reserved for safety primitives + final QA execution.
- [ ] **Status** starts as `pending` unless enriching an existing task tree with known completed work.
- [ ] **Skills** are named in each task body, not in the master table, when a task requires explicit skill activation.
- [ ] **Web/Docs Impact** subsection exists in every backend task body, even when "none" (run `cy-web-docs-impact` to populate).
- [ ] **QA impact** line exists in every task body that changes user-visible behavior: names the `docs/qa/scenarios/*.md` ids to reset to `untested` (or new content-addressed files to add), or states `none — no user-visible behavior change`.
- [ ] **Extensibility / Agent Manageability / Config Lifecycle** subsection exists in every feature-bearing backend task body, even when "none with evidence".

## Trailing QA pair

- [ ] Last two rows: `qa-report` (high) and `qa-execution` (critical), per the `cy-tasks-tail-qa-pair` tail template (living `docs/qa/` contract).
- [ ] `qa-report` depends on the last implementation task.
- [ ] `qa-execution` depends on `qa-report` and relies on implementation completion transitively.
- [ ] UI-bearing features: `qa-execution` body cites Playwright via `browser-use:browser` (fallback `agent-browser`).
- [ ] CLI/API features: `qa-execution` body cites `make test-e2e-runtime` and CLI/HTTP cross-surface comparison.
- [ ] Activate `cy-tasks-tail-qa-pair` to enforce the shape.

## Test density

- [ ] Per-task test plan is proportional to behaviors documented in the TechSpec section it implements.
- [ ] Reject "fraco" plans: 1-2 tests for many behaviors.
- [ ] Critical-complexity tasks list happy + failure-path + concurrency-stress + contract/redaction cases.
- [ ] CLI/HTTP/UDS changes include agent-operability tests: structured output, status/config discovery, deterministic errors, and cross-surface state comparison when applicable.
- [ ] Config changes include merge/overlay, default, validation, docs/example, and restart/reload tests where applicable.
- [ ] Test plan cites `agh-test-conventions` for shape, `agh-cleanup-failure-paths` for cleanup audit, `agh-schema-migration` for migrations, `agh-contract-codegen-coship` for contract changes.

## Competitor refs

- [ ] When the TechSpec drew on `.resources/<repo>`, each task body lists exact file paths so the implementer reads them.
- [ ] Reference paths stay relative to `.resources/` and are not paraphrased.

## Hygiene

- [ ] No TBD / placeholder rows.
- [ ] Status reconciled (no `task_03 pending` while `task_10 completed`).
- [ ] No cycles in the dependency graph.

## Validation

If any directive fails, surface the failing rows and direct the author to fix the table before execution. Do not start tasks against a broken `_tasks.md`.
