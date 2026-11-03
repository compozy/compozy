---
id: ET-live-skill-source-reload
area: ET
title: Reload configured skill sources without restarting the daemon
persona: Dora
journey: J-absorb-skills-from-other-tools
expected: Source enablement, disablement, replacement, and scan-health changes become visible through skill resources, settings diagnostics, and agent envelopes within two watcher intervals without restarting the daemon
entry_points: PATCH /api/settings/skills over HTTP or UDS; compozy config set skills.sources; compozy skill sources; Settings > Skills sources section at /settings/skills; resources/list; agent session envelope
qa_status: pass
bug_ids: BUG-20260825-skill-source-event-omits-custom-roots
fix_status: fixed
retest_status: pass
fix_commits: e7dffdb74
evidence: /Users/pedronauck/dev/qa-labs/compozy-skill-sources-final-rebased-20260825-20260825-230120-931206-lab/qa-artifacts/qa/skill-sources/live-apply-summary.json;/Users/pedronauck/dev/qa-labs/compozy-skill-sources-final-rebased-20260825-20260825-230120-931206-lab/qa-artifacts/qa/skill-sources/list-after-live-apply.json;/Users/pedronauck/dev/qa-labs/compozy-skill-sources-final-rebased-20260825-20260825-230120-931206-lab/qa-artifacts/qa/skill-sources/logs-skill-cli.json
last_report: docs/qa/reports/2026-08-25-skill-sources.md
overlaps: ET-manage-skill-source-policy
---

Start with preset and custom sources containing distinct skills, then disable, enable, and replace them while the daemon remains running. Confirm removed skills disappear, newly enabled skills appear, workspace inheritance remains isolated, and truncation or skipped-link diagnostics set and clear after the source is repaired. Read each surface from a fresh request and allow no more than two configured watcher intervals for convergence.

QA plan 2026-08-25 (skill sources cycle): re-pointed from the `J-layer-profile-resources` placeholder to `J-absorb-skills-from-other-tools`. Entry points extended with `compozy skill sources` and the Settings route, because the live-apply promise is only settled when the operator-facing meter converges too, not just the resource and envelope reads. Charter: `CH-skill-sources-live-apply`.


QA impact 2026-10-11 (verification in progress): On native Windows, scan a skill tree with at least 64 entries and immediately resolve it repeatedly. Discovery must remain reusable. Edit, rename, add, and remove definitions and retarget links; each real change must invalidate the prior discovery. Evidence and exact verification scope: [startup/cache/notification report](../reports/2026-10-11-startup-cache-notifications.md).

QA impact 2026-11-03 (#1230, unverified): In an isolated daemon lab with two installed extensions, make one extension resource root unresolvable, then add, edit, and remove a separate skill. Confirm registry snapshots advance despite callback failure, unchanged ticks neither re-parse global skills (no repeated `skills: verification warning` or unknown-frontmatter lines) nor repeat registry refreshes, and callback retries back off to at most once per minute. Confirm the healthy extension's agents and skills stay published while the broken extension's resources are withdrawn and the poll-failure line names only the broken extension and profile. Repair the root without another skill edit and confirm the next callback retry republishes the repaired extension. Count unknown-frontmatter warnings with Python: each skill path and field must warn once per process and include the path. Unknown fields remain tolerated metadata with no added runtime semantics. Historical root cause of the incident's missing Compozy-home ancestor remains unproven: only the `open-design` extension projected for the `open-design` profile failed (its default-profile projection succeeded in the same pass), the failing ancestor moved from `~/.compozy` (8 ticks) to `~/.compozy/extensions` (84 ticks) while other home-dir work kept succeeding, and both directories exist now with no symlinks under the extension root.

The extension manager also reuses parsed skill definitions across profile projections when the
SKILL.md and MCP sidecar snapshots are unchanged. Path containment is rechecked before cache reuse;
real edits and sidecar addition/removal invalidate the entry.
