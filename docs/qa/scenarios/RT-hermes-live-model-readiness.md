---
id: RT-hermes-live-model-readiness
area: RT
title: Discover Hermes models only after a real ACP handshake
persona: Ada
journey: J-20
expected: Hermes contributes live model and configuration descriptors only after a bounded ACP handshake; `hermes acp --check` remains an install/import preflight, and handshake or Hermes-owned MCP setup failures surface redacted actionable status without erasing stale rows.
entry_points: compozy provider models list hermes --all; compozy provider models refresh hermes; compozy provider models status hermes; HTTP/UDS/native model-catalog surfaces
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: MS-live-model-release-refresh; MS-043
---

Added for the ACP runtime catalog rebuild. Readiness means protocol handshake success, not only a
zero exit from the Hermes preflight command.
