---
id: ET-web-catalog-navigation
area: ET
title: Navigate the desktop app registry
persona: Bruno
journey: J-marketplace-acquisition
expected: The dock and command palette expose the canonical desktop app registry without duplicate windows; the registry labels read Bridges (route `/bridges`) and Sandbox (route `/sandbox`), and every dock tooltip, Go menu entry, window title, and palette hit shows the same label from the one descriptor; Marketplace owns its Extensions, Skills, and MCP routes; Sandbox and Vault stay in the final dock group; Settings opens from the menubar cog; focusing a child route preserves the owning app window.
entry_points: web desktop dock; command palette; settings cog; Catalog and System destinations
qa_status: untested
bug_ids: BUG-20260802-retired-marketplace-kind-alias
fix_status: fixed
retest_status: pass
fix_commits: 7701a3f
evidence: /Users/pedronauck/dev/qa-labs/compozy-marketplace-task11-final-20260715-20260716-011529-818379-lab/qa-artifacts/qa/notes/marketplace-under-minute.json;/Users/pedronauck/dev/qa-labs/compozy-qa-et-current-source-20260730-061655-910372-lab/qa-artifacts/qa;/Users/pedronauck/dev/qa-labs/compozy-devtool-oss-launch-20260802-195112-911343-lab/qa-artifacts/qa;docs/qa/reports/2026-08-20-ui-normies-retry.md
last_report: docs/qa/reports/2026-08-20-ui-normies-retry.md
overlaps: ET-web-marketplace-landing-browse; ET-web-extensions-manage
---

2026-08-20 retry: skipped by explicit user instruction. No current registry-navigation pass is claimed.

Planning note: Marketplace nested-route active-indicator verification remains pending; no bug fix is associated with this scenario.

Added by marketplace Task 07. Verify the exact D13 ordering without count badges.

QA impact 2026-07-16: Marketplace now uses fuzzy route matching so kind and entry detail routes
retain the sidebar active indicator.

QA impact 2026-07-18: Network and Bridges now use the same descendant-aware active matching as
other catalog roots. Verify nested channel/thread and bridge-detail routes retain their parent
sidebar indicator.

QA impact 2026-07-20: OS Shell Task 04 deleted the sidebar and replaced its app-navigation role
with the dock, command palette, and settings cog. Reset to `untested`; later app-port tasks own the
content journeys inside each registered window.

QA impact 2026-08-02: Marketplace navigation now has exactly Extensions, Skills, and MCP. Reset for
the next QA cycle.

QA impact 2026-08-20: the normie-friendly UI foundation pass renamed two registry labels —
`Bridges` → **Connections** and `Sandbox` → **Permissions** (`os/lib/app-catalog.ts:118,132`) — and
swapped the Session icon from `SquareTerminal` to `MessagesSquare` (`:57`). Routes are unchanged
(`/bridges`, `/sandbox`). That label remap was reverted the same day: the product surfaces are
**Bridges** and **Sandbox** again. `expected:` was rewritten to match. Reset because this file is
where the registry labels are asserted.

One array feeds the dock, the Go menu, window titles, and the command palette, so the walk's real
question is consistency: a persona searching the palette for "Bridges" must find the bridges app,
and no surface may still say Connections or Permissions for those two apps. `Loops`, `Jobs`,
`Triggers`, and `Network` were deliberately left alone pending an owner decision on the alias table.
