# Fix Loop

What happens between finding and closing. The loop is governed: the runner judges the **size of the fix before touching code**, fixes only what is safely inside bounds, and escalates the rest with options — because an agent that patches the product mid-QA to clear its own findings is optimizing the dashboard, not the product.

Fidelity boundary first (per the persona-fidelity guardrails, routed at Step 3 of the SKILL): fixes never happen *inside* a session. A session ends, findings exist, then the governor runs.

## Contents

- The governor
- Auto-fix requirements
- Decisions for a Human
- Paper cuts in the loop
- Retest protocol
- The exit gate
- Anti-patterns

## The governor

For each `fail` finding or sharp paper cut, judge before editing. **Auto-fix only when ALL hold:**

- **Small** — a few files, no schema/data migration, no API contract change.
- **Well-understood** — root cause identified and stated (symptom ≠ cause, written separately in the bug).
- **Low-risk** — blast radius contained to the touched surface; adjacent journeys unlikely to shift.
- **No product trade-off** — the correct behavior is unambiguous. Anything a PM/designer might reasonably decide differently is not yours to decide.

These are defaults for QA-only scope. Existing user authorization to repair a broader issue takes precedence; continue that work outside the persona session. Ask only for an unresolved product decision or an action beyond authorization. If scope grows, preserve work and record the open finding; do not run destructive Git without permission.

## Auto-fix requirements

Every auto-fix ships with, non-negotiably:

1. **Proof of the corrected invariant** in its owning suite or real replay. Reuse existing coverage; add a regression only for an uncovered invariant. Copy/visual changes use before/after replay evidence, without a mandatory automation-debt entry.
2. Keep each logical fix reviewable and cite the bug id. Commit only when the active workflow authorizes it.
3. **Root cause in the bug file** (`Fix` section: root cause, commit SHA, regression test path).
4. **Retest** per the protocol below before the finding's row moves to `Fixed`.

## Decisions for a Human

Escalations are findings with a recommendation, recorded in the report's **Decisions for a Human** section:

```markdown
### <finding title> (bug id / paper cut)
- What's broken: <user-side description, evidence path>
- Why not auto-fixed: <which governor bound it fails>
- Options:
  1. <option> — <trade-off>
  2. <option> — <trade-off>
- Recommendation: <one of the options, with the reason>
```

The matrix row becomes `Blocked (human decision)` — a terminal state for this run. It is never silently retried, and the scenario's `qa_status` becomes `blocked-decision` so the tracker surfaces it across cycles.

## Paper cuts in the loop

Sharp paper cuts (the persona would complain or hesitate to return) enter the governor like failures — many are copy or spacing fixes squarely inside auto-fix bounds, and fixing them is where dogfooding pays for itself. Dull ones stay in the report for pattern-watching; a paper cut recurring across personas or cycles gets promoted to a `Friction` bug.

## Retest protocol

After any fix:

1. Re-walk the impacted journey **from scratch, in persona** — a fresh session (fresh state, real entry), not a resumed browser. The Recovering User persona is often the right walker: it tests the fix *and* the recovery experience.
2. Re-walk adjacent journeys when a shared component or service makes propagation plausible; no mandatory neighbor walk for an isolated change.
3. On pass: bug → `fixed` (then `verified` once confirmed under the original persona), matrix row → `Fixed`, tracker row per the schema (`fix_status: fixed`, `retest_status: pass`, `fix_commits` updated).
4. On fail: reopen and diagnose; repair within authorization or record the specific decision needed. Preserve unrelated changes.

## The exit gate

Before Final Status, satisfy the project's required local gate once or cite current evidence for the same inputs. Run affected checks after fixes; do not repeat unchanged successful checks. When delivery includes a PR, report required CI at its current head. A red required gate keeps the corresponding readiness claim open.

## Anti-patterns

- **Fix-forward inside the session** — patching mid-walk destroys role fidelity and hides how a real user experiences the bug.
- **Fix without regression proof** — "fixed it, looks good" is a claim. Red-before/green-after or a documented replay, always.
- **Scope-creep fixes** — "while I'm here" refactors ride along and widen blast radius. Keep the patch scoped to the finding.
- **Deciding product questions** — picking a behavior a designer might pick differently, to clear a row. Escalate with options.
- **Ignoring plausible propagation** — verify adjacent journeys when shared behavior changed.
- **Silently requeueing blocked items** — `Blocked (human decision)` and `Blocked (needs human verify)` are terminal for the run; they wait for a person, visibly.
