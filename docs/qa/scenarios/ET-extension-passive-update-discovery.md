---
id: ET-extension-passive-update-discovery
area: ET
title: Discover extension updates without flags
persona: Ada
journey: J-extension-distribution
expected: Installed extensions advertise a newer remote version in list and search human output and structured fields without an update-check flag, while a degraded discovery source never blocks local inventory.
entry_points: compozy extension list; compozy extension search; GET /api/extensions; GET /api/extensions/search
qa_status: blocked-verify
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-ext-improvs-final-20260729-230047-267985-lab/qa-artifacts/qa/extension-charters.json;/Users/pedronauck/dev/qa-labs/compozy-qa-et-current-source-20260730-061655-910372-lab/qa-artifacts/qa
last_report: docs/qa/reports/2026-07-28-untested-full.md
overlaps: ET-015; ET-016; ET-019
---

Added by ext-improvs Task 07. Exercise both a healthy catalog and one degraded source, then compare
the human Update column with `update_available`, `installed_version`, and `remote_version` in the
structured responses.

QA impact 2026-07-29: GitHub repository discovery no longer projects `default_branch` as an
extension release version. Re-run the search projection against a repository whose default branch
is `main` and confirm the result does not claim that branch name as `remote_version` or derive a
false update comparison from it.

Marketplace hard-cut scope 2026-09-13: catalog update badges join only persisted (source_ref, entry_id). A direct GitHub acquisition must not acquire catalog identity from a matching name or repository. The browser update journey installs a real curated v0.1.0, publishes v0.2.0 in the fixture feed, refreshes through the public API, and verifies shelf/detail/update/persisted state. Existing direct extension update commands and release discovery remain covered separately.

Installed-detail coverage: open the catalog update through its persisted installed name, accept trust, verify the new version through the API and reopened detail, and verify Update disappears from Installed and detail. Installed cards follow the approved compact layout; version verification belongs to the detail. An unavailable catalog must leave actual installed contents accessible.
