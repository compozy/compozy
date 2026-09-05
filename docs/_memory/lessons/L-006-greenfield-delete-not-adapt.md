# L-006 — Name delete targets and their compatibility regime

> **Scope narrowed 2026-09-04** (SD-013, [L-040](L-040-real-users-end-zero-legacy-posture.md)). The rule below — enumerate delete targets — still binds every breaking-change spec. Its "no migration code, no aliases" posture now applies to internal code only: user state migrates losslessly and public surfaces deprecate one release before deletion.

**Class:** Project posture
**Date discovered:** 2026-04-17 (harness TechSpec review, Portuguese-language reviewer)
**Evidence sources:** Harness review + `remove-legacy-alpha.md` standing directive + repeated architecture reviews.

## Context

The harness TechSpec proposed migrating an `inputAugmenter` callback to a `TurnAugmenter` pipeline. The spec did not say whether the old callback was deleted, kept as an adapter, or coexisting. The reviewer (in Portuguese) flagged this directly: _"política zero-legacy exige declarar 'delete'"_ — the zero-legacy policy _requires_ the spec to declare what is deleted.

This is a stronger application of the CLAUDE.md "Greenfield Alpha — Zero Legacy Tolerance" rule: it's not enough to _allow_ deletion; specs must _enumerate_ what is deleted.

## Root cause

When a spec says "we are migrating to X" without naming the delete-target, agents default to keeping both. Compatibility shims, adapters, and "preserve old behavior" branches accumulate as technical debt. Greenfield discipline only works if every breaking-change spec explicitly names what disappears.

## Rule

> Breaking-change specs enumerate delete targets and classify each as user state, public surface, or internal code under SD-013.

## Operationalization

- Internal renames update all consumers together and delete obsolete types/fields without aliases.
- User state ships a lossless migration. Public renames auto-migrate at the boundary or keep the old shape for the SD-013 deprecation window, with a replacement warning and removal release.
- Record these decisions in the owning compatibility plan; task files link to it instead of repeating a deletion checklist.

## Allowed exception (single-pass repair)

The former single-boot repair exception is historical. Current state transformations use the owning explicit migration mechanism; SQLite migrations stay append-only. Data loss requires the user sign-off and release-note block defined in SD-013. The original incident is preserved in `session-driver-override/adrs/adr-005.md`.

## Source

- `.codex/plans/remove-legacy-alpha.md` (standing directive)
- `.codex/sessions/2026/04/17/.../exec-20260417-232547-929722000/turns/0001/response.txt` (harness review, Portuguese)
- Multiple `network-rename`, `assistant-ui-hard-cut`, `workspace-menu-hardcut` plans in `.codex/plans/`
- `../analysis/analysis_local_runs.md` lesson LL-1
