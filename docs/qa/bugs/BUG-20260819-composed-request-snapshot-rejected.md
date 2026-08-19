# BUG-20260819-composed-request-snapshot-rejected: Composed request schema cannot start

- **Status:** open
- **Impact (user-side):** Blocks-Completion
- **Severity:** Critical · **Priority:** P0
- **Persona Affected:** Bruno
- **Journey Step:** J-supervise-loop-request, create the pending request
- **Scenarios:** LP-answer-typed-request-entities
- **Found:** 2026-08-19 · **Report:** docs/qa/reports/2026-08-19-typed-loop-inputs-remediation.md

## Summary

Bruno could not reach a typed request whose response schema placed an agent entity inside
`allOf`, an array `items` schema, and `oneOf`. The Loop was rejected while its executed definition
snapshot was built.

## Reproduction

- **Charter:** CH-typed-request-entity-answer · **Tour:** Network Tour
- **Environment:** desktop / flaky / en-US, fresh isolated CLI and daemon

1. Publish a Loop with an `ask` node.
2. Put an agent-annotated string inside `expect.allOf[].properties.reviewers.items.oneOf[]`.
3. Start the Loop.

**Expected:** The executed definition snapshot preserves the composed schema and creates a pending
request.
**Actual:** Snapshot hydration adds schema template paths that were absent from the authored
manifest, so the start fails with `manifest key ... added during hydration`.

## Evidence

- Isolated public CLI replay recorded in the report and lab journey log.
- The canonical snapshot regression reproduces the same composed schema path.

## Fix

- **Root cause:** YAML composition arrays remained `[]dsl.Schema` during the first compile, while
  JSON hydration represented them as `[]any`; the template-source walker only traversed the latter.
- **Fix commit:** pending current fix commit
- **Regression test:** `internal/loop/coordinator_snapshot_test.go`

## Verification

Pending a clean persona re-walk through request rejection and acceptance.
