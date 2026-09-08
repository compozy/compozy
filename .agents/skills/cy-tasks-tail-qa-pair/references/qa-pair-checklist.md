# QA Pair Append Checklist

Use only when diagnosing an existing pair or uncertain graph/coverage.
Normal authoring checks the changed contract once within the owning step.

## Structural

- [ ] `_tasks.md` table column order is preserved.
- [ ] Both new rows use sequential `task_NN` IDs.
- [ ] Customized QA bodies and unrelated rows were preserved; any repair addresses a concrete gap.
- [ ] No duplicate `qa-report` or `qa-execution` rows.

## Dependency wiring

- [ ] `qa-report` follows all implementation prerequisites (the last task suffices for a linear chain).
- [ ] `qa-execution` depends on `qa-report`.
- [ ] `qa-execution` reaches implementation prerequisites transitively through `qa-report`; no redundant dependency chain is duplicated.
- [ ] Graph nodes/edges, display table, and QA files agree.
- [ ] No cyclic dependencies introduced.

## Skills & contract

- [ ] `qa-report` task body references the `qa-report` skill with `qa-docs-path=docs/qa`.
- [ ] `qa-execution` task body references `qa-execution` with `qa-docs-path=docs/qa` (and `eng-real-scenario-qa` for release-grade runtime scope).
- [ ] Task frontmatter uses exact phase types: `qa-report` and `qa-execution`.
- [ ] Neither body references per-round `qa/` trees, `qa-output-path`, `TC-*` test cases, or `verification-report.md` — the living-docs contract is `docs/qa/{scenarios/, journeys/, charters/, bugs/BUG-<date>-<slug>.md, reports/<date>-<slug>.md}`; `state.csv` is generated output only.

## Complexity

- [ ] Both complexity values reflect actual risk; QA task types do not mandate `high`/`critical`.

## Verification ownership

- [ ] The pair names remaining changed/integration journeys and visual rows, with valid task evidence reused.
- [ ] If browser evidence is needed: body names the affected real scenario/flow and browser driver.
- [ ] If CLI/API/agent evidence is needed: body names the affected real integration scenario/entry path; cross-surface comparisons follow actual risk.
- [ ] If extensibility/config-bearing: row body cites extension/config lifecycle validation where applicable.
- [ ] Full suites/labs have a scope, risk, or policy reason; UI paths alone do not trigger them.
- [ ] No-work/reuse closure cites evidence in workflow memory without inventing scenarios, sessions, or passing runs.

## MVP Boundary

- [ ] If a `## MVP Boundary` section names the QA tasks explicitly, the range is updated.
- [ ] Kernel boundary text is unchanged.

## Final

- [ ] No existing review rounds, ADRs, or memory snapshots were touched.
- [ ] The result identifies added/repaired tasks or confirms the existing pair is valid.
