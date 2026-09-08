---
name: eng-consolidate-test-suites
description: Use when adding, moving, reviewing, or auditing tests in Compozy to identify the invariant, owning layer, and canonical suite before changing coverage. Do not use as a replacement for framework-specific testing skills or final verification gates.
---

# Consolidate Test Suites

Before changing a test, name its invariant, owning layer, and canonical suite in one concise working note. Reuse that decision across task notes and other skills; no separate completion form is needed.

- Prefer the lowest layer that proves observable behavior, a public contract, security/concurrency boundary, or data invariant against its real owner. Use `references/test-placement-rules.md` when placement is unclear.
- Search existing suites with `rg --files` and a focused feature/API search. Extend a suitable existing file. Create a new one only when none fits or repository conventions require it; record why.
- Multiple layers are justified only when each proves a distinct failure mode. Do not mirror the same invariant across layers or add coverage to meet a quota.
- Prose, CSS values, snapshots, generated output, config, and file-existence assertions need an actual product/operator artifact contract without a stronger owning check. Prefer behavior, lint, codegen, accessibility, or visual evidence where those own the concern.
- A template requiring tests requires a test decision, not automatic new unit and integration suites. If no testable invariant is justified, continue the requested work with the appropriate existing or manual check.

For a bug, reproduce it and extend the canonical regression case when coverage is missing. Repair production regressions without weakening assertions. Remove redundant tests only when test cleanup is authorized. Report the meaningful verification result with the requested deliverable rather than repeating the placement note in every output.
