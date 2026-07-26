# QA Pair Append Checklist

Run before exiting the skill.

## Structural

- [ ] `_tasks.md` table column order is preserved.
- [ ] Both new rows use sequential `task_NN` IDs.
- [ ] No existing rows were modified.
- [ ] No duplicate `qa-report` or `qa-execution` rows.

## Dependency wiring

- [ ] `qa-report` depends on the last implementation task.
- [ ] `qa-execution` depends on `qa-report`.
- [ ] `qa-execution` relies on the last implementation task transitively through `qa-report`; do not duplicate the full implementation dependency chain.
- [ ] No cyclic dependencies introduced.

## Skills & contract

- [ ] `qa-report` task body references the `qa-report` skill with `qa-docs-path=docs/qa`.
- [ ] `qa-execution` task body references `qa-execution` with `qa-docs-path=docs/qa` (and `real-scenario-qa` for release-grade runtime scope).
- [ ] Task frontmatter uses exact phase types: `qa-report` and `qa-execution`.
- [ ] Neither body references per-round `qa/` trees, `qa-output-path`, `TC-*` test cases, or `verification-report.md` — the living-docs contract is `docs/qa/{scenarios/, journeys/, charters/, bugs/BUG-<date>-<slug>.md, reports/<date>-<slug>.md}`; `state.csv` is generated output only.

## Complexity

- [ ] `qa-report`: complexity `high`.
- [ ] `qa-execution`: complexity `critical`.

## E2E directives

- [ ] If UI-bearing: row body cites Playwright + `browser-use:browser` (fallback `agent-browser`).
- [ ] If CLI/API/agent-manageability-bearing only: row body cites daemon E2E + CLI/HTTP/UDS cross-surface comparison.
- [ ] If extensibility/config-bearing: row body cites extension/config lifecycle validation where applicable.
- [ ] If neither: row body documents the no-UI rationale requirement.

## MVP Boundary

- [ ] If a `## MVP Boundary` section names the QA tasks explicitly, the range is updated.
- [ ] Kernel boundary text is unchanged.

## Final

- [ ] No existing review rounds, ADRs, or memory snapshots were touched.
- [ ] Diff is printed to stdout for review.
