---
id: ET-ext-preview
area: ET
title: Preview an extension lifecycle change
persona: Ada
journey: J-agent-marketplace-parity
expected: Preview reports the resources, conflicts, unbound environment keys, automation starts, and Network digest for enable, update, or disable without mutating runtime or stored state.
entry_points: compozy extension preview; GET /api/extensions/:name/preview over HTTP and UDS; compozy__extensions_preview
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: ET-ext-inventory; ET-web-extensions-manage
---

QA impact 2026-08-02: new read-only agent surface. Compare all structured planes and prove fresh
inventory, automation state, and resources are byte-stable around every preview action.
