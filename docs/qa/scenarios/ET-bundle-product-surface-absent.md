---
id: ET-bundle-product-surface-absent
area: ET
title: Reject the retired Bundle product while preserving homonyms
persona: Ada
journey: J-bundle-product-boundary
expected: Every former Bundle product command, route, kind, tool, capability, Web route, docs entry, resource kind, and stored projection is absent with no alias or residue, while support bundle create/status/download and the bundled-skill source filter still work.
entry_points: compozy bundle; /api/bundles/* over HTTP and UDS; compozy__bundles_* and compozy__bundles; marketplace kind bundle; /marketplace/bundles; /docs/api/bundles; /docs/cli/bundle; compozy support bundle --yes [--output <path>]; /api/support/bundles/*; compozy skill list --source bundled -o json|jsonl|toon
qa_status: pass
bug_ids: BUG-20260802-retired-marketplace-kind-alias
fix_status: fixed
retest_status: pass
fix_commits: 7701a3f
evidence: /Users/pedronauck/dev/qa-labs/compozy-devtool-oss-launch-20260802-195112-911343-lab/qa-artifacts/qa
last_report: docs/qa/reports/2026-08-02-bundles-removal.md
overlaps: MS-046; MS-047; MS-048; ET-spec-cycle-skill-bundle; ET-api-marketplace-namespace; ET-049
---

QA impact 2026-08-02: hard-cut boundary scenario. Probe the deleted product and the surviving
homonyms in one session so a broad text deletion cannot masquerade as a correct product cut.
