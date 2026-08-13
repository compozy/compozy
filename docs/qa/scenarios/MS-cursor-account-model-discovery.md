---
id: MS-cursor-account-model-discovery
area: MS
title: Discover the signed-in Cursor account model catalog
persona: Ada
journey: J-20
expected: Before any session, the first Cursor catalog list inspects the signed-in `cursor-agent acp` model option once, preserves its exact advertised values across CLI, HTTP, UDS, and native tools, records source status, and reruns only on explicit refresh.
entry_points: compozy provider models list cursor --all; GET /api/model-catalog/providers/cursor/models?view=all; UDS model-catalog route; compozy provider models status cursor; compozy provider models refresh cursor
qa_status: pass
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-issue-389-cursor-model-final-20260813-222525-271707-lab/qa-artifacts/qa/cursor-refresh.json;/Users/pedronauck/dev/qa-labs/compozy-issue-389-cursor-model-final-20260813-222525-271707-lab/qa-artifacts/qa/cursor-catalog.json;/Users/pedronauck/dev/qa-labs/compozy-issue-389-cursor-model-final-20260813-222525-271707-lab/qa-artifacts/qa/cursor-http.json;/Users/pedronauck/dev/qa-labs/compozy-issue-389-cursor-model-final-20260813-222525-271707-lab/qa-artifacts/qa/cursor-native-tool.json;docs/qa/reports/2026-08-13-issue-389-cursor-model.md
last_report: docs/qa/reports/2026-08-13-issue-389-cursor-model.md
overlaps: MS-042;MS-043;MS-044;MS-055
---

The account model IDs come from Cursor itself. Curated metadata may enrich a discovered row but never removes a real account row or becomes a runtime allowlist.

QA impact 2026-08-13: Cursor discovery now reads the exact advertised values from a short-lived ACP
session. Reset for the catalog, explicit-model, and native-default walk; `cursor-agent models` aliases
are not model identities.

QA impact 2026-08-05 (review remediation): bootstrap ownership and catalog generation publication
changed. Reset for a concurrent first-read and cross-surface lifecycle walk.

QA 2026-08-05 (review remediation): simultaneous first CLI/UDS and HTTP reads returned the same 193
Cursor account rows and one persisted live generation; cached read and explicit refresh behaved as specified.
