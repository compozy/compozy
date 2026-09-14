---
id: ET-cli-marketplace-sources
area: ET
title: Manage marketplace sources from the CLI
persona: Ada
journey: J-agent-marketplace-parity
expected: "Source commands expose experimental stability, structured errors and persistent global mutations with immediate runtime effect."
entry_points: "compozy marketplace sources list|add|remove|refresh -o json"
qa_status: pass
bug_ids:
fix_status:
retest_status: pass
fix_commits:
evidence: docs/qa/evidence/2026-09-13-marketplace-catalog/source-readded.json
last_report: docs/qa/reports/2026-09-13-marketplace-catalog.md
overlaps:
---

QA 2026-09-13: the current hard-cut contract passed the scoped live/API/browser walks and applicable unchanged owning integration checks. See the dated report for exact evidence and boundaries; historical notes below do not redefine the current catalog.


Use a fixture folder; duplicate add and a missing/oversized document exit 2 with structured details. Toggle via config set marketplace.plugin_sources.<name>.enabled false --scope user. Install a named-source plugin with extension install; preserve curated owner/repo resolution.

Task_08 defines the behavior; task_10 owns the live walk and screenshots. Focused integration receipts are not QA verdicts.
