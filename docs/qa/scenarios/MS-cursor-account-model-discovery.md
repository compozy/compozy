---
id: MS-cursor-account-model-discovery
area: MS
title: Discover Cursor logical models and valid option combinations
persona: Ada
journey: J-20
expected: Before any session, Cursor runs `cursor-agent models`, groups private aliases into logical models including Grok 4.5, Grok 4.6, and Opus 5, exposes valid Reasoning/Fast/Thinking combinations across CLI, HTTP, UDS, native tools, and Web, records source freshness, row count, errors, next refresh, and stale state, then refreshes after TTL or explicit request without exposing transport aliases.
entry_points: compozy provider models list cursor --all; GET /api/model-catalog/providers/cursor/models?view=all; UDS model-catalog route; compozy provider models status cursor; compozy provider models refresh cursor
qa_status: pass
bug_ids:
fix_status:
retest_status: pass
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-issue-389-cursor-model-final-20260813-222525-271707-lab/qa-artifacts/qa/cursor-refresh.json;/Users/pedronauck/dev/qa-labs/compozy-issue-389-cursor-model-final-20260813-222525-271707-lab/qa-artifacts/qa/cursor-catalog.json;/Users/pedronauck/dev/qa-labs/compozy-issue-389-cursor-model-final-20260813-222525-271707-lab/qa-artifacts/qa/cursor-http.json;/Users/pedronauck/dev/qa-labs/compozy-issue-389-cursor-model-final-20260813-222525-271707-lab/qa-artifacts/qa/cursor-native-tool.json;docs/qa/reports/2026-08-13-issue-389-cursor-model.md;/Users/pedronauck/dev/qa-labs/compozy-acp-runtime-catalog-20260828-004625-083662-lab/qa-artifacts/qa/evidence/cursor-models-cli.json;/Users/pedronauck/dev/qa-labs/compozy-acp-runtime-catalog-20260828-004625-083662-lab/qa-artifacts/qa/evidence/cursor-models-http.json;/Users/pedronauck/dev/qa-labs/compozy-acp-runtime-catalog-20260828-004625-083662-lab/qa-artifacts/qa/evidence/cursor-models-native.json;/Users/pedronauck/dev/qa-labs/compozy-acp-runtime-catalog-20260828-004625-083662-lab/qa-artifacts/qa/evidence/cursor-cross-surface-parity.json;/Users/pedronauck/dev/qa-labs/compozy-acp-runtime-catalog-20260828-004625-083662-lab/qa-artifacts/qa/evidence/web-cursor-grok-catalog.png;/Users/pedronauck/dev/qa-labs/compozy-acp-runtime-catalog-20260828-004625-083662-lab/qa-artifacts/qa/evidence/web-opus-5-catalog.png
last_report: docs/qa/reports/2026-08-27-acp-runtime-catalog.md
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

QA impact 2026-08-27: Cursor discovery now uses `cursor-agent models`, projects logical models and
valid option combinations, and keeps launch aliases private. Reset for cross-surface catalog and
five-minute refresh verification.

QA 2026-08-27: live discovery returned 33 logical models before a session. CLI, HTTP, and native-tool
model arrays were byte-identical after stable JSON sorting. Grok 4.5 exposed 6 valid Reasoning/Fast
configurations, Grok 4.6 exposed 8, and Opus 5 exposed 16 plus its typed Thinking option. Web search
showed all three logical models without a private `cursor-*` launch alias.
