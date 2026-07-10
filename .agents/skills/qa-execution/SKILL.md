---
name: qa-execution
description: >-
  Runs real-user dogfooding sessions through the product's public interfaces: a
  persona walks a journey in the browser, takes a thematic tour, probes edges,
  hunts paper cuts, and reports what a real user would experience. Reads its
  plan from the living QA docs tree (<qa-docs-path>, default docs/qa/). Use when
  validating a release candidate, branch diff, migration, or user-facing change
  against production-like behavior. For planning that tree — personas, journeys,
  charters, the bug registry — use qa-report; for CI gate runs, AI
  implementation audits, or integration/security/performance suites, use
  agent-output-audit.
argument-hint: "[qa-docs-path]"
metadata:
  author: Pedro Nauck
  github: https://github.com/pedronauck
  repository: https://github.com/pedronauck/skills
---
# Real-User QA Execution

QA the product the way a real person meets it: a **persona** walks a journey through the product's public interfaces, feels the friction, hits the edges, and reports what happened. This is **dogfooding**, not a scripted test pass — the session is the work, and the living QA docs tree remembers it.

Three non-negotiables hold every session:

1. **In persona.** Every interaction and every verification goes through a surface a real user can reach — no dev-tools shortcut, no code-reading to decide what should happen, no patching over a stall.
2. **Proof, not optimism.** A `Pass` is the expected observable seen, confirmed through an independent read path, surviving a refresh, with evidence captured. Optimistic UI is not confirmation.
3. **Write back or it didn't happen.** Every session updates the tree — charter debrief, `state.csv` verdicts, bug registry, dated report.

## Input

- **qa-docs-path** (optional): root of the living QA docs tree; defaults to `docs/qa/`. The tree is this skill's memory and its only output location — never a temp dir. If it doesn't exist, run `qa-report` first; it owns the tree and its bootstrap.

## Steps

Each step names the reference that owns its detail — read it in full when you reach the step; the inline text is the trigger, not the contract.

**Step 1 — Resolve the tree, scope, and preconditions**
- Read, in order: `<qa-docs-path>/README.md` (entry points, dev-server command, area codes), `state.csv`, open `bugs/`, and this cycle's charters. The tree is the memory; running without reading it recreates the duplication this design kills.
- Scope: a **branch/PR run** covers the journeys its user-visible diff touches plus one adjacent canary — no user-visible change, report that and stop. A **release/full run** covers the journeys the cycle plan marked in scope.
- Preconditions: the automated suite is green (a precondition, not a QA step — gaps route to `agent-output-audit`) and the product is reachable in a production-parity build (real dev server, real auth, no mocks). Not reachable → name the exact gap and stop.
- **Done when:** scope is fixed and every precondition is met or its gap is surfaced.

**Step 2 — Build the matrix and create the report now**
- Read `references/status-and-reporting.md` — it owns the six-value status enum and the report lifecycle.
- Assemble the session matrix from the planned charters: persona × journey × tour × time-box, ordered by risk. A charter missing for an in-scope journey is drafted per `../qa-report/references/session-charters.md` before running — never walk unplanned.
- Create `<qa-docs-path>/reports/<YYYY-MM-DD>-<scope>.md` from the report template (project copy at `<qa-docs-path>/templates/report.md`, else `assets/report-template.md`) **before the first session**, with every matrix row `Pending`. This on-disk report is the source of truth for resume — update it after every session and every fix, never only at the end.
- **Done when:** the report exists on disk carrying the full matrix, every row `Pending`.

**Step 3 — Walk each journey in persona**
- Read `references/session-protocol.md` (the enter→act→verify→capture loop and the evidence standard) and `references/persona-fidelity.md` (the public-interface guardrails and stall-is-a-finding).
- For each charter, in matrix order: adopt the persona (device, network, locale), enter through its real entry point, and walk the journey verb by verb to its **true end state** — verifying each step against the evidence standard.
- Hunt **paper cuts** throughout: persona-felt friction no functional check fails; sharp ones become findings.
- A leg only a human can complete (real payment, external email/SMS, real OAuth) is marked `Blocked (needs human verify)` with exact instructions — never faked.
- **Done when:** every charter is walked to a recorded verdict, evidence captured at checkpoints and divergences, the debrief appended to the charter file, and the matrix row updated.

**Step 4 — Run each tour and edge probe**
- Read `references/tours.md` (the 10-tour catalog and surface-to-tour matrix) and `references/edge-cases.md` (the non-technical user edge cases).
- Run each charter's single **tour** against its surface, in persona, inside the box, asking at each action: *"would this matter for this tour's theme?"*
- Pick 5-10 edge cases matching the surface and persona and attempt them; attempted-and-clean is evidence too.
- **Done when:** every charter's tour is run and its chosen edge cases are attempted and recorded.

**Step 5 — Experiential lens pass**
- Read `references/lenses.md` — the six lenses and their severity defaults.
- Pick the 2 journeys covering the largest changed surface and re-walk them holding the six lenses in a 45-minute box, recording `pass` / `friction` / `fail` per lens.
- **Done when:** both journeys are re-walked and every lens verdict is recorded.

**Step 6 — File findings into the registry**
- Read `../qa-report/references/bug-registry.md` — it owns ids, dedup, and the impact rubric.
- Dedup first: search `bugs/` and the affected rows' `bug_ids`. Re-found → append `## Re-found`; regressed → reopen with `## Regressed`; only a genuinely new symptom mints a new `BUG-NNNN`.
- File with the user first — impact tier, persona, journey step, reproduction from the persona's entry point, evidence paths — then link the id into the affected `state.csv` rows.
- **Done when:** every finding is deduped, filed, and linked to its rows.

**Step 7 — Fix loop (governed)**
- Read `references/fix-loop.md` — the **governor**, the regression-test-per-fix rule, and Decisions for a Human.
- Judge each fix against the governor **before editing**: only what passes all its bounds is auto-fixed, and each auto-fix ships a regression test (red before, green after, or a documented replay) and re-walks its impacted and adjacent journeys in persona. Everything else goes to the report's **Decisions for a Human** with options and a recommendation.
- **Done when:** every finding is either fixed-and-retested or escalated with a recommendation, and no fix is left half-applied.

**Step 8 — Close the round**
- Re-read the round-close checklist in `references/status-and-reporting.md`; map matrix verdicts to tracker enums per `../qa-report/references/state-schema.md`.
- Exit gate: run the project's full automated suite once and record the result verbatim — a green matrix over a red suite is not ready, and Final Status must say so.
- **Done when:** zero matrix rows are `Pending`, `state.csv` verdicts and bug statuses are current, charter debriefs are appended, and the report's Final Status states release readiness with totals by impact tier — backed by fresh evidence from the current build.

## Companion skills

- **qa-report** — plans what this skill runs and owns the tree's schemas (tracker, bug registry, charters, personas, journeys). Results written here feed the next cycle's planning.
- **agent-output-audit** — owns CI gates, AI test-hygiene scans, task-status reconciliation, and flaky-test triage. A session that uncovers those files the finding and names that gate; it does not pivot mid-session.
- **agent-browser** — the browser driver for Steps 3-5; its command surface lives in `references/session-protocol.md`.

## Error handling

- **Dev server or browser tooling unavailable:** mark the browser legs `Blocked (needs human verify)` with the exact missing prerequisite, and continue with CLI/HTTP journeys still walkable in persona.
- **A flow hangs:** close the session, record it, retry once from a clean session, then mark it blocked. A stall is a finding to file, never a thing to nudge past (`references/persona-fidelity.md`).
- **Credentials or test data missing:** mark those sessions blocked with the exact prerequisite and proceed with the rest.
- **Matrix larger than the window:** cut by risk (Blocks-Completion candidates first, then Data-Loss, then Trust-Damage), mark the cut rows `Skipped` with reasoning, and disclose it in Final Status — coverage shrinks visibly or not at all.
