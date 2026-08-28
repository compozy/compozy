---
id: RT-hermes-live-model-readiness
area: RT
title: Discover Hermes models only after a real ACP handshake
persona: Ada
journey: J-20
expected: Hermes contributes live model and configuration descriptors only after a bounded ACP handshake; `hermes acp --check` remains an install/import preflight, and handshake or Hermes-owned MCP setup failures surface redacted actionable status without erasing stale rows.
entry_points: compozy provider models list hermes --all; compozy provider models refresh hermes; compozy provider models status hermes; HTTP/UDS/native model-catalog surfaces
qa_status: pass
bug_ids:
fix_status:
retest_status: pass
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-acp-runtime-catalog-20260828-004625-083662-lab/qa-artifacts/qa/evidence/hermes-live-readiness.json
last_report: docs/qa/reports/2026-08-27-acp-runtime-catalog.md
overlaps: MS-live-model-release-refresh; MS-043
---

Added for the ACP runtime catalog rebuild. Readiness means protocol handshake success, not only a
zero exit from the Hermes preflight command.

QA 2026-08-27: the real `hermes acp` handshake published 11 live models. With the configured command
and catalog fingerprint unchanged, an isolated failing executable produced an actionable ACP
disconnect error while preserving all 11 rows as `available_stale`. Restoring the real executable
and forcing refresh returned the same 11 rows to `available_live`.
