---
id: ET-extension-batch-update-partial-progress
area: ET
title: Preserve per-extension progress when a batch update partially fails
persona: Bruno
journey: J-extension-distribution
expected: One `extension update --all` request updates every eligible extension it can, returns ordered structured outcomes for both successes and failures, and preserves completed updates when another source is unavailable or one artifact fails verification.
entry_points: `compozy extension update --all`; `POST /api/extensions/update`; `compozy__extensions_update`
qa_status: untested
bug_ids:
fix_status:
retest_status: untested
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-ext-improvs-final-20260729-230047-267985-lab/qa-artifacts/qa/extension-charters.json
last_report: docs/qa/reports/2026-07-29-ext-improvs.md
overlaps: ET-019; ET-023
---

Added by ext-improvs Task 05. Planning flag only; no QA session ran.

Seed at least two extensions with distinct source outcomes. Assert HTTP and UDS return the same ordered
`updated` and `failed` items from one daemon batch call, with stable redacted diagnostics for failures.
Repeat in check-only mode and verify it mutates neither registry rows nor runtime activation state.

QA impact 2026-09-13 (marketplace-catalog task03): exercise explicit profile/workspace selection over
CLI, PUT /api/extensions/{name}, POST /api/extensions/update, and the native update tool. `--all`
excludes other scopes/profiles; a named list processes each distinct name rather than only the first.
Preserve prior successes on later failure. A scoped update with new required inputs must refuse before
writes, then restore the old manifest/value/secret on publication failure. Other input cells and every
attachment/creation timestamp remain unchanged. Final09/10 owns live walks; current focused tests
cover actual archive/SQLite/vault transactions, transport mapping and publication lock ownership.
