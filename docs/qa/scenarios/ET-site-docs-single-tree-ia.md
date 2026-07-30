---
id: ET-site-docs-single-tree-ia
area: ET
title: Single /docs tree hard-cuts /runtime and /protocol with journey groups
persona: Dora
journey: J-evaluate-compozy-beta
expected: The top nav reads Home · Docs · Marketplace · Blog · Changelog. /docs renders one sidebar with the eight journey groups (Start here, Guides & examples, Core concepts, Automation, Compozy Network, Extensibility, Operations, Reference) and no sub-tab bar; the protocol spec nests under Network as a collapsed "Protocol spec (compozy-network/v0)" folder with its internal groups intact; legacy /runtime and /protocol URLs return 404 while canonical /docs pages resolve directly.
entry_points: compozy.com /docs; /docs/loops; /docs/network/protocol/envelope; legacy /runtime/*, /protocol/*
qa_status: pass
bug_ids: BUG-20260730-docs-index-invalid-hydration
fix_status: fixed
retest_status: pass
fix_commits: working-tree
evidence: /Users/pedronauck/dev/qa-labs/compozy-site-improvs-deep-review-20260730-024918-833208-lab/qa-artifacts/qa/visual-contract/deep-review-remediation/vc01-docs-landing; /Users/pedronauck/dev/qa-labs/compozy-site-improvs-deep-review-20260730-024918-833208-lab/qa-artifacts/qa/visual-contract/deep-review-remediation/raw/implementation/protocol-envelope.png
last_report: docs/qa/reports/2026-07-29-site-improvs-deep-review.md
overlaps: ET-site-docs-sidebar-opendesign; ET-site-docs-first-session; ET-site-docs-search-context
---

Added 2026-07-29 with the site IA restructure (spec `.compozy/tasks/site-docs-ia/_spec.md`
Phase A): the runtime/protocol trees merged into `content/docs/`, code-synthesized navigation
was replaced by declarative root `meta.json` groups with a build-time completeness assertion,
the docs-header sub-tab bar was deleted (D3). The greenfield hard cut now removes the temporary
redirect bridge, so legacy `/runtime` and `/protocol` URLs must return 404. Future cycles re-walk
after sidebar or route changes.

QA impact 2026-07-29: the `/docs` landing was rebuilt to the OpenDesign reference
(`docs/design/opendesign/site/site-docs-landing.html`) — a four-card audience path grid with iconed
links, a group index replacing the markdown table, and the page heading now reading
`CompozyOS documentation` while the sidebar row stays `Overview` and carries a BookOpen icon. The
`Guides & examples` group gained an `Examples` folder between Guides and Use cases. Re-walk the eight
groups, the landing's own links, and the legacy 404 set; reset to `untested` because the landing and the
root sidebar row both changed.
