---
id: ET-cli-mcp-authorize
area: ET
title: Authorize a remote MCP server through the daemon
persona: Ada
journey: J-mcp-authorize-repair
expected: `compozy mcp auth login <name>` prints a live copyable URL, waits for a credential change, and exits successfully only when redacted status is `authenticated` with `token_present=true`; scope and workspace selectors target one exact server definition.
entry_points: compozy mcp auth login <name>; compozy mcp auth login <name> --scope workspace --workspace <id>
qa_status: fail
bug_ids: BUG-20260729-mcp-cli-json-parity; BUG-20260914-mcp-secret-replacement-owner; BUG-20260914-mcp-invalid-definition-repair
fix_status: pending
retest_status: blocked-decision
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-marketplace-task11-final-20260715-20260716-011529-818379-lab/qa-artifacts/qa/notes/marketplace-agent-parity-final.json; /Users/pedronauck/dev/qa-labs/compozy-marketplace-task11-final-20260715-20260716-011529-818379-lab/qa-artifacts/qa/notes/mcp-oauth-name-segment.json; /Users/pedronauck/dev/qa-labs/compozy-northstar-pay-20260729-021949-664736-lab/qa-artifacts/qa/evidence/023-mcp-catalog-install
last_report: docs/qa/reports/2026-07-30-mcp-2026-catalog-v2.md
overlaps: ET-047; ET-cli-mcp-auth-manual-exchange; ET-cli-mcp-install
---

Planning note: end-to-end authorization timeout semantics remain pending; no bug fix is associated with this scenario.

Added by marketplace Task 04. QA should cover global and two workspace targets with the same
server name, an already-authenticated baseline that must change before success, and a false-success
status with `token_present=false` that must exit non-zero.

QA impact 2026-07-16: the authorization timeout now starts before status/begin and bounds automatic
polling, manual input, and manual exchange; the daemon session expiry may only shorten it.

QA impact 2026-07-17: automatic authorization sends `mode=automatic`; `--manual` sends
`mode=manual`. Confirm the two modes create distinct sessions and preserve scoped targeting.

QA result 2026-07-29: automatic and manual S256 flows, the login alias, confirmed credential change,
bounded timeout, three-scope isolation, presence-only status, targeted logout, durable redaction, and
cleanup passed. Workspace JSON added the CLI-only `resolution_source` field; the scenario remains
failed while the required structural writer TechSpec is pending.

PR636 review retest: use manual and extension MCPs with the same server name across user,
profile, workspace and workspace-profile scopes. A configured secret owned by another cell, or
an access/refresh/DCR/registration token ref, must fail before resolution or authorization begins.
Own configured client secrets, explicit shared refs and environment refs remain usable. Verify
released manual user client-secret refs still resolve their persisted values without exposing them.
Focused auth and real Settings/Vault tests passed; the targeted public retest is recorded below.

PR636 recovery follow-up: BUG-20260914-mcp-invalid-definition-repair is verified on 71fb14665. Steps 145–153 replace the previously invalid credential, read Settings and CLI auth status, restart and read again. Steps 155–161 accept shared/environment refs without deleting shared metadata. These are configuration/ownership observations, not completed external OAuth.

PR636 targeted owner-reference retest passed: rejected foreign/managed replacements preserve the existing secret; invalid definitions remain repairable; own/shared/environment references are accepted. Both 2026-09-14 bugs are verified. The historical qa_status and JSON-parity blocker remain unchanged because the full authorization journey was not rerun.
