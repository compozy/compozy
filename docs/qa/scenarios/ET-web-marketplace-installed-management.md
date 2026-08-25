---
id: ET-web-marketplace-installed-management
area: ET
title: Manage every installed Marketplace kind
persona: Bruno
journey: J-marketplace-acquisition
expected: Each kind opens in Installed scope with `tab` omitted and exposes only daemon-backed controls: skill content, shadows, enable and update; extension kit inventory, lifecycle, environment, diagnostics and provenance; MCP creation and exact-scope configuration, status and authorization.
entry_points: /marketplace/skills; /marketplace/mcps; /marketplace/extensions
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: docs/qa/evidence/2026-08-10-loop-browser-runtime-closeout/marketplace-skill-disabled.png; docs/qa/evidence/2026-08-10-loop-browser-runtime-closeout/extension-spec-cycle-trust.png; docs/qa/evidence/2026-08-10-loop-browser-runtime-closeout/marketplace-mcp-playwright-installed.png
last_report: docs/qa/reports/2026-08-10-loop-browser-runtime-closeout.md
overlaps: ET-web-extensions-manage; ET-web-extension-detail; ET-web-mcp-status-matrix
---

Skipped in the 2026-07-30 MCP 2026/catalog-v2 closeout: retained capture covers MCP installed UI only, not every Marketplace kind and its management actions.

Use one installed item of each surviving kind and confirm detail refresh preserves the same runtime
truth.

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

QA impact 2026-07-18: installed mutations publish pending state by the exact MCP owner, skill, or
extension identity, so sibling cards remain independently operable and explicit global detail links
keep every action out of the active workspace scope.

QA impact 2026-07-18: consolidated installed details preserve daemon-backed operational context.
Skills show capabilities, recent calls, provenance, and resolver shadows; extensions show state,
health, daemon/PID/uptime, capabilities, actions, environment, diagnostics, provenance, and kit inventory.

QA impact 2026-07-18: switching the active workspace closes any open MCP definition editor before
its captured scope can be saved. Installed Update actions also inherit the exact catalog-entry
pending state, so the initiating card stays disabled until its mutation settles.

QA impact 2026-08-02: the installed Marketplace has exactly three kinds and extension detail owns
kit inventory. Reset for the next QA cycle.

QA impact 2026-08-07: marketplace detail pages were rebuilt to the OpenDesign marketplace contract —
kind-specific content is now the body (skill readme + calls/content/resolution; MCP
authorization/connection/tools; extension kit/access/environment/diagnostics/logs) and the rail
holds short collapsible property cards. Update and Authorize are OS-head primary actions only; the
rail Manage card keeps the enable switch, overflow (Provenance/Remove), and trust badges. Reset to
untested.

QA impact 2026-08-10: an installed Skill now labels its switch from daemon truth (`Enabled` or
`Disabled`) after mutation, and extension Update opens an update-specific trust confirmation instead
of install copy. This scenario remains untested for a fresh Skill toggle, Extension update-confirm,
refresh, and exact-scope management walk.

QA completion: blocked-verify 2026-08-10 — Bruno toggled the bundled `compozy` skill, confirmed the `Disabled` label
survived reload, and restored `Enabled`; disabled and restored `spec-cycle` while runtime health and
official trust stayed truthful; then installed Playwright in workspace scope and inspected its
installed stdio detail. The isolated catalog exposed no extension update candidate, so the remaining
update branch could not be walked without fabricated state; its detail contract is tracked by
ET-web-extension-detail.

QA impact 2026-08-25 (skill sources): reset because the installed skill detail gained a new daemon-backed control surface. The Exposures card now sits beside skill content, shadows, enable, and update, and origin attribution renders on the installed rows. Bundled skills must show no Exposures card at all — absent, not disabled — so the "only daemon-backed controls" claim in this scenario's promise has to be re-settled against the new card. Rides along in `CH-skill-expose-web-repair`.
