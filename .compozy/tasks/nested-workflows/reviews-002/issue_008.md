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

# Issue 008: Atomicity claims lack failure-injection coverage

## Review Comment

The test contract requests generic error and failure-path coverage but has no atomicity rule pack. A task may claim that writes A, B, and C are atomic while owning only a happy-path test, which cannot prove rollback at intermediate boundaries or commit failure.

Detect multi-write atomicity requirements and generate failure cases for A failing, A succeeding then B failing, A and B succeeding then C failing, and transaction commit failing. Use repository failures, database constraints, transaction callbacks, or dedicated test seams rather than production-only flags. Require assertions that domain, ledger, and event state roll back together when specified, nested/bound transaction scopes preserve rollback, retry-after-rollback behavior is defined, and every atomicity requirement owns at least one failure-injection test.

## Triage

- Decision: `VALID`
- Notes: `tests-template.md` requires generic component error-path and interrupted-flow coverage, but it does not detect atomic multi-write requirements or require tests at each write and commit boundary. A generated `_tests.md` can therefore claim atomicity while testing only successful execution. Add an atomicity rule pack under Coverage Demands that enumerates failure-injection cases, requires joint rollback assertions for every specified state surface, preserves nested or bound transaction semantics, defines retry-after-rollback behavior, and forbids production-only test flags.
- Resolution: Added the atomicity rule pack and a bundled-skill regression test covering every required failure boundary, injection mechanism, rollback surface, transaction scope, retry rule, and owning test ID. `make verify` passed after the implementation.
