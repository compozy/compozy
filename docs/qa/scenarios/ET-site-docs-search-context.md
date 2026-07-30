---
id: ET-site-docs-search-context
area: ET
title: Distinguish duplicate documentation results by section
persona: Dora
journey: J-evaluate-compozy-beta
expected: Public documentation search results include URL-derived section breadcrumbs, so duplicate titles such as Sessions identify Core concepts, CLI reference, API reference, or protocol implementation guides without opening each result.
entry_points: compozy.com documentation search; Runtime Sessions; CLI session reference; API Sessions; protocol implementation guides
qa_status: blocked-verify
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-qa-et-current-source-20260730-061655-910372-lab/qa-artifacts/qa
last_report: docs/qa/reports/2026-07-28-untested-full.md
overlaps: ET-compozy-public-brand-navigation
---

QA impact 2026-07-29: documentation search now derives semantic breadcrumbs from route ancestry. The
next QA cycle owns keyboard, visual, and duplicate-title result verification across Runtime,
CLI, API, and Protocol.
