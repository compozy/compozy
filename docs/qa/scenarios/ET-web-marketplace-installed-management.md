---
id: ET-web-marketplace-installed-management
area: ET
title: Manage every installed Marketplace kind
persona: Bruno
journey: J-marketplace-acquisition
expected: Each kind's Installed scope exposes only daemon-backed controls: skill content, shadows, enable and update; extension enable, environment, diagnostics and provenance; arbitrary MCP creation and exact-scope configuration, status and authorization; bundle scope, profile, update and deactivation.
entry_points: /marketplace/skills?tab=installed; /marketplace/mcps?tab=installed; /marketplace/extensions?tab=installed; /marketplace/bundles?tab=installed
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: ET-web-extensions-manage; ET-web-extension-detail; ET-web-mcp-status-matrix
---

Added by the unified Marketplace hard cut. Use one installed item of each kind, including a skill
managed by a bundle, and confirm detail refresh preserves the same runtime truth.

QA impact 2026-07-18: MCP Installed now owns the generic stdio/http/sse editor. Retest global and
workspace creation, editing a global item while a workspace is active, and exact-scope detail links.

QA impact 2026-07-18: installed detail links now carry exact local identity, MCP edits preserve
config versus sidecar ownership, hidden same-scope conflicts block saves, and success feedback names
the write target plus restart lifecycle. Include a local MCP name that collides with a curated ID.

QA impact 2026-07-18: Installed hydrates every catalog page before reporting update state, blocks
partial truth when continuation fails, loads global skills without an active workspace, and retains
installed skill tag search. MCP install toasts open the exact entry, scope, server, and workspace.

QA impact 2026-07-18: MCP labels, detail links, edits, removals, and authorization dialogs now use
`source_metadata.effective_source`, including inherited global definitions returned by a workspace
collection and exact config-versus-sidecar ownership.

QA impact 2026-07-18: an exact `installed_name` wins over another MCP definition sharing the same
catalog entry, while an inherited global row no longer blocks creation of a workspace override with
the same name. Confirm the installed detail and editor target the intended definition.

QA impact 2026-07-18: successful acquisition feedback now runs on the shared motion tokens and
releases its flash state from the animation lifecycle. Confirm the card flashes once without the
former long fallback timing and remains operable afterward.

QA impact 2026-07-18: installed mutations now publish pending state by the exact activation, MCP
owner, skill, or extension identity, so sibling cards remain independently operable. Bundle skills
resolve `installed_from_bundle` as an activation ID, bundle listings join on extension plus bundle
name, and explicit global detail links keep every action out of the active workspace scope.

QA impact 2026-07-18: consolidated installed details preserve daemon-backed operational context.
Skills show capabilities, recent calls, provenance, and resolver shadows; extensions show state,
health, daemon/PID/uptime, capabilities, actions, environment, diagnostics, provenance, and bundles.

QA impact 2026-07-18: switching the active workspace closes any open MCP definition editor before
its captured scope can be saved. Installed Update actions also inherit the exact catalog-entry
pending state, so the initiating card stays disabled until its mutation settles.

QA impact 2026-07-19: rejected skill enable or disable mutations remain visible beside the owning
switch while server truth is restored. Installed bundle Update remains disabled for the full
activation-scoped mutation. Status remains untested; no QA replay ran.
