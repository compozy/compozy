# L-006 — Greenfield + zero-legacy means _delete_, not _adapt_

**Class:** Project posture
**Date discovered:** 2026-04-17 (harness TechSpec review, Portuguese-language reviewer)
**Evidence sources:** Harness review + `remove-legacy-alpha.md` standing directive + repeated architecture reviews.

## Context

The harness TechSpec proposed migrating an `inputAugmenter` callback to a `TurnAugmenter` pipeline. The spec did not say whether the old callback was deleted, kept as an adapter, or coexisting. The reviewer (in Portuguese) flagged this directly: _"política zero-legacy exige declarar 'delete'"_ — the zero-legacy policy _requires_ the spec to declare what is deleted.

This is a stronger application of the CLAUDE.md "Greenfield Alpha — Zero Legacy Tolerance" rule: it's not enough to _allow_ deletion; specs must _enumerate_ what is deleted.

## Root cause

When a spec says "we are migrating to X" without naming the delete-target, agents default to keeping both. Compatibility shims, adapters, and "preserve old behavior" branches accumulate as technical debt. Greenfield discipline only works if every breaking-change spec explicitly names what disappears.

## Rule

> Every breaking-change techspec MUST explicitly name its delete targets. "Delete the old thing" is not a default; it is a checklist item that must be enumerated.

## Operationalization

In every TechSpec that changes a public surface (or any meaningful internal contract), include a section like:

```markdown
## Delete Targets

- `internal/foo.OldType` (replaced by `internal/foo.NewType` in step 3)
- `pkg/bar.LegacyAdapter` (no callers after migration; remove in step 5)
- TOML key `[old.section]` (renamed; no backward alias)
- HTTP endpoint `/v0/old/path` (replaced by `/v1/new/path`; no redirect)
```

Renames sweep code, storage, APIs, CLI, extensions, specs, RFCs, AND `.compozy/tasks/*` artifacts in the same change. No aliases, no dual fields, no migration code.

## Allowed exception (single-pass repair)

When the cost of "delete the old thing" is "every developer rebuilds their local SQLite," in-place ALTER + one-shot repair is allowed if and only if:

1. Repair is bounded to a single boot.
2. Strict semantics resume immediately after repair.
3. The exception is documented in an ADR.

Reference: `session-driver-override/adrs/adr-005.md`.

## Source

- `.codex/plans/remove-legacy-alpha.md` (standing directive)
- `.codex/sessions/2026/04/17/.../exec-20260417-232547-929722000/turns/0001/response.txt` (harness review, Portuguese)
- Multiple `network-rename`, `assistant-ui-hard-cut`, `workspace-menu-hardcut` plans in `.codex/plans/`
- `../analysis/analysis_local_runs.md` lesson LL-1
