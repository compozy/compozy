# BUG-20260911-blocked-goal-manual: Blocked Goal guidance requires unnecessary replacement

- **Status:** verified
- **Impact (user-side):** Friction
- **Severity:** Low · **Priority:** P3
- **Persona Affected:** Lea and managed agents
- **Journey Step:** J-26 start after a terminal blocked Goal
- **Scenarios:** GL-012
- **Found:** 2026-09-11
- **Report:** docs/qa/reports/2026-09-10-qa-execution-unblock.md

## Reproduction and cause

The official Loop reference says an existing Goal requires replacement and specifically tells agents to replace or clear a terminal blocked Goal. GL-012 already requires direct start after blocked. The real Web walk confirmed no Resume, CLI goal_not_active for resume, and a plain new /goal starting a new Run without replace or clear. Its predecessor remains exhausted; the successor completes approved and survives reload.

## Fix and validation

Clarify that replacement is required for a live Goal and that a new Goal can start directly after terminal blocked. No runtime change, migration, native tool, API, hook, configuration, Web behavior or workspace boundary changes. Official-skill impact: editorial correction only. Reviewed the two changed sentences, unchanged links and grammar against goal-blocked-proof.json. No prose-only regression test; no agent-compliance improvement is claimed. Reuse the passing production gate from ed4593651; this documentation-only patch cannot affect lint, typecheck or executable test behavior. Local commit: f062946a8. Explicit no-stash precommit, commitlint and diff checks passed.
