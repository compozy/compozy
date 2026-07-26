# QA Living Docs Migration — 2026-07-12

- **Scope:** Adopt the conflict-resistant `qa-report` / `qa-execution` contract without changing runtime behavior or re-running historical sessions.
- **Status:** in-progress
- **Source tree:** `docs/qa/`

## Adoption Result

- The tracked 439-row `state.csv` was exploded into one source file per scenario under `scenarios/`; all legacy ids and lifecycle fields were preserved.
- The legacy CSV contained one malformed row: `RT-083` had one surplus empty field. The row was repaired to the 16-field schema before conversion.
- The generated tracker view round-tripped to 439 rows with zero parse errors. It is now gitignored and may be regenerated on demand.
- The shared 12-item `automation-backlog.md` was split into one content-addressed file per item under `automation-backlog/`, with `AB-001..AB-012` retained as legacy lookup ids.
- Project templates now cover scenarios and use content-addressed journey, charter, and bug placeholders. Session debriefs live only in dated reports.
- Healthy existing counter ids remain grandfathered because historical reports cite them. No counter is used for new ids.
- Existing charter-embedded debriefs remain frozen as pre-migration history; new runs must not edit them.

## Historical Cycle Index

This section closes the former shared README changelog. Details remain in the dated reports.

- 2026-07-11 — Agent-details execution: `2026-07-11-agent-details.md`; CH-049..CH-052, BUG-0035..0038, full automated gates, clean teardown.
- 2026-07-11 — Agent-details plan: `2026-07-11-agent-details-plan.md`; J-30..J-32 and CH-049..CH-052.
- 2026-07-11 — Goal plan: `2026-07-11-goal-plan.md`; J-23..J-29, GL-001..GL-040, CH-037..CH-045, AB-010..AB-012.
- 2026-07-11 — Frontend/runtime stability plan: `2026-07-11-frontend-stability-plan.md`; recovery journeys and stale-verdict resets.
- 2026-07-10 — Model-selector plan and execution: `2026-07-10-model-selector-plan.md` and `2026-07-10-model-selector.md`; J-17..J-22 and CH-028..CH-036.
- 2026-07-09 — Loops-refac execution: `2026-07-09-loops-refac.md`; J-16, CH-022..CH-027, BUG-0022..0023.
- 2026-07-08 — Session-improvements plan and execution: `2026-07-08-session-improvements-plan.md` and `2026-07-08-session-improvements.md`; J-11..J-15 and CH-014..CH-021.
- 2026-07-08 — Loops-refac planning/reconciliation: `2026-07-08-loops-refac-plan.md` and `2026-07-08-loops-refac.md`.
- 2026-07-08 — Session observability: `2026-07-08-session-observability.md`.
- 2026-07-06 — Loops plan and execution: `2026-07-06-loops-plan.md` and `2026-07-06-loops.md`; J-01..J-10, CH-001..CH-013, BUG-0018..0019.
- 2026-07-05 — Initial living-tree bootstrap from feature stories, final-QA seeds, and the E2E playbook; old per-round trees retired.

## Validation

- Scenario materialization: pending final validation.
- Referential integrity: pending final validation.
- Repository gate: pending final validation.

## AGH Impact Audit

- Native tools: no impact — no `agh__*` ids, descriptors, schemas, digests, risk flags, availability diagnostics, or capability gates changed.
- Extensibility and hooks: no runtime impact — extension, hook, capability, tool/resource, bundle, registry, bridge SDK, MCP, and config lifecycle surfaces are unchanged; only QA workflow instructions that inspect them are migrated.
- Workspace data isolation: no runtime data change — no global/workspace/session/agent datum or propagation path changed; scenario records preserve their existing workspace-isolation expectations.
- Official AGH skill: no public AGH behavior changed — `skills/agh/` requires no capability or tool guidance update; local QA orchestration skills are updated only to consume the new tracker source.
