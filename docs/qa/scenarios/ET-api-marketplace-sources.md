---
id: ET-api-marketplace-sources
area: ET
title: Manage marketplace sources over HTTP and UDS
persona: Ada
journey: J-agent-marketplace-parity
expected: "Both transports enforce privileged mutations and return matching source state, preview, diagnostics and rejection envelopes."
entry_points: "GET/POST /api/marketplace/sources; PATCH/DELETE /api/marketplace/sources/{name}; POST /api/marketplace/sources/{name}/refresh"
qa_status: pass
bug_ids:
fix_status:
retest_status: pass
fix_commits:
evidence: docs/qa/evidence/2026-09-13-marketplace-catalog/parity-http.json
last_report: docs/qa/reports/2026-09-13-marketplace-catalog.md
overlaps:
---

QA 2026-09-13: the current hard-cut contract passed the scoped live/API/browser walks and applicable unchanged owning integration checks. See the dated report for exact evidence and boundaries; historical notes below do not redefine the current catalog.


Compare dry-run/config bytes, Add201, duplicate409, invalid422, preset-delete403 and remove204. Exercise concurrent additions, disabled offline refresh and retained installed names. Re-read sources and extension inventory after each mutation.

Task_08 defines the behavior; task_10 owns the live walk and screenshots. Focused integration receipts are not QA verdicts.

## Single-review remediation re-walk

Verify rejected add, disable and remove mutations leave both the persisted source overlay and the
currently browsable catalog unchanged. Preserve unrelated settings and original TOML comments;
a subsequent successful settings edit must survive the rejected mutation. The canonical config and
daemon suites passed with real SQLite failure triggers. Public HTTP/UDS scenario execution remains
assigned to final CI/QA; no heavy local gate was run for this correction.
