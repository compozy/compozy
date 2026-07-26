---
name: qa-report
description: >-
  Plans real-user QA as living repo docs — the durable <qa-docs-path> tree
  (default docs/qa/) that every QA cycle appends to. Use when bootstrapping or
  updating a project's QA docs, planning a cycle before execution (map journeys
  as flows, derive scenarios, plan persona-driven session charters), or
  registering bugs into the durable bug registry. Do not use for live
  session execution, browser evidence, or fix loops — use qa-execution for those.
trigger: explicit
argument-hint: "[qa-docs-path]"
metadata:
  author: Pedro Nauck
  github: https://github.com/pedronauck
  repository: https://github.com/pedronauck/skills
---
# Real-User QA Planner

Plan QA as journeys real people walk, not test cases that accumulate. This skill owns the project's **living QA docs** — one committed tree (`<qa-docs-path>`, default `docs/qa/`) that every round appends to — and plans the persona-driven sessions `qa-execution` runs.

Two rules anchor everything:

1. **Living docs, not round artifacts.** All durable QA knowledge lives in the one committed tree; rounds append to it (structure, durability, and anti-patterns: `references/qa-docs-layout.md`).
2. **Sessions, not cases.** The atomic planning unit is the **session charter** (persona + journey + tour + time-box), derived from journey flowcharts. Coverage means "every planned journey was walked by a persona this cycle" — a session ledger, never a per-case count.

Each step points at the reference file that owns its contract; read that file before producing the step's deliverable — the inline text is a pointer, not the spec.

## Required Inputs

- **qa-docs-path** (optional): root of the living tree; defaults to `docs/qa` at the repo root — a durable, committed location, never a temp dir. If the argument points outside the repository, confirm before proceeding: living docs outside the repo lose review, diff, and history.

## Procedures

**Step 1 — Resolve or bootstrap the tree.** Read `references/qa-docs-layout.md` (canonical tree, gitignore block, bootstrap procedure, adoption procedure for scattered legacy artifacts). Resolve `<qa-docs-path>`. If the tree exists, read its `README.md` and scan `scenarios/` and open `bugs/` first, and build every decision below on that state; when the branch just merged parallel QA work, reconcile before planning — two files describing one behavior or one symptom fold into the older id (merge verdict fields by `last_report` recency, update references, delete the duplicate, record the fold in the cycle's report). If the tree does not exist, bootstrap it per the layout reference — directory tree, seeded `templates/`, and the gitignore block. If legacy QA artifacts sit scattered outside it, adopt them per the reference: index them, migrate durable knowledge, re-mint counter-based ids.

**Step 2 — Establish project personas.** Read `references/personas.md` (seed catalog + derivation rules). Personas are durable instance data in `<qa-docs-path>/personas.md`: update them only when the product's audience changed; if absent, derive 3-6 from the seed catalog, adapted to the product's real audience.

**Step 3 — Map journeys as flows (before any scenario).** Read `references/journeys-and-flows.md` (journey anatomy, Mermaid mapping, flows-before-matrix). Scope the mapping: a branch/PR cycle covers every user-visible change in the diff; a release cycle covers the product's high-value journeys. For each, write or update `<qa-docs-path>/journeys/J-<slug>.md` — the YAML journey map plus a Mermaid flowchart from entry → actions → branch points → side effects → the **true end state**, with at least one abandonment path. Map the flow first; the scenario comes from it.

**Step 4 — Derive scenarios into the tracker.** Read `references/state-schema.md` (fields, enums, id minting — exact) and `references/taxonomy.md` (the five coverage dimensions). Walk each flowchart and derive scenarios: one `scenarios/<AREA>-<slug>.md` file per scenario with a content-addressed id, updated in place, overlaps recorded in the `overlaps` field. Sweep the five taxonomy dimensions per journey so coverage is deliberate. Scenario files are planning output — `qa_status` stays `untested` until `qa-execution` runs them.

**Step 5 — Plan session charters.** Read `references/session-charters.md` (charter anatomy, cadence tiers, the coverage inversion). Pick the cadence tier (smoke / targeted / full / sanity); the tier picks the journeys. Write one charter per session to `<qa-docs-path>/charters/CH-<slug>.md` from `<qa-docs-path>/templates/charter.md` (seed: `assets/charter-template.md`), preserving its headings — mission, persona, journey, exactly one tour, time-box, must-try guidance — ordered by risk: highest-impact journey × highest-blast-radius tour first. Reuse an existing charter whose mission still fits before writing a sibling.

**Step 6 — Register bugs.** Read `references/bug-registry.md` (id minting, dedup, the five-tier user-impact rubric — the canonical severity model for both skills). Dedup before filing: search `<qa-docs-path>/bugs/` for the symptom and update the existing file rather than duplicating — a re-found bug is history worth keeping on one id. Only a genuinely new symptom mints a new content-addressed `BUG-<YYYYMMDD>-<slug>` id; write it from `<qa-docs-path>/templates/bug.md` (seed: `assets/bug-template.md`), preserving its headings, and link the id into the affected scenario files' `bug_ids`.

**Step 7 — Validate cycle completeness.** Before handing off to `qa-execution`, verify — and record gaps honestly rather than padding:

- every in-scope journey has a flowchart with a true end state and ≥1 abandonment path;
- every in-scope journey has ≥1 charter with an assigned persona;
- every in-scope scenario file has a content-addressed id, a linked journey, and a `qa_status` reflecting reality;
- every open bug has a registry file and appears in ≥1 scenario's `bug_ids`;
- the five taxonomy dimensions were considered per journey — a skipped one is recorded with reasoning.

The completeness bar is "every journey walked by a persona", a session ledger — never a per-case count. Case accumulation is the failure mode this skill exists to prevent.

When a journey grows stable or regression-prone enough to deserve an automated E2E spec, read `references/automation-backlog.md` in full, then record the intent as one file in `<qa-docs-path>/automation-backlog/` — one backlog, never automation fields on individual scenarios or charters.

## Companion Skills

- **qa-execution** — runs the sessions this skill plans and writes results back into the same tree (statuses, bugs, reports). The living tree is the contract between the two.
- **agent-output-audit** — owns CI verification gates, AI test-hygiene scans, and task-status reconciliation. Route technical integration/security/performance/load suites there or to dedicated tooling; record the routing decision, don't absorb the work.

## Error Handling

- **A scenario file's frontmatter won't parse** (missing delimiter, unknown field, nested value): repair it and report what was repaired before any downstream step — every step depends on a loadable tracker.
- **Two files describe one behavior or one symptom under different slugs** (typical after merging parallel QA branches): run the Step 1 fold before any downstream step plans on top of the duplicates.
- **A branch cycle's diff has no user-visible change:** say so and stop; there is nothing to dogfood. Do not invent scenarios to fill a cycle.
- **`<qa-docs-path>` can't be created** (permissions, read-only checkout): surface the error and stop — never fall back to a temp directory.
