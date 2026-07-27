---
id: ET-compozy-public-brand-navigation
area: ET
title: Navigate the public Compozy brand and launch post
persona: Ada
journey: J-validate-compozy-hard-cut
expected: The public site, launch post, metadata, OpenGraph assets, sitemap, robots, RSS, llms output, and authored runtime guidance identify Compozy at https://compozy.com; the former launch-post slug redirects permanently to the Compozy slug; no active page points at agh.network.
entry_points: local packages/site root and metadata outputs; local /blog/introducing-compozy-the-first-agent-network-protocol; local /blog/introducing-agh-the-first-agent-network-protocol redirect; canonical https://compozy.com declarations
qa_status: pass
bug_ids: BUG-20260727-runtime-legacy-identity
fix_status: fixed
retest_status: pass
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-compozy-migration-beta-20260727-135201-116083-lab/qa-artifacts/qa/browser/compozy-home.png; /Users/pedronauck/dev/qa-labs/compozy-compozy-migration-beta-20260727-135201-116083-lab/qa-artifacts/qa/browser/compozy-dev-home.png; /Users/pedronauck/dev/qa-labs/compozy-compozy-migration-beta-20260727-135201-116083-lab/qa-artifacts/qa/gate-test-e2e-web-final-2.log
last_report: docs/qa/reports/2026-07-27-devtool-oss-launch.md
overlaps: ET-053
---

QA impact 2026-07-26: the public brand origin and launch-post topology changed. Planning flag
only; the next QA cycle owns browser and metadata retesting.

QA impact 2026-07-27: Task 11 replaced the launch-post narrative and landing/OG thesis without
changing the sole permanent same-domain slug redirect. The scenario remains `untested`; the next QA
cycle owns route, metadata, RSS, search, and social-card evidence.
