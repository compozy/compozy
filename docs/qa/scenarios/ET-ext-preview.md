---
id: ET-ext-preview
area: ET
title: Preview an extension lifecycle change
persona: Ada
journey: J-extension-kit-lifecycle
expected: Preview reports the resources, conflicts, unbound environment keys, automation starts, and Network digest for enable or reload without mutating runtime or stored state.
entry_points: compozy extension preview <name> -o json|jsonl|toon; GET /api/extensions/:name/preview over HTTP and UDS; compozy__extensions_preview
qa_status: pass
bug_ids:
fix_status:
retest_status: pass
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-devtool-oss-launch-20260802-195112-911343-lab/qa-artifacts/qa
last_report: docs/qa/reports/2026-08-02-bundles-removal.md
overlaps: ET-ext-inventory; ET-web-extensions-manage
---

QA impact 2026-08-02: new read-only agent surface. Compare all structured planes and prove fresh
inventory, automation state, and resources are byte-stable around every preview action.
