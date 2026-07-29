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
evidence: 2026-07-29 walk on local `next build` + `next start :4123` — curl probes returned 301 with resolving 200 targets for /runtime/, /runtime/core/sessions/, /runtime/core/network/protocol/ (→ /docs/network/protocol-model/), /runtime/cli-reference/session/, /runtime/api-reference/, /protocol/, /protocol/envelope/, /runtime/guides/debug-a-failed-session/, /runtime/how-to-use-these-docs/, /runtime/migration/; screenshots /tmp/site-shots/{docs-landing,docs-loops,docs-protocol-spec}.png show the five-link nav, eight sidebar groups, no tab bar, and the nested protocol spec folder with Wire format/Delivery/Trust/Build groups.
last_report:
overlaps: ET-site-docs-sidebar-opendesign; ET-site-docs-first-session; ET-site-docs-search-context
---

Added 2026-07-29 with the site IA restructure (spec `.compozy/tasks/site-docs-ia/_spec.md`
Phase A): the runtime/protocol trees merged into `content/docs/`, code-synthesized navigation
was replaced by declarative root `meta.json` groups with a build-time completeness assertion,
the docs-header sub-tab bar was deleted (D3), and `next.config.mjs` carries the sanctioned 301
bridge (D7, scheduled delete one stable release cycle after Phase A ships). Future cycles
re-walk after sidebar or redirect changes.
