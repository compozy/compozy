---
id: SITE-resource-mutation-boundary
area: SITE
title: Resource guide respects service-owned mutation boundaries
persona: Ada
journey: J-agent-marketplace-parity
expected: The desired-state resource guide demonstrates generic optimistic CRUD with a directly mutable registered kind and explains that extension-owned resources change only through the extension lifecycle.
entry_points: compozy.com `/docs/resources`; `PUT /api/resources/:kind/:id`; `compozy extension enable|disable`
qa_status: untested
bug_ids: BUG-20260729-resource-docs-protected-kind
fix_status:
retest_status:
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-qa-site-search-context-20260730-062701-777068-lab/qa-artifacts/qa
last_report: docs/qa/reports/2026-07-28-untested-full.md
overlaps: ET-033
---

story: As an agent developer I can follow the resource guide without being sent through a forbidden mutation path.

QA impact 2026-08-02: the guide now uses `automation.job` for generic optimistic CRUD and names the
extension lifecycle as the owner of packaged resources. Reset for the next QA cycle.
