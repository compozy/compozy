---
id: ET-ext-preview
area: ET
title: Preview an extension lifecycle change
persona: Ada
journey: J-extension-kit-lifecycle
expected: Preview reports the resources, conflicts, unbound environment keys, automation starts, and Network digest for enable or reload without mutating runtime or stored state.
entry_points: compozy extension preview <name> -o json|jsonl|toon; GET /api/extensions/:name/preview over HTTP and UDS; compozy__extensions_preview
qa_status: pass
bug_ids: BUG-20260803-extension-preview-layout-identity
fix_status: verified
retest_status: pass
fix_commits: pending Phase D checkpoint
evidence: /Users/pedronauck/dev/qa-labs/compozy-bundles-removal-review-20260803-040035-513450-lab/qa-artifacts/qa/preview-cli.json; /Users/pedronauck/dev/qa-labs/compozy-bundles-removal-review-20260803-040035-513450-lab/qa-artifacts/qa/preview-http.json; /Users/pedronauck/dev/qa-labs/compozy-bundles-removal-review-20260803-040035-513450-lab/qa-artifacts/qa/status-before-preview.json; /Users/pedronauck/dev/qa-labs/compozy-bundles-removal-review-20260803-040035-513450-lab/qa-artifacts/qa/status-after-preview.json; /Users/pedronauck/dev/qa-labs/compozy-bundles-removal-review-20260803-040035-513450-lab/qa-artifacts/qa/preview-enabled-retest.json; /Users/pedronauck/dev/qa-labs/compozy-bundles-removal-review-20260803-040035-513450-lab/qa-artifacts/qa/preview-http-enabled-retest.json
last_report: docs/qa/reports/2026-08-03-bundles-removal-review.md
overlaps: ET-ext-inventory; ET-web-extensions-manage
---

QA impact 2026-08-02: new read-only agent surface. Compare all structured planes and prove fresh
inventory, automation state, and resources are byte-stable around every preview action.

QA impact 2026-08-02: preview now reports only canonical added, changed, and removed resource
deltas; re-walk unchanged content, content edits, renames, and enabled automation changes.
