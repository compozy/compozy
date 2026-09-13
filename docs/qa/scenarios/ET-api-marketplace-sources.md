---
id: ET-api-marketplace-sources
area: ET
title: Manage marketplace sources over HTTP and UDS
persona: Ada
journey: J-agent-marketplace-parity
expected: "Both transports enforce privileged mutations and return matching source state, preview, diagnostics and rejection envelopes."
entry_points: "GET/POST /api/marketplace/sources; PATCH/DELETE /api/marketplace/sources/{name}; POST /api/marketplace/sources/{name}/refresh"
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps:
---

Compare dry-run/config bytes, Add201, duplicate409, invalid422, preset-delete403 and remove204. Exercise concurrent additions, disabled offline refresh and retained installed names. Re-read sources and extension inventory after each mutation.

Task_08 defines the behavior; task_10 owns the live walk and screenshots. Focused integration receipts are not QA verdicts.
