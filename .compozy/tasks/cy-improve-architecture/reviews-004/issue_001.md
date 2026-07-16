---
provider: manual
pr: 6
round: 4
round_created_at: 2026-07-14T02:01:21Z
status: resolved
file: extensions/cy-improve-architecture/skills/cy-improve-architecture/SKILL.md
line: 92
severity: medium
author: claude-code
provider_ref:
---

# Issue 001: Cross-feature ADR route contradicts the write boundary

## Review Comment

The invariant at line 18 permits writes only to the two reports, the audited
`ARCHITECTURE.md` section, and an accepted glossary update. Step 7.5 then
requires a durable cross-feature outcome to be routed by creating a workflow
draft ADR under `.compozy/tasks/<workflow>/adrs/`. That path is outside the
declared write set.

This leaves the E2E-035 contract with no conforming execution: an agent that
honors the invariant will skip the required draft ADR, while an agent that
creates it violates the invariant. Clarify the ownership boundary by either
explicitly allowing this one user-confirmed draft-ADR write (and documenting
its workflow selection and summary), or by changing the required route to an
instruction that hands off draft creation without writing it. Preserve the
existing prohibition on directly writing `.compozy/DECISIONS.md` or
`.compozy/decisions/`.

## Triage

- Decision: `VALID`
- Root cause: The write boundary permits only the reports, the audited
  `ARCHITECTURE.md` section, and an accepted glossary update, but workflow
  step 7.5 mandates creating a draft ADR under
  `.compozy/tasks/<workflow>/adrs/`. That required path is outside the
  declared set, so the skill contains contradictory instructions.
- Fix approach: Add one narrowly scoped, user-confirmed exception for the
  workflow draft ADR to the write boundary. Require step 7.5 to obtain the
  workflow and draft-summary confirmation and include both in the run summary,
  while retaining the prohibition on writing the durable decision log.
