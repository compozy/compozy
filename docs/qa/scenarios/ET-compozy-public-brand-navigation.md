---
id: ET-compozy-public-brand-navigation
area: ET
title: Navigate the public Compozy brand and launch post
persona: Ada
journey: J-evaluate-compozy-beta
expected: The public site, launch post, metadata, OpenGraph assets, sitemap, robots, RSS, llms output, and authored runtime guidance identify Compozy at https://compozy.com; the former launch-post slug redirects permanently to the Compozy slug; no active page points at agh.network.
entry_points: https://compozy.com; https://compozy.com/blog/introducing-compozy-the-first-agent-network-protocol; https://compozy.com/blog/introducing-agh-the-first-agent-network-protocol
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: ET-053
---

QA impact 2026-07-26: the public brand origin and launch-post topology changed. Planning flag
only; the next QA cycle owns browser and metadata retesting.

QA impact 2026-07-27: Task 11 replaced the launch-post narrative and landing/OG thesis without
changing the sole permanent same-domain slug redirect. The scenario remains `untested`; the next QA
cycle owns route, metadata, RSS, search, and social-card evidence.
