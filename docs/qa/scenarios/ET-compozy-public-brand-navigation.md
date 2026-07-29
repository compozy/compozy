---
id: ET-compozy-public-brand-navigation
area: ET
title: Navigate the public Compozy brand and launch post
persona: Ada
journey: J-validate-compozy-hard-cut
expected: The public site, launch post, metadata, OpenGraph assets, sitemap, robots, RSS, llms output, and authored runtime guidance identify Compozy at https://compozy.com; no compatibility redirect or active page points at the retired site.
entry_points: local packages/site root and metadata outputs; local /blog/introducing-compozyos; canonical https://compozy.com declarations
qa_status: untested
bug_ids: BUG-20260727-runtime-legacy-identity
fix_status: fixed
retest_status: pending
fix_commits: e4df8634
evidence: /Users/pedronauck/dev/qa-labs/compozy-compozy-migration-beta-20260727-135201-116083-lab/qa-artifacts/qa/browser/compozy-home.png; /Users/pedronauck/dev/qa-labs/compozy-compozy-migration-beta-20260727-135201-116083-lab/qa-artifacts/qa/browser/compozy-dev-home.png; /Users/pedronauck/dev/qa-labs/compozy-compozy-migration-beta-20260727-135201-116083-lab/qa-artifacts/qa/gate-test-e2e-web-final-2.log
last_report: docs/qa/reports/2026-07-27-devtool-oss-launch.md
overlaps: ET-053
---

QA impact 2026-07-26: the public brand origin and launch-post topology changed. Planning flag
only; the next QA cycle owns browser and metadata retesting.

QA impact 2026-07-27: Task 11 replaced the launch-post narrative and landing/OG thesis without
changing the sole permanent same-domain slug redirect. The scenario remains `untested`; the next QA
cycle owns route, metadata, RSS, search, and social-card evidence.

QA impact 2026-07-27: the final identity hard cut removed the retired launch-post compatibility
redirect. The next QA cycle owns route and metadata verification.

QA impact 2026-07-29: the launch URL now names CompozyOS directly, and the homepage, blog metadata,
search, RSS, OpenGraph, and internal links must expose `/blog/introducing-compozyos` with no old
Network-first route. The scenario remains `untested`.
