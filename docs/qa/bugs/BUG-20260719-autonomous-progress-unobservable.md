# BUG-20260719-autonomous-progress-unobservable: One-kickoff progress appears stalled while agents complete work

- **Status:** open
- **Impact (user-side):** Blocks-Completion
- **Severity:** High · **Priority:** P1
- **Persona Affected:** QA operator; registered collaborator agents
- **Journey Step:** RT-073 one-kickoff autonomous collaboration, runtime observation
- **Scenarios:** RT-073; LP-run-read-agent-journey; LP-runs-roster-server-ordering; LP-web-run-default-read-briefing; LP-web-run-operator-register
- **Found:** 2026-07-19 · **Report:** docs/qa/reports/2026-07-19-hermes-comparison.md
- **Origin:** n/a

## Summary

After Priya's single kickoff, the registered team completed 10 of the 11 declared tasks, but the
runtime journey log never recorded task completion, agent activity, review, or channel progress.
The release observer therefore reported every task as unstarted and every task-owning agent as
silent. An operator cannot use the required observation surface to distinguish real autonomous
progress from a stalled team.

## Reproduction

- **Charter:** consumer-saas-growth behavioral scenario · **Tour:** autonomous collaboration
- **Environment:** isolated `hermes-comparison-consumer-saas-growth-20260719-190252-199062` lab;
  desktop Web at `127.0.0.1:61528`; isolated daemon at `127.0.0.1:61527`; one confirmed provider
  kickoff and no follow-up provider prompt.

1. Materialize the `consumer-saas-growth` playbook and create its seven agents, four channels, and
   eleven declared tasks.
2. Queue the eleven runs behind the scheduler barrier, deliver the Head of Growth kickoff once,
   and release dispatch.
3. Run `observe-runtime.py` for 1,800 seconds with a 300-second stall threshold while agents work.
4. After the window, compare `qa/observation-summary.json` with
   `compozy task list --workspace lumen-notes -o toon` and the exact task-run lists.

**Expected:** Runtime-owned task, session, and Network progress keeps the journey log growing so the
observer identifies the actual silent agent or stalled task, if any.

**Actual:** The journey log stopped at 14 bootstrap/controller rows. The observer marked all 11
declared tasks unstarted and six task-owning agents silent, while the independent public CLI showed
10 declared tasks completed and one task returned to ready after a failed run.

## Evidence

- Observation summary:
  `/Users/pedronauck/dev/qa-labs/compozy-hermes-comparison-consumer-saas-growth-20260719-190252-199062-lab/qa-artifacts/qa/observation-summary.json`
- Journey log:
  `/Users/pedronauck/dev/qa-labs/compozy-hermes-comparison-consumer-saas-growth-20260719-190252-199062-lab/qa-artifacts/qa/journey-log.jsonl`
- Independent CLI comparison:
  `/Users/pedronauck/dev/qa-labs/compozy-hermes-comparison-consumer-saas-growth-20260719-190252-199062-lab/qa-artifacts/qa/notes/autonomous-progress-observer-mismatch.md`
- Task activation and one-kickoff evidence:
  `/Users/pedronauck/dev/qa-labs/compozy-hermes-comparison-consumer-saas-growth-20260719-190252-199062-lab/qa-artifacts/qa/task-activation.json`;
  `/Users/pedronauck/dev/qa-labs/compozy-hermes-comparison-consumer-saas-growth-20260719-190252-199062-lab/qa-artifacts/qa/operator-kickoff.jsonl`
- Fresh attempt-2 reproduction:
  `/Users/pedronauck/dev/qa-labs/compozy-hermes-comparison-consumer-saas-growth-r2-20260719-202601-971723-lab/qa-artifacts/qa/observer-result.json`;
  the observer again reported ten declared tasks as unstarted and all eleven runs as lacking
  completion, while the public Task catalog showed eight completed tasks and three ready tasks.
- Attempt-2 journey/public-surface evidence:
  `/Users/pedronauck/dev/qa-labs/compozy-hermes-comparison-consumer-saas-growth-r2-20260719-202601-971723-lab/qa-artifacts/qa/journey-log.jsonl`;
  `/Users/pedronauck/dev/qa-labs/compozy-hermes-comparison-consumer-saas-growth-r2-20260719-202601-971723-lab/qa-artifacts/qa/web-task-catalog.png`.
- Compozy migration beta reproduction (2026-07-27):
  `/Users/pedronauck/dev/qa-labs/compozy-compozy-migration-beta-20260727-135201-116083-lab/qa-artifacts/qa/observation-summary.json`;
  the observer reported all eleven seeded runs as lacking completion while independent CLI, HTTP,
  Web, Task, session, and Network reads proved seven Tasks completed, three documentation artifacts
  were authored and approved, and the benchmark Task was blocked on a confirmed regression.
- Migration-beta independent/runtime evidence:
  `/Users/pedronauck/dev/qa-labs/compozy-compozy-migration-beta-20260727-135201-116083-lab/qa-artifacts/qa/task-list-progress-2.json`;
  `/Users/pedronauck/dev/qa-labs/compozy-compozy-migration-beta-20260727-135201-116083-lab/qa-artifacts/qa/network-observation/`;
  `/Users/pedronauck/dev/qa-labs/compozy-compozy-migration-beta-20260727-135201-116083-lab/qa-artifacts/qa/cross-surface/`.
- Cross-workspace-access reproduction (2026-07-29):
  `/Users/pedronauck/dev/qa-labs/compozy-northstar-pay-20260729-124649-419333-lab/qa-artifacts/qa/observation-summary.json`;
  all twelve declared tasks and runs are independently completed in
  `/Users/pedronauck/dev/qa-labs/compozy-northstar-pay-20260729-124649-419333-lab/qa-artifacts/qa/task-terminal-statuses.json`,
  while the journey log still contains no runtime-owned completion row.
- Loops-paper-adoption reproduction (2026-08-01):
  `/Users/pedronauck/dev/qa-labs/compozy-northstar-pay-20260801-135009-390014-lab/qa-artifacts/qa/observation-summary.json`;
  the observer marked seven declared tasks unstarted and twelve runs without completion while the
  public task catalog in the same lab showed all twelve received runs, ten completed, and two
  explicitly blocked on typed dependencies:
  `/Users/pedronauck/dev/qa-labs/compozy-northstar-pay-20260801-135009-390014-lab/qa-artifacts/qa/task-terminal-statuses.json`.
- Bundles-removal reproduction (2026-08-02):
  `/Users/pedronauck/dev/qa-labs/compozy-devtool-oss-launch-20260802-195112-911343-lab/qa-artifacts/qa/observation-summary.json`;
  the observer reported all eleven runs without completion and six task-owning agents as silent,
  while five independent public Task catalog snapshots show all eleven tasks `completed` with
  durable `closed_at` timestamps under the same lab's `qa/tasks-final-*.json` files.

## Fix

- **Root cause:** `observe-runtime.py` exclusively tails `qa/journey-log.jsonl`, but the Compozy daemon,
  agent sessions, task scheduler, and Network runtime have no writer that projects their durable
  lifecycle events into that log. Only bootstrap/controller helpers wrote rows in this run. The
  observer then derived task and agent state from that incomplete log instead of a runtime-owned
  public progress stream.
- **Fix commit:** none
- **Regression test:** pending a runtime-to-observer progress contract and a fresh one-kickoff replay
  proving the log grows through task completion without controller-authored runtime actions.

## Verification

- **Retested:** 2026-07-19 in a fresh isolated attempt after correcting the playbook review channel
  and CLI run-status rendering.
- **Result:** Reproduced. Controller-authored observations kept the journey log non-empty across all
  five required surfaces, but the observer still raised `stall_detected=true` and derived stale
  task state because runtime-owned lifecycle events were absent. No second provider prompt was sent
  to conceal the observer stall.
- **Retested:** 2026-07-27 in fresh isolated migration-beta lab
  `compozy-migration-beta-20260727-135201-116083`.
- **Result:** Reproduced again. `observe-runtime.py` ran the full 1,800-second window, set
  `stall_detected=true` at 2026-07-27T14:14:28Z, and listed every seeded run under
  `tasks_in_progress_no_completion`, despite independently persisted Task and Network progress.
  Exactly one provider kickoff was sent; no operator follow-up masked the stall.
- **Retested:** 2026-07-29 in fresh isolated lab
  `northstar-pay-20260729-124649-419333`.
- **Result:** Reproduced again. The initial observer exhausted its stall threshold before the task
  catalog reached terminal state; afterward all twelve declared tasks and runs were independently
  `completed`, but the journey log still had no runtime-owned task/session/Network progress. A final
  short observer pass truthfully indexed the observer-owned CLI comparison without fabricating
  runtime completion rows. Exactly one provider kickoff was sent.
- **Retested:** 2026-08-01 in fresh isolated lab
  `northstar-pay-20260801-135009-390014`.
- **Result:** Reproduced again. The 1,800-second observer set `stall_detected=true` at its five-minute
  threshold and diagnosed seven declared tasks as unstarted plus all twelve runs as lacking
  completion. Independent persisted reads at the end showed ten declared tasks completed and two
  explicitly blocked after every declared run had started. Three disruption probes also reached
  recorded recoveries. Exactly one provider kickoff was sent; no follow-up prompt concealed the
  observer mismatch.

## Re-found — 2026-08-02

- **Persona:** Mateo Rivera, release operator for the `devtool-oss-launch` playbook.
- **Report:** `docs/qa/reports/2026-08-02-bundles-removal.md`.
- **Result:** Reproduced after exactly one kickoff. All eleven Task catalog entries completed between
  `2026-08-02T20:00:35Z` and `2026-08-02T20:05:05Z`, but the 1,800-second observer saw no
  runtime-owned progress after scheduler resume and set `stall_detected=true`. Observer-authored CLI
  comparison rows were appended afterward with actor `qa-observer`; they do not masquerade as runtime events.

## Re-found — 2026-08-13

- **Persona:** Mateo Rivera, release operator for the `devtool-oss-launch` playbook during the
  worktree-support release QA.
- **Report:** `docs/qa/reports/2026-08-13-worktree-support.md`.
- **Result:** Reproduced after exactly one kickoff. The observer stopped at its five-minute stall
  threshold and reported all eleven runs without completion, while the independent public Task
  catalog already showed ten tasks `completed` and only the disruption-dependent decision task
  `ready`. No follow-up provider prompt was sent.
- **Evidence:**
  `/Users/pedronauck/dev/qa-labs/compozy-worktree-support-20260813-083057-155448-lab/qa-artifacts/qa/observation-summary.json`;
  `/Users/pedronauck/dev/qa-labs/compozy-worktree-support-20260813-083057-155448-lab/qa-artifacts/qa/tasks-runtime-0840.json`.

## Re-found — 2026-08-16

- **Persona:** Sofia Mendes, CTO for the `northstar-pay` Herdr parity replay.
- **Report:** `docs/qa/reports/2026-08-16-herdr-parity.md`.
- **Result:** Reproduced after exactly one kickoff. The 1,800-second observer stopped growing after
  fourteen bootstrap/controller rows, set `stall_detected=true` at its five-minute threshold, and
  classified all twelve declared runs as incomplete plus all twelve declared tasks as unstarted.
  No follow-up provider prompt was sent to conceal the stalled public progress surface.
- **Evidence:**
  `/Users/pedronauck/dev/qa-labs/compozy-northstar-pay-20260816-141901-835450-lab/qa-artifacts/qa/observation-summary.json`;
  `/Users/pedronauck/dev/qa-labs/compozy-northstar-pay-20260816-141901-835450-lab/qa-artifacts/qa/journey-log.jsonl`.

## Closure verification — scheduled 2026-08-21

Planned by the `loop-task-legibility` QA cycle (`.compozy/tasks/loop-task-legibility/task_06.md`);
executed by that program's QA tail (task_07). Status stays `open` until that run records a verdict.

- **Why this cycle can close it.** The root cause is that the daemon, agent sessions, task scheduler
  and Network runtime had no writer projecting lifecycle events into any stream the observer could
  read — so `observe-runtime.py` tailed `journey-log.jsonl` and derived task and agent state from an
  incomplete log. This program ships the missing surface: a runtime-owned public progress read.
  `compozy loop runs` now serves `progress{round, steps_done, steps_total}` on every item plus
  `attention{kind, count, since}` when something waits; `compozy loop why` always returns a non-empty
  verdict; `compozy loop events <run-id> --after <seq> --follow` resumes from a durable per-run sequence; and
  `compozy task list` returns a calm, truthful catalog on the same persisted state. None of these
  existed at any of the six reproductions.
- **Owning charters.** `CH-loop-legibility-run-read-resume` (the agent-side progress stream and its
  resume seam), `CH-loop-legibility-run-default-read` and `CH-loop-legibility-operator-register`
  (the human-side registers ADR-002 and ADR-003 cite this bug to justify).
- **Pass condition.** A one-kickoff replay in a fresh isolated lab where progress is derived from the
  runtime-owned reads above instead of from tailing `journey-log.jsonl`, and the observer's account
  of task and run state matches an independent public task-catalog read for the whole window — no
  `stall_detected` while the catalog is advancing, and no task reported unstarted that the catalog
  shows started. A run that reproduces the divergence against the new surfaces keeps the bug open and
  supersedes the root-cause statement above.
- **Regression debt.** Recorded at `docs/qa/automation-backlog/runtime-owned-progress-observer.md` —
  the fix has no regression test yet, and a sixth-plus reproduction is what earns one.

## Blocked decision — 2026-08-21 root review

The scheduled closure did not run against the promised observation contract. The fresh
`northstar-pay` lab continued to execute `observe-runtime.py`, whose module contract and
implementation only tail `qa/journey-log.jsonl`; it explicitly does not read the daemon, SSE, or
Network. That run again declared a stall while an independent public Task catalog later showed
seven completed tasks. Exercising `loop why`, `loop runs`, and `loop events` separately proves those
reads exist, but it does not prove that the release observer derives its account from them.

This is outside the bounded fix governor because closure requires a product/QA architecture choice,
changes to the shared real-scenario observer and audit contract, and a fresh one-kickoff playbook
run. The recommended decision is to make the observer poll the daemon's public structured reads
and compare them with an independent catalog read. Making the production daemon write into a
lab-owned QA file would invert ownership and is not recommended. Status remains `open`; task 07 and
the runtime phase cannot be reported as PASS until that decision is implemented and the stated pass
condition is re-walked.
