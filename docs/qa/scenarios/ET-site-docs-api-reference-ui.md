---
id: ET-site-docs-api-reference-ui
area: ET
title: API Reference pages render styled OpenAPI components with full sidebar icon coverage
persona: Dora
journey: J-evaluate-compozy-beta
expected: On /docs/api/* every surviving OpenAPI tag renders the docs API resource template (endpoint index, then styled operations beside request samples); Extension inventory, preview, secrets, and digest-confirm fields render, while the Bundle tag and operations are absent with no broken sidebar entry. Sidebar icons follow the docs draft.
entry_points: compozy.com /docs/api/extensions; /docs/api/marketplace; /docs/api/sessions; /docs/api/settings; retired /docs/api/bundles
qa_status: pass
bug_ids: BUG-20260802-site-topbar-client-boundary
fix_status: fixed
retest_status: pass
fix_commits: a817e37
evidence: .compozy/tasks/site-redesign/qa/R4/ET-site-docs-reference-pages/walk.jsonl; .compozy/tasks/site-redesign/qa/R4/ET-site-docs-reference-pages/api-1440-site.jpg; .compozy/tasks/site-redesign/qa/R4/ET-site-docs-reference-pages/api-390-site.jpg
last_report: docs/qa/reports/2026-11-10-site-redesign-r4.md
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

QA impact 2026-11-10 (site redesign): /docs/api/<tag> now renders through the docs draft's API resource template (endpoint index, then each endpoint beside its request samples and response example) over fumadocs-openapi. The sidebar shows only the current section and follows the docs draft, which owns whether rows carry icons. Current template coverage is ET-site-docs-reference-pages; the Bundle-absence leg still applies. Status reset to untested pending a fresh walk.

QA result 2026-11-10 (R4 walk, `next start`, 1440 and 390, dark): pass on the updated expectation. All 31 surviving tags render the API resource template with styled operations; `/docs/api/extensions` carries the inventory, preview, secrets and digest-confirm operations and fields; `/docs/api/bundles/` answers 404 and no Bundle entry is in the sidebar or the API index.
