# QA Tail Template

Full-loop only: this pair belongs to a requested `cy-loop-tasks` graph, not to every ordinary fix. It owns remaining changed/integration journeys and final visual rows in the living `docs/qa/` tree. Reuse current plans and evidence. A no-work disposition lives in workflow QA memory and does not fabricate a dated run report. Preserve phase types and dependencies; rate complexity by actual risk.

Replace `<risk>` with `low`, `medium`, `high`, or `critical` before saving task files.

## Column order (preserved from `cy-create-tasks` output)

```
| # | Title | Status | Complexity | Dependencies |
```

## qa-report row template

```markdown
| NN | QA Plan and Session Charters | pending | <risk> | <implementation prerequisites> |
```

Body content for the task file (`task_NN.md`):

Wire planning after the implementation graph's terminal prerequisites (only the
last implementation task for a linear chain). Add the QA nodes/edges to the graph
manifest together with its table rows and task files; dependencies do not live in
individual task frontmatter.

- Frontmatter `type: qa-report` (required by the loop phase detector).
- `Read the scope, linked contracts, relevant ADR decisions, and supplied task-memory paths; reuse the existing QA map.`
- Reconcile task evidence and remaining integration/visual obligations first. Use `qa-report` with `qa-docs-path=docs/qa` only for planning gaps; existing plans and evidence can satisfy this task without new artifacts.
- Output: update affected journeys/scenarios/charters only where needed; record selected scope and reused evidence in workflow QA memory. Planning does not need a runtime lab or mandatory worker.
- Coverage: account for changed public behavior across relevant CLI, HTTP/UDS, Web, config, and extension/agent surfaces. Name the remaining walks and visual rows, their dependencies, and the evidence already covering the rest; file presence alone does not create another journey.
- Map regression hot spots from `_spec.md` Part II invariants and ADRs into the cycle's charter selection (targeted tier, adding adjacent journeys where propagation is plausible).

## qa-execution row template

```markdown
| NN | Real-User QA Execution | pending | <risk> | task_<qa_report> |
```

Body content:

- Frontmatter `type: qa-execution` (required by the loop phase detector).
- Read/reuse the selected scenarios, related open bugs, and charters; do not reload the entire QA tree.
- Use `qa-execution` with `qa-docs-path=docs/qa` for the remaining selected walks. Pass the evidence reuse decisions and enclosing workflow's ownership of final PR gates. If no execution remains, record the applicability/coverage disposition in workflow QA memory instead of invoking the skill.
- Use `eng-qa-bootstrap` only when selected execution requires a lab, and its manifest teardown on every exit with `teardown.json` reporting `clean: true`. For explicit release-grade runtime scope, apply `eng-real-scenario-qa`; ordinary targeted QA does not acquire that release workflow.
- For UI features: drive Playwright via `browser-use:browser` with `agent-browser` fallback.
- For CLI/API/agent-manageability features: exercise the affected entry points and changed observable contract. Compare CLI/HTTP/UDS and persisted state when the change can make them disagree.
- Register every reproduced defect in `docs/qa/bugs/BUG-<YYYYMMDD>-<slug>.md` (dedup against the registry first) and link it in the affected scenario files.
- Fixes follow the fix-loop governor: respect existing repair authorization; prove each fix in its owning suite or replay, adding a regression only for a coverage gap. Escalate unresolved product decisions.
- Complete the integration-owned Visual Contract rows once, using `eng-ui-screenshot`; reuse valid row bundles and capture only missing/invalidated states.
- Update scenario verdicts and write/update `docs/qa/reports/<YYYY-MM-DD>-<slug>.md` for actual runs. Re-walk only evidence invalidated by repairs. Cite current local gate evidence; commit/push checks and required CI remain owned by the enclosing workflow, not another QA execution cycle.

## Verification scope variants

When changed UI behavior needs browser evidence:

> Run the relevant existing browser scenario or drive the affected workflow through `browser-use:browser`, with `agent-browser` fallback. Frontend suite commands use root Turborepo with affected-package filters. Add runtime coverage for a changed backend contract the browser walk does not cover. Do not prescribe both full E2E suites merely because UI files changed.

When changed CLI/API/agent behavior needs runtime evidence:

> Run the owning real integration scenario or exercise the affected public entry path against an isolated daemon. Include cross-surface state comparisons when relevant. Expand to the full runtime E2E suite only when the changed contract/risk or project policy requires it.

When no user journey changed:

> Cite or run the owning real integration/contract check for changed backend behavior. Editorial-only scope needs no runtime walk. Record the scope and evidence disposition in workflow memory; do not invent a session report.

## MVP Boundary update

If `_tasks.md` ends with a section like:

```markdown
## MVP Boundary
Tasks 01-16 implement the autonomy kernel. Tasks 17-18 prepare and execute QA.
```

Update the trailing range to include the appended tasks. Do NOT alter the kernel boundary description, only the numbers.
