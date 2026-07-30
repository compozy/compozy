---
id: ET-cli-marketplace-search
area: ET
title: Search the marketplace through structured CLI output
persona: Ada
journey: J-agent-marketplace-parity
expected: `compozy marketplace search` returns fixed-order grouped JSON for all kinds, supports `--kind` browse and `jsonl`, and preserves truthful installed and update fields from the daemon.
entry_points: compozy marketplace search [query] -o json; compozy marketplace search [query] --kind <kind> -o json; compozy marketplace search [query] -o jsonl; compozy.com/runtime/core/marketplace (guide)
qa_status: fail
bug_ids: BUG-20260715-native-marketplace-extension-parity; BUG-20260715-marketplace-stale-report; BUG-20260729-marketplace-json-parity; BUG-20260729-marketplace-file-cursor-fence
fix_status: pending
retest_status:
fix_commits: 8eeb8a38
evidence: /Users/pedronauck/dev/qa-labs/compozy-marketplace-task11-final-20260715-20260716-011529-818379-lab/qa-artifacts/qa/notes/marketplace-agent-parity-final.json; /Users/pedronauck/dev/qa-labs/compozy-northstar-pay-20260729-021949-664736-lab/qa-artifacts/qa/evidence/022-marketplace-namespace
last_report: docs/qa/reports/2026-07-28-untested-full.md
overlaps: ET-007; ET-016
---

Historical QA note: opaque cursor continuation through structured CLI output remains pending.

Added by marketplace Task 02 after the hard cut to one discovery namespace. The next agent-surface QA cycle should compare CLI JSON byte semantics with HTTP and UDS for the same daemon state, including one isolated kind failure.

QA impact 2026-07-18: `--cursor` now continues a single `--kind` from `next_cursor`; grouped search
rejects it. Compare both pages with HTTP/UDS and prove the cursor remains bound to scope/workspace.

QA impact 2026-07-18: grouped JSON omits continuation metadata, while a single-kind continuation
rejects a cursor when its catalog projection changes. Confirm the structured error tells the
operator to restart from the first page.

QA impact 2026-07-18: single-kind human and TOON output append a Page block, and JSONL appends one
`type: "page"` record after the rows. Verify every format exposes `next_cursor` and available total,
stale, and diagnostic metadata without changing the JSON envelope.

QA result 2026-07-29: CLI JSON added a transport-only resolution field, and unchanged local
catalog refetches invalidated continuation cursors. Both canonical regressions, staged root fixes,
and the rebuilt CLI/HTTP/UDS replay are green; the scenario remains failed until the fixes have
governed commits.
