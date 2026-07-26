---
id: ET-web-catalog-navigation
area: ET
title: Navigate the desktop app registry
persona: Bruno
journey: J-marketplace-acquisition
expected: The dock and command palette expose the canonical desktop app registry without duplicate windows; Marketplace owns its Extensions, Skills, MCP, and Bundles routes; Sandbox and Vault stay in the final dock group; Settings opens from the menubar cog; focusing a child route preserves the owning app window.
entry_points: web desktop dock; command palette; settings cog; Catalog and System destinations
qa_status: untested
bug_ids:
fix_status:
retest_status: pending Marketplace nested-route active indicator
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/agh-marketplace-task11-final-20260715-20260716-011529-818379-lab/qa-artifacts/qa/notes/marketplace-under-minute.json
last_report: docs/qa/reports/2026-07-15-marketplace.md
overlaps: ET-web-marketplace-landing-browse; ET-web-extensions-manage
---

Added by marketplace Task 07. Verify the exact D13 ordering without count badges.

QA impact 2026-07-16: Marketplace now uses fuzzy route matching so kind and entry detail routes
retain the sidebar active indicator.

QA impact 2026-07-18: Network and Bridges now use the same descendant-aware active matching as
other catalog roots. Verify nested channel/thread and bridge-detail routes retain their parent
sidebar indicator.

QA impact 2026-07-20: OS Shell Task 04 deleted the sidebar and replaced its app-navigation role
with the dock, command palette, and settings cog. Reset to `untested`; later app-port tasks own the
content journeys inside each registered window.
