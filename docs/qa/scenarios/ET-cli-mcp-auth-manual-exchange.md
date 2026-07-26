---
id: ET-cli-mcp-auth-manual-exchange
area: ET
title: Complete MCP authorization through manual exchange
persona: Iris
journey: J-mcp-authorize-repair
expected: `agh mcp authorize <name> --manual` accepts either an authorization code or full redirect URL through stdin, completes the daemon-owned single-use session, and never echoes the code, redirect URL, verifier, or token material.
entry_points: agh mcp authorize <name> --manual; agh mcp auth login <name> --manual
qa_status: untested
bug_ids: BUG-20260715-mcp-oauth-name-segment; BUG-20260715-mcp-manual-tty-echo
fix_status: verified
retest_status: pending timeout across manual input and exchange
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/agh-marketplace-northstar-20260715-20260715-114240-757254-lab/qa-artifacts/qa/notes/mcp-guided-oauth-workspace-isolation.json; /Users/pedronauck/dev/qa-labs/agh-marketplace-task11-final-20260715-20260716-011529-818379-lab/qa-artifacts/qa/notes/mcp-manual-tty-redaction.json
last_report: docs/qa/reports/2026-07-15-marketplace.md
overlaps: ET-047; ET-cli-mcp-authorize; ET-api-mcp-oauth-endpoints; ET-web-mcp-authorize-manual
---

Added by marketplace Task 04. QA should exercise code and redirect-URL forms, mismatched state,
expired state, cancellation, a non-loopback daemon bind, and output/log/event scans for the submitted
OAuth material.

QA impact 2026-07-16: `--timeout` is now one deadline over pending manual input and the exchange
request. Verify both phases terminate with the deterministic timeout and no input/token disclosure.
