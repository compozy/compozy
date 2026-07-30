---
id: ET-site-docs-api-reference-ui
area: ET
title: API Reference pages render styled OpenAPI components with full sidebar icon coverage
persona: Dora
journey: J-evaluate-compozy-beta
expected: On /docs/api/* every sidebar entry (all OpenAPI tags — including Marketplace, Bundles, Notifications, Diagnostics, Filesystem, Logs, Support, Providers, Openai, Roles) shows a Lucide icon; operation pages render the fumadocs-openapi two-column layout (content left, sticky cURL/JavaScript/Go/Python usage tabs + response examples right at wide widths), serif operation titles with breathing room above the method/route card, hairline dividers between operations, 15px reference body, compact media-type chips in response accordions, method labels in signal-token colors (GET success, POST info, PUT/PATCH warning, DELETE danger), schema property names in fg (not accent), and fit-content enum "Value in" panels. Unlike prose docs (56.25rem cap), the API article scales with the main column up to 90rem and centers once the cap binds; containers ≥72rem widen the example column to 26rem.
entry_points: compozy.com /docs/api/marketplace; /docs/api/sessions; /docs/api/settings
qa_status: pass
bug_ids:
fix_status:
retest_status: pass
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-site-improvs-deep-review-20260730-024918-833208-lab/qa-artifacts/qa/visual-contract/deep-review-remediation/raw/implementation/api-reference.png
last_report: docs/qa/reports/2026-07-29-site-improvs-deep-review.md
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
