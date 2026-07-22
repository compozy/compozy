---
provider: manual
pr:
round: 2
round_created_at: 2026-07-22T15:39:03Z
status: resolved
file: skills/cy-create-techspec/references/tests-template.md
line: 63
severity: high
author: claude-code
provider_ref:
---

# Issue 003: Idempotency tests ignore authorization changes

## Review Comment

The test contract treats idempotency as a case tag and permissions as a generic failure path, but never composes related requirements. A workflow can separately cover idempotency and authorization while missing the replay path where access changes after the original request, allowing a stored response to expose data under obsolete permissions.

Add cross-requirement detection for idempotency, replay, deduplication, or stored responses combined with authorization, redaction, tenant isolation, or sensitive output. Generate scenarios where the first request succeeds, access is revoked, the same idempotency key is replayed, and current authorization controls the response projection. Require tests proving canonical stored results remain unchanged, current authorization/redaction is rechecked before replay, fingerprint mismatches never replay, and revocation plus relevant grant transitions are covered.

## Triage

- Decision: `VALID`
- Notes: The template treats `idempotency` as an isolated unit-test class and permission denials as a generic failure path. It never requires the test author to detect or compose replay semantics with authorization, redaction, tenant isolation, or sensitive response projection. A generated contract can therefore satisfy both existing bullets while replaying a stored response under stale access. Add a cross-requirement rule that detects this combination and requires transition scenarios for revocation and relevant grants, preservation of the canonical stored result, current-policy authorization/redaction on every replay, and rejection of fingerprint mismatches.
- Verification: The generated review-worktree path exceeds the macOS Unix-socket length used by Playwright's daemon fixture, so the in-place gate fails at socket bind with `invalid argument`. The full unchanged `make verify` gate passed in a short-lived clone after confirming byte-identical copies of both scoped files.
