---
id: MS-settings-roles-panel
area: MS
title: Configure background roles in Settings
persona: Dora
journey: J-route-background-work
expected: Settings → Roles renders the six roles in product order with truthful builtin, inherit, off, provenance, fallback, and diagnostic states; a valid edit applies Live, survives reload, and never exposes virtual builtins in the Agents fleet.
entry_points: Web /settings/roles; GET/PATCH /api/settings/roles; GET /api/roles
qa_status: untested
bug_ids: BUG-20260724-inherited-role-provider-resolution
fix_status: fixed
retest_status: pending
fix_commits: a9a8fcad63f4354505e4c9a0701a6d0f559cc991
evidence: /Users/pedronauck/dev/qa-labs/agh-agent-roles-devtool-oss-launch-20260724-094737-758561-lab/qa-artifacts/qa/screenshots/settings-roles-loaded.png; /Users/pedronauck/dev/qa-labs/agh-agent-roles-devtool-oss-launch-20260724-094737-758561-lab/qa-artifacts/qa/screenshots/settings-roles-ghost-diagnostic.png; /Users/pedronauck/dev/qa-labs/agh-agent-roles-devtool-oss-launch-20260724-094737-758561-lab/qa-artifacts/qa/screenshots/settings-roles-compact.png; /Users/pedronauck/dev/qa-labs/agh-agent-roles-devtool-oss-launch-20260724-094737-758561-lab/qa-artifacts/qa/ui-settings-roles-after-save.json; /Users/pedronauck/dev/qa-labs/agh-agent-roles-devtool-oss-launch-20260724-094737-758561-lab/qa-artifacts/qa/inherit-provider-fix-parent-after-title.json
last_report: docs/qa/reports/2026-07-24-agent-roles.md
overlaps: MS-background-role-routing;MS-inspect-background-role-routing;MS-026;RT-reserved-builtin-agent-names
---

QA impact 2026-07-23: the Roles settings surface is new. Planning flag only; the next QA cycle owns
desktop and compact viewport behavior, accessibility, validation recovery, Live save/reload, and
truthful catalog separation.

Planning 2026-07-24 (Task 05): persona reconciled Bruno → Dora (Settings pages are Dora's secondary
surface; the panel is runtime administration). Session charter: CH-settings-roles-live-truth, which
also settles MS-026's retained-memory-controls check on the adjacent Memory settings page.

QA 2026-07-24: the real panel rendered six roles in product order with truthful states/provenance, applied and reloaded a model plus fallback live, discarded a dirty draft on navigation, retained/focused invalid fallback input, cleared a repaired ghost diagnostic, hid builtins from the fleet, and survived a 900x700 viewport without horizontal overflow. The saved model-only inherited route initially exposed BUG-20260724-inherited-role-provider-resolution; its rebuilt-daemon retest generated the title on the primary route with no fallback.

QA impact 2026-07-24 (final review remediation): incomplete role rosters now fail closed and invalid
numeric fields receive focus when they block save. The next QA cycle owns these corrected error and
recovery states.

QA impact 2026-07-25 (Roles panel redesign): the panel is now a collapsed accordion — one disclosure
row per role carrying name, resolution line, projected route chip, BUILTIN/INHERIT pill and the
enabled switch (the OFF pill is gone; the switch carries that state). Provider, model and reasoning
effort are one `RuntimeSelector` with a Clear override action instead of three free-text/select
fields, the agent is picked through `AgentCommandSelect` with a "Role default" entry, and a row
force-opens on a daemon diagnostic or a save-blocking validation error. Fallback routes are one
selector per entry with a single "Choose a provider and model." error. The next QA cycle owns
expand/collapse behavior, keyboard reach across rows, runtime-selector commit inside the settings
scroll container, out-of-catalog agent display, and Live save/reload through the new controls.

QA impact 2026-07-25 (deep-review remediation): role validation now reports invalid fields against
the owning role consistently. Flag only; the next QA cycle owns save-blocking focus and recovery.
