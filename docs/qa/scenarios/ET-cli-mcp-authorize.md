---
id: ET-cli-mcp-authorize
area: ET
title: Authorize a remote MCP server through the daemon
persona: Ada
journey: J-mcp-authorize-repair
expected: `agh mcp authorize <name>` prints a live copyable URL, waits for a credential change, and exits successfully only when redacted status is `authenticated` with `token_present=true`; scope and workspace selectors target one exact server definition.
entry_points: agh mcp authorize <name>; agh mcp authorize <name> --scope workspace --workspace <id>; agh mcp auth login <name>
qa_status: untested
bug_ids:
fix_status:
retest_status: pending end-to-end authorization timeout semantics
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/agh-marketplace-task11-final-20260715-20260716-011529-818379-lab/qa-artifacts/qa/notes/marketplace-agent-parity-final.json; /Users/pedronauck/dev/qa-labs/agh-marketplace-task11-final-20260715-20260716-011529-818379-lab/qa-artifacts/qa/notes/mcp-oauth-name-segment.json
last_report: docs/qa/reports/2026-07-15-marketplace.md
overlaps: ET-047; ET-cli-mcp-auth-manual-exchange; ET-cli-mcp-install
---

Added by marketplace Task 04. QA should cover global and two workspace targets with the same
server name, an already-authenticated baseline that must change before success, and a false-success
status with `token_present=false` that must exit non-zero.

QA impact 2026-07-16: the authorization timeout now starts before status/begin and bounds automatic
polling, manual input, and manual exchange; the daemon session expiry may only shorten it.

QA impact 2026-07-17: automatic authorization sends `mode=automatic`; `--manual` sends
`mode=manual`. Confirm the two modes create distinct sessions and preserve scoped targeting.
