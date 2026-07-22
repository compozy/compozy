---
provider: manual
pr:
round: 2
round_created_at: 2026-07-22T15:39:03Z
status: resolved
file: skills/cy-create-techspec/references/techspec-template.md
line: 82
severity: high
author: claude-code
provider_ref:
---

# Issue 004: Observability requirements can publish without a contract

## Review Comment

The TechSpec observability section asks only for generic metrics, log fields, and alerts. That is insufficient for requirements such as “emit an operational event”: event identity, payload privacy, outcomes, delivery semantics, and sink-failure behavior can remain undefined, yet task generation can still succeed.

Require a machine-readable contract for every mandated event: name, trigger, required/optional fields, per-field privacy classification, request/correlation ID sources, actor/resource identifiers, allowed outcomes, success/rejection/replay/stale-command behavior, delivery semantics, and sink-failure behavior. Missing fields must create a blocking product decision instead of inferred values. Generate tests for required fields and outcome enums, forbidden payload fields, replay/rejection behavior, and event-sink failures.

## Triage

- Decision: `VALID`
- Notes: `Monitoring and Observability` currently permits a TechSpec to describe events with only generic log-field prose. It defines neither a machine-readable event schema nor a completeness gate, while `tests-template.md` can derive concrete message-contract cases only from details present in the TechSpec. Consequently, event identity, payload privacy, outcome behavior, delivery guarantees, and sink-failure policy can all remain unspecified without blocking publication.
- Root cause: the template treats operational events as optional observability narrative instead of design contracts whose unresolved fields are blocking product decisions.
- Fix: require one YAML contract per mandated event, enumerate every required identity, payload, privacy, outcome, delivery, and failure-policy field, prohibit inference when any value is unresolved, and require matching `_tests.md` cases. Add a bundled-skill regression test that pins these requirements in the shipped template.
- Verification: the focused regression test passed all 21 contract assertions. The full `make verify` pipeline passed with 5,199 Go tests, zero lint issues, successful builds and extension checks, and 7 Playwright tests.
- Verification environment: the generated review-worktree path exceeded macOS's Unix-socket path limit during the first E2E attempt. Scoping an official short `COMPOZY_HOME` override to Make's `BUNCMD` kept Go test isolation unchanged and allowed the unmodified E2E suite to exercise the same daemon behavior successfully.
