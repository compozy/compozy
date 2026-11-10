---
id: ET-site-docs-search-context
area: ET
title: Distinguish duplicate documentation results by section
persona: Dora
journey: J-evaluate-compozy-beta
expected: Public documentation search results include URL-derived section breadcrumbs, so duplicate titles such as Sessions identify Core concepts, CLI reference, API reference, or protocol implementation guides without opening each result.
entry_points: compozy.com documentation search; Runtime Sessions; CLI session reference; API Sessions; protocol implementation guides
qa_status: pass
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: .compozy/tasks/site-redesign/qa/R4/ET-site-docs-search-context/walk.jsonl; .compozy/tasks/site-redesign/qa/R4/ET-site-docs-search-context/sessions-1440-site.jpg; .compozy/tasks/site-redesign/qa/R4/ET-site-docs-search-context/sessions-390-site.jpg
last_report: docs/qa/reports/2026-11-10-site-redesign-r4.md
overlaps: ET-compozy-public-brand-navigation
---

QA impact 2026-07-29: documentation search now derives semantic breadcrumbs from route ancestry. The
next QA cycle owns keyboard, visual, and duplicate-title result verification across Runtime,
CLI, API, and Protocol.

QA impact 2026-11-10 (site redesign): search moved to the docs draft's ⌘K palette, which groups results by section and adds a section filter, so duplicate titles stay distinguishable. The old header search input and dialog were deleted. Walk it with ET-site-docs-shell; status reset to untested.

QA result 2026-11-10 (R4 walk, `next start`, 1440 and 390, dark): pass. Searching "sessions" in the ⌘K palette groups results under Core concepts, Automation, Extensibility, Operations and Reference, and each row carries its section path, so the duplicate "Sessions" titles read as "Core concepts", "Reference / API reference" and "Reference / SDK reference". A failed search shows its own error state with Try again (walked with ET-site-docs-shell).
