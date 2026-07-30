---
id: ET-cli-mcp-auth-manual-exchange
area: ET
title: Complete MCP authorization through manual exchange
persona: Iris
journey: J-mcp-authorize-repair
expected: `compozy mcp auth login <name> --manual` accepts only a full redirect URL through stdin, validates state and issuer for the daemon-owned single-use session, burns that session on every exchange attempt, and never echoes OAuth material.
entry_points: compozy mcp auth login <name> --manual
qa_status: blocked-verify
bug_ids: BUG-20260715-mcp-oauth-name-segment; BUG-20260715-mcp-manual-tty-echo; BUG-20260729-mcp-manual-exchange-timeout
fix_status: fixed
retest_status: pass
fix_commits: 8eeb8a38;351f3535
evidence: /Users/pedronauck/dev/qa-labs/compozy-marketplace-northstar-20260715-20260715-114240-757254-lab/qa-artifacts/qa/notes/mcp-guided-oauth-workspace-isolation.json; /Users/pedronauck/dev/qa-labs/compozy-marketplace-task11-final-20260715-20260716-011529-818379-lab/qa-artifacts/qa/notes/mcp-manual-tty-redaction.json; /Users/pedronauck/dev/qa-labs/compozy-northstar-pay-20260729-021949-664736-lab/qa-artifacts/qa/evidence/024-mcp-oauth-endpoints
last_report: docs/qa/reports/2026-07-30-mcp-2026-catalog-v2.md
overlaps: ET-047; ET-cli-mcp-authorize; ET-api-mcp-oauth-endpoints; ET-web-mcp-authorize-manual
---

Added by marketplace Task 04. QA should exercise full redirect-URL input, mismatched state or
issuer, expired state, cancellation, a non-loopback daemon bind, and output/log/event scans for the
submitted OAuth material.

Historical QA note: timeout coverage across manual input and exchange remains pending.

QA impact 2026-07-16: `--timeout` is now one deadline over pending manual input and the exchange
request. Verify both phases terminate with the deterministic timeout and no input/token disclosure.

QA result 2026-07-29: bare-code and full-redirect stdin forms, the login alias, wrong-state
non-consumption, signal cancellation, input timeout, exchange timeout, and OAuth-material redaction
ran against a real loopback provider. The exchange phase originally exposed the raw UDS POST error;
the staged CLI correction now returns the canonical authorization timeout and passes its live replay.
The scenario remains failed until the root fix has a governed commit.
