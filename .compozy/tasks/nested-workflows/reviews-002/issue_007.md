---
provider: manual
pr:
round: 2
round_created_at: 2026-07-22T15:39:03Z
status: resolved
file: skills/cy-create-techspec/references/tests-template.md
line: 24
severity: medium
author: claude-code
provider_ref:
---

# Issue 007: Quantified requirements can lose their verification

## Review Comment

The test contract's coverage matrix has source, behavior, and test-ID columns but no metric schema. Quantities such as fixture size, latency, retries, page size, concurrency, or retention can disappear during decomposition without failing validation. Assigning all existing `_tests.md` IDs is insufficient when no test preserves the original value or threshold.

Extract quantified requirements into structured metrics containing the metric name, target value, measurement method, test environment, category (`correctness`, `capacity`, or `performance`), and owning test ID. Require the assigned test to use the stated quantity or threshold, and require deterministic, reproducible fixture generation. Block generation when any quantitative requirement lacks a verification method or owning test.

## Triage

- Decision: `VALID`
- Notes: The template gated traceability from sources and behaviors to test IDs, but it had no required structure for preserving numeric limits or proving how they would be measured. Added a quantitative-verification table with source, metric, exact target, measurement, environment, category, and owner fields; coverage now blocks incomplete metrics and requires exact threshold use plus deterministic fixtures. A focused bundled-skill contract test protects these requirements. `env -u NO_COLOR make verify` passed with 5,240 Go tests, 7 intentional skips, 7 Playwright tests, zero lint issues, and zero warnings.
