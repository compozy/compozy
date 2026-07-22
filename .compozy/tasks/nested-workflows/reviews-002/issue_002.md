---
provider: manual
pr:
round: 2
round_created_at: 2026-07-22T15:39:03Z
status: resolved
file: skills/cy-create-prd/references/user-stories-template.md
line: 54
severity: high
author: claude-code
provider_ref:
---

# Issue 002: Authorization coverage is sampled instead of systematic

## Review Comment

The PRD edge-case sweep reduces authorization to one generic “Permissions” probe, and downstream test assignment only distributes IDs already present in `_tests.md`. It never expands security requirements into systematic coverage. A catalog with one allowed and one denied example can therefore pass assignment even when other protected operations, sensitive fields, actors, or denial side effects are untested.

Add an authorization rule pack that models operations (`create`, `read`, `update`, `delete`, `transition`, `replay`), data classifications, actors/roles/capabilities, expected outcomes (`allow`, `deny`, `redact`, `ignore`), and permitted side effects. Require a complete matrix for security-sensitive behavior and allow documented pairwise coverage only for lower-risk behavior. The generation gate must reject any protected operation without a negative test. Generated cases must cover every client-controlled sensitive field, assert that denial leaves protected state unchanged, and verify field-level read redaction rather than only endpoint access.

## Triage

- Decision: `VALID`
- Notes: The current template reduces authorization discovery to one generic
  `Permissions` probe. It does not enumerate protected operations, sensitive
  data, actors, outcomes, side effects, or coverage requirements, so a PRD can
  omit negative and field-level authorization behavior while still satisfying
  the edge-case sweep. Add an authorization rule pack to the generated
  `_user_stories.md` contract, require exhaustive coverage for security-sensitive
  combinations (with documented pairwise coverage only for lower-risk cases),
  and add generation gates for negative tests, client-controlled sensitive
  fields, state preservation on denial, and field-level read redaction.
- Verification: `make verify` passed against the exact batch diff from a
  short-path temporary clone. The nested review worktree itself exceeds the
  macOS Unix-domain socket path limit used by Playwright's daemon fixture;
  before relocation, all earlier checks passed and only daemon bootstrap failed.
