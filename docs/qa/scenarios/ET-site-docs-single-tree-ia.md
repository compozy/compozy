---
id: ET-site-docs-single-tree-ia
area: ET
title: Single /docs tree replaces /runtime + /protocol with journey groups and 301 bridges
persona: Dora
journey: J-evaluate-compozy-beta
expected: The top nav reads Home · Docs · Marketplace · Blog · Changelog. /docs renders one sidebar with the eight journey groups (Start here, Guides & examples, Core concepts, Automation, Compozy Network, Extensibility, Operations, Reference) and no sub-tab bar; the protocol spec nests under Network as a collapsed "Protocol spec (compozy-network/v0)" folder with its internal groups intact; every legacy /runtime and /protocol URL 301s (after the trailing-slash 308) to a resolving /docs page, including /runtime/core/network/protocol → /docs/network/protocol-model.
entry_points: compozy.com /docs; /docs/loops; /docs/network/protocol/envelope; legacy /runtime/*, /protocol/*
qa_status: pass
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: 2026-07-29 walk on a local `next build` + `next start :4598`. All routes returned 200: /docs/, /docs/examples/ and its five wave-one pages, /marketplace/, /marketplace/{skills,mcp,extensions}/, /marketplace/bridges/, /marketplace/bundled/dev-cycle/, /marketplace/mcp/context7/, and the bridge setup guides. Visual-contract bundles VC-01..VC-05 under .compozy/tasks/site-docs-ia/evidence/visual/gap-closure/ validate PASS with 0 blocking divergences. Landing capture VC-04 shows the rebuilt path grid and group index, the sidebar Overview row with its BookOpen icon, and Examples between Guides and Use cases; the eight groups and the redirect map are unchanged from the prior passing walk.
last_report:
overlaps: ET-site-docs-sidebar-opendesign; ET-site-docs-first-session; ET-site-docs-search-context
---

Added 2026-07-29 with the site IA restructure (spec `.compozy/tasks/site-docs-ia/_spec.md`
Phase A): the runtime/protocol trees merged into `content/docs/`, code-synthesized navigation
was replaced by declarative root `meta.json` groups with a build-time completeness assertion,
the docs-header sub-tab bar was deleted (D3), and `next.config.mjs` carries the sanctioned 301
bridge (D7, scheduled delete one stable release cycle after Phase A ships). Future cycles
re-walk after sidebar or redirect changes.

QA impact 2026-07-29: the `/docs` landing was rebuilt to the OpenDesign reference
(`docs/design/opendesign/site/site-docs-landing.html`) — a four-card audience path grid with iconed
links, a group index replacing the markdown table, and the page heading now reading
`CompozyOS documentation` while the sidebar row stays `Overview` and carries a BookOpen icon. The
`Guides & examples` group gained an `Examples` folder between Guides and Use cases. Re-walk the eight
groups, the landing's own links, and the redirect set; reset to `untested` because the landing and the
root sidebar row both changed.
