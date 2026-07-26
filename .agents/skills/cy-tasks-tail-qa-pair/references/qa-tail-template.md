# QA Tail Template

Canonical row shape for the QA pair. Mirror this exactly when appending to a fresh `_tasks.md`. The pair operates on the repo's living QA tree (`docs/qa/` — scenario files, journeys, charters, content-addressed bugs, dated reports), never on per-round `qa/` trees.

## Column order (preserved from `cy-create-tasks` output)

```
| # | Title | Status | Complexity | Dependencies |
```

## qa-report row template

```markdown
| NN | QA Plan and Session Charters | pending | high | task_<last_impl> |
```

Body content for the task file (`task_NN.md`):

- Frontmatter `type: qa-report` (required by the loop phase detector).
- `<critical>ALWAYS READ _techspec.md, every ADR, and every per-task memory file before planning.</critical>`
- Activate the `qa-report` skill with `qa-docs-path=docs/qa` (bootstrap the tree if absent).
- Output: journey flowcharts updated in `docs/qa/journeys/`, scenario files minted/updated in `docs/qa/scenarios/`, session charters in `docs/qa/charters/` for this cycle.
- Coverage: every public surface touched by tasks 01..N — CLI verbs, HTTP, UDS, web routes, doc pages, automation triggers, extension points, agent-operation paths, and `config.toml` keys — expressed as scenario `entry_points` on journey-derived rows, not as standalone test cases.
- Map regression hot spots from `_techspec.md` invariants and ADRs into the cycle's charter selection (targeted tier + one adjacent canary journey).

## qa-execution row template

```markdown
| NN | Real-User QA Execution | pending | critical | task_<qa_report> |
```

Body content:

- Frontmatter `type: qa-execution` (required by the loop phase detector).
- `<critical>ALWAYS READ the in-scope docs/qa/scenarios/ files, open docs/qa/bugs/, and the cycle's charters in docs/qa/charters/ before executing.</critical>`
- Activate `qa-execution` with `qa-docs-path=docs/qa`. For release-grade scope on the AGH runtime, also activate `real-scenario-qa` (playbook lab + operator kickoff + runtime observation).
- Activate `agh-worktree-isolation` (unique `AGH_HOME` + ports + tmux socket) when concurrency is signaled.
- For UI features: drive Playwright via `browser-use:browser` with `agent-browser` fallback.
- For CLI/API/agent-manageability features: exercise structured CLI output, HTTP/UDS routes, status/config discovery, deterministic errors, and compare persisted state.
- Register every reproduced defect in `docs/qa/bugs/BUG-<YYYYMMDD>-<slug>.md` (dedup against the registry first) and link it in the affected scenario files.
- Fixes follow the fix-loop governor: small/contained only, regression test red-before/green-after, one logical fix per commit; escalate the rest to "Decisions for a Human".
- Update scenario-file verdicts and write the dated run report at `docs/qa/reports/<YYYY-MM-DD>-<slug>.md`. Exit gate: re-run gates (`make verify`) before Final Status.

## E2E directive variants

When `requires_e2e=true`:

> Run `make test-e2e-runtime` (daemon harness) AND `make test-e2e-web` (Playwright). Drive the highest-risk UI workflow through `browser-use:browser`; fall back to `agent-browser` only if `browser-use:browser` is unavailable. Do not silently substitute shell-only checks.

When `requires_cli_e2e=true` and `requires_e2e=false`:

> Run `make test-e2e-runtime` and exercise the affected CLI verbs, HTTP/UDS routes, agent-operation paths, and config lifecycle end-to-end against a daemon-served runtime (unique `AGH_HOME`). Compare structured CLI output with HTTP/UDS responses for the same persisted state.

When neither is true (rare backend-only):

> Run `make test-integration` and a smoke `make test-e2e-runtime` even if no UI changed. Document the no-UI rationale in the run report under `docs/qa/reports/`.

## MVP Boundary update

If `_tasks.md` ends with a section like:

```markdown
## MVP Boundary
Tasks 01-16 implement the autonomy kernel. Tasks 17-18 prepare and execute QA.
```

Update the trailing range to include the appended tasks. Do NOT alter the kernel boundary description, only the numbers.
