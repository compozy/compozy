---
id: LP-fanout-verdict-identity
area: LP
title: Distinguish fan-out gate verdicts in generation detail
persona: Bruno
journey: J-improve-loop-with-feedback
expected: Generation detail preserves every durable gate verdict as a separate row keyed by gate_id and item_index, including siblings that share one gate_id, with the same identity through HTTP, UDS, CLI structured output, and generated clients.
entry_points: compozy loop status -o json; HTTP/UDS GET /api/workspaces/:workspace_id/loop-runs/:run_id; generated OpenAPI clients
qa_status: pass
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-loops-fanout-verdict-identity-20260802-025955-563449-lab/qa-artifacts/cli-loop-status.json; /Users/pedronauck/dev/qa-labs/compozy-loops-fanout-verdict-identity-20260802-025955-563449-lab/qa-artifacts/api-loop-status.json; /Users/pedronauck/dev/qa-labs/compozy-loops-fanout-verdict-identity-20260802-025955-563449-lab/qa-artifacts/web-loop-run.png
last_report: docs/qa/reports/2026-08-02-loops-coderabbit-followup.md
overlaps: LP-ratchet-climb-restore
---

Create or recover a generation containing two verdict rows for the same fan-out gate. Confirm the
public generation detail returns both rows, each with its own non-negative `item_index`, without
collapsing diagnostics or changing their durable order.
