---
id: SITE-resource-mutation-boundary
area: SITE
title: Resource guide respects service-owned mutation boundaries
persona: Ada
journey: J-agent-marketplace-parity
expected: The desired-state resource guide demonstrates generic optimistic CRUD with a directly mutable registered kind and routes `bundle.activation` mutations through the bundle lifecycle API or CLI.
entry_points: compozy.com `/runtime/core/resources`; `PUT /api/resources/:kind/:id`; `/api/bundles/activations`; `compozy bundle`
qa_status: blocked-verify
bug_ids: BUG-20260729-resource-docs-protected-kind
fix_status:
retest_status:
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-qa-site-search-context-20260730-062701-777068-lab/qa-artifacts/qa
last_report: docs/qa/reports/2026-07-28-untested-full.md
overlaps: ET-033
---

story: As an agent developer I can follow the resource guide without being sent through a forbidden mutation path.

QA impact 2026-07-29: the guide now uses `automation.job` for generic optimistic CRUD and names
the dedicated bundle activation boundary. This new public docs behavior remains untested for the next
QA cycle, per the tracker flag-without-retest rule.
