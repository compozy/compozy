# Expected outcome — feat-noop

Exercises: no-op when there are no promotable decisions (E2E-017).

The fixture has one `Proposed` (not `Accepted`) ADR and one `Accepted` but feature-local ADR.

## After capture on an existing log

- Zero promotions. Run summary lists both ADRs with reasons: `adr-001` not `Accepted`; `adr-002`
  feature-local (fails the relevance gate).
- No new files created; the existing log is unchanged (no empty or dangling files written).

## After capture on a fresh project (no log yet)

- A valid empty-state `.compozy/DECISIONS.md` is written (header + "No active, proven decisions captured
  yet."), and no `.compozy/decisions/AD-*.md` bodies are created.

## Assertions

- No `AD-*.md` body is ever created for a non-`Accepted` or feature-local ADR.
- The empty-state index is well-formed and safe to `@import`.
