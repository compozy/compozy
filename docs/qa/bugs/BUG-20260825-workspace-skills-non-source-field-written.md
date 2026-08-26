# BUG-20260825-workspace-skills-non-source-field-written: Workspace writes accept forbidden skill settings

- **Status:** verified <!-- open | fixed | verified | wont-fix | invalid -->
- **Impact (user-side):** Blocks-Completion
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Dora
- **Journey Step:** J-absorb-skills-from-other-tools, validate a workspace source policy
- **Scenarios:** ET-manage-skill-source-policy; ET-skill-source-agent-parity
- **Found:** 2026-08-25 · **Report:** docs/qa/reports/2026-08-25-skill-sources.md

## Summary

`compozy config set --scope workspace skills.poll_interval 9s` succeeds, writes the forbidden field
into the workspace overlay, and asks for a daemon restart. The workspace skills contract permits
only `sources` and `custom_sources`; every other field must fail with
`workspace_scope_field_forbidden` before a file changes.

## Reproduction

- **Charter:** CH-skill-sources-live-apply · **Tour:** Interrupt Tour
- **Environment:** desktop / wifi-fast / en-US · isolated lab daemon `127.0.0.1:55384`, build `1ece66f`

1. Run `compozy config set --scope workspace --workspace <id> skills.poll_interval 9s -o json`.
2. Inspect `<workspace>/.compozy/config.toml`.

**Expected:** exit 1 with `error.code: workspace_scope_field_forbidden`, `field: poll_interval`, and
no file mutation.
**Actual:** exit 0; `poll_interval = "9s"` is persisted with `restart_required: true`.

## Evidence

- `<lab>/qa-artifacts/qa/policy-workspace-field.json` — accepted mutation payload.
- The value was removed immediately after reproduction; the lab workspace contains no stale field.

## Fix

- **Root cause:** `ValidateConfigWriteScope` rejected workspace-only violations for Marketplace,
  Gateway, and Shell, but had no policy for the `[skills]` section. The source-specific validator
  returned early for every non-source path, so the generic overlay editor wrote the field locally
  instead of reaching the strict settings API decoder that already enforced the contract.
- **Fix commit:** `b346b36d4`
- **Regression tests:** `TestWriteScopeValidationAndTargetScope/Should reject non-source skill
  fields at workspace scope` owns the policy; `TestConfigSkillSourceValuesAndValidationRendering/
  Should reject a non-source workspace skill field without writing a file` owns the structured CLI
  error and no-mutation guarantee.

## Verification

- **Retested:** 2026-08-25, same persona/journey · **Report:** docs/qa/reports/2026-08-25-skill-sources.md
- **Result:** the rebuilt CLI exits 1 with `workspace_scope_field_forbidden` and `field:
  poll_interval`; SHA-256 of the workspace config is identical before and after, and the field is
  absent. Evidence: `<lab>/qa-artifacts/qa/policy-workspace-field-fixed.json`.
