# BUG-20260825-playbook-changelog-task-unsourced: The release-week scenario asks for a changelog it gives no source for

- **Status:** open <!-- open | fixed | verified | wont-fix | invalid -->
- **Impact (user-side):** Friction
- **Severity:** Low · **Priority:** P3
- **Persona Affected:** Ada
- **Journey Step:** real-scenario runtime lane (`devtool-oss-launch`), task `task-devtool-oss-launch-005`
- **Scenarios:** (none — QA harness defect, not a product scenario)
- **Found:** 2026-08-25 · **Report:** docs/qa/reports/2026-08-25-skill-sources.md

## Summary

Filed because the observer reported a stall and the skill requires a registry bug naming the silent
agent and the stalled task. On inspection the agent was not silent and the runtime did not stall in
the way the label suggests: `docs-engineer-agent` picked up "v1.0 changelog spec", found it could not
satisfy the brief honestly, and said so.

The `devtool-oss-launch` playbook seeds `changelog-style.md` with "Cite the issue or PR id for every
entry", then gives the workspace no repository, no git history, no source tree, and no issue or PR
list in any knowledge file. The agent's recorded failure reason is exactly that:

> No source-of-truth for PR/issue citations exists in this workspace (no git history, no source tree,
> no issue tracker access, no PR list in any knowledge file …)

Refusing to invent PR numbers is the correct behavior — the alternative would have been a changelog
full of fabricated citations. The defect is in the playbook, which asks for an artifact it makes
unsatisfiable.

## Reproduction

- **Environment:** isolated lab `compozy-devtool-oss-launch-20260825-165802-509267-lab`, build `80f17b536`, one in-persona kickoff, 1800s observation window

1. Bootstrap with `--playbook devtool-oss-launch`; register the 8 agents, 5 channels, 11 tasks.
2. Post the single operator kickoff and release dispatch.
3. Observe: `compozy task list` reaches 10 `completed`, 1 `ready`.
4. `compozy task inspect task-devtool-oss-launch-005 -o json`.

**Expected:** every declared deliverable is producible from what the scenario provides.
**Actual:** 10 of 11 tasks completed and produced real artifacts (Go service stub that compiles, two
benchmark scripts, a shell release script, a TSX landing page, a TSX thread component, a TS contract
test, two runbooks, a decision spec). Task 005's run finished `failed` after `attempt 1` of
`max_attempts 3` with the reason above, and the task returned to `ready`.

## Evidence

- `<lab>/qa-artifacts/qa/observation-summary.json` — `outcome: stall`, `progress_transitions: 19`
- `<lab>/qa-artifacts/qa/task-activation.json` — 11 runs queued behind the barrier, released after kickoff
- Deliverables under `<lab>/project/workspaces/` — 21 non-knowledge files across all five workspaces,
  including a compiled `release-control` binary
- `compozy task inspect task-devtool-oss-launch-005 -o json` — the recorded failure reason

## Open question for the runtime (not resolved here)

The run failed at `attempt 1` of `max_attempts 3` and the task went back to `ready`, but no further
attempt was observed in the remaining ~25 minutes of the window. Whether a failed run is expected to
be re-dispatched automatically or to wait for an operator is a runtime-contract question outside this
cycle's scope; it is recorded so a later cycle can settle it rather than inheriting the ambiguity.

## Confound disclosed

The lab daemon was restarted twice during the observation window while fixing
BUG-20260825-skill-detail-rejects-workspace-id. The restarts cannot explain this particular finding —
the run's failure reason is a content judgement recorded at 17:40, before the first restart — but they
do mean the window was not pristine, and any timing-sensitive conclusion from this run should be
re-taken on a clean pass.

## Fix

- **Suggested direction:** seed the playbook with a citable source — a short `knowledge/global/pr-index.md`
  listing the v1.0 PR/issue ids, or a seeded git history in `ws_docs_site` — so the changelog task is
  satisfiable without invention. Alternatively relax `changelog-style.md` for scenarios with no
  repository. This is a change to
  `.agents/skills/eng/eng-real-scenario-qa/references/playbooks/devtool-oss-launch.md`, and it needs a
  fresh scenario run to verify, which this pass did not perform.
- **Fix commit:**
- **Regression test:**
