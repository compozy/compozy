---
id: ET-cli-marketplace-sources
area: ET
title: Manage marketplace sources from the CLI
persona: Ada
journey: J-agent-marketplace-parity
expected: "Source commands expose experimental stability, structured errors and persistent global mutations with immediate runtime effect."
entry_points: "compozy marketplace sources list|add|remove|refresh -o json"
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps:
---

Use a fixture folder; duplicate add and a missing/oversized document exit 2 with structured details. Toggle via config set marketplace.plugin_sources.<name>.enabled false --scope user. Install a named-source plugin with extension install; preserve curated owner/repo resolution.

Task_08 defines the behavior; task_10 owns the live walk and screenshots. Focused integration receipts are not QA verdicts.
