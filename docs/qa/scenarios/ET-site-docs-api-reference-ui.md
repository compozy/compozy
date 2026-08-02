---
id: ET-site-docs-api-reference-ui
area: ET
title: API Reference pages render styled OpenAPI components with full sidebar icon coverage
persona: Dora
journey: J-evaluate-compozy-beta
expected: On /docs/api/* every surviving OpenAPI tag shows a Lucide icon and styled two-column operations; Extension inventory, preview, secrets, and digest-confirm fields render, while the Bundle tag and operations are absent with no broken sidebar entry.
entry_points: compozy.com /docs/api/extensions; /docs/api/marketplace; /docs/api/sessions; /docs/api/settings; retired /docs/api/bundles
qa_status: pass
bug_ids: BUG-20260802-site-topbar-client-boundary
fix_status: fixed
retest_status: pass
fix_commits: a817e37
evidence: /Users/pedronauck/dev/qa-labs/compozy-site-improvs-deep-review-20260730-024918-833208-lab/qa-artifacts/qa/visual-contract/deep-review-remediation/raw/implementation/api-reference.png;/Users/pedronauck/dev/qa-labs/compozy-devtool-oss-launch-20260802-195112-911343-lab/qa-artifacts/qa
last_report: docs/qa/reports/2026-08-02-bundles-removal.md
overlaps: ET-site-docs-sidebar-opendesign; ET-site-docs-typography-opendesign; ET-site-docs-masthead-opendesign
---

QA impact 2026-07-29: fixed the API Reference UI — the missing
`fumadocs-openapi/css/preset.css` import left every OpenAPI component
unstyled, and the tag→icon pipeline missed 8 tags plus 2 dangling icon names.
Icons now come from the shared `lib/docs-icons.ts` registry (invariant-tested);
layout comes from site-owned renderPageLayout/renderOperationLayout wrappers +
`.site-api-page` CSS. Code samples reduced to cURL/JavaScript/Go/Python. The
next QA cycle owns visual verification across dense pages (sessions, tasks,
settings) at desktop and stacked (<896px container) widths, accordion
expand/collapse, and anchor deep-links to operations.

QA impact 2026-08-02: the deleted API tag left the generated sidebar. Reset to verify icon coverage
and layout across the surviving OpenAPI tags.
