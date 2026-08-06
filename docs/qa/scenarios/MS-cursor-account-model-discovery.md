---
id: MS-cursor-account-model-discovery
area: MS
title: Discover the signed-in Cursor account model catalog
persona: Ada
journey: J-20
expected: Before any session, the first Cursor catalog list invokes the native signed-in CLI once, preserves its exact model IDs across CLI, HTTP, and UDS, records source status, and reruns only on explicit refresh.
entry_points: compozy provider models list cursor --all; GET /api/model-catalog/providers/cursor/models?view=all; UDS model-catalog route; compozy provider models status cursor; compozy provider models refresh cursor
qa_status: pass
bug_ids:
fix_status:
retest_status: pass
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-issue-312-cursor-models-20260805-200518-943803-lab/qa-artifacts/qa/issue-312-evidence.md;/Users/pedronauck/dev/qa-labs/compozy-issue-312-review-remediation-final-20260805-230015-520918-lab/qa-artifacts/qa/issue-312-review-evidence.md
last_report: docs/qa/reports/2026-08-05-issue-312-review-remediation.md
overlaps: MS-042;MS-043;MS-044;MS-055
---

The account model IDs come from Cursor itself. Curated metadata may enrich a discovered row but never removes a real account row or becomes a runtime allowlist.

QA impact 2026-08-05 (review remediation): bootstrap ownership and catalog generation publication
changed. Reset for a concurrent first-read and cross-surface lifecycle walk.

QA 2026-08-05 (review remediation): simultaneous first CLI/UDS and HTTP reads returned the same 193
Cursor account rows and one persisted live generation; cached read and explicit refresh behaved as specified.
