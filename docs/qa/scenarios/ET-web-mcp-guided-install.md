---
id: ET-web-mcp-guided-install
area: ET
title: Configure a curated MCP server with typed or Vault-backed values
persona: Bruno
journey: J-marketplace-acquisition
expected: Stdio required fields accept either typed values or present namespace=mcp Vault refs, inline secret creation stores without echo, remote entries show URL/auth without secret fields, and the next daemon step is announced.
entry_points: /marketplace/mcp/$entryId; MCP Install action
qa_status: untested
bug_ids: BUG-20260714-keyboard-focus-invisible; BUG-20260715-mcp-install-null-values
fix_status: BUG-20260714-keyboard-focus-invisible fixed; BUG-20260715-mcp-install-null-values fixed
retest_status: pending exact invoked Authorize and Manage toast destinations
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/agh-marketplace-northstar-20260715-20260715-114240-757254-lab/qa-artifacts/qa/screenshots/marketplace-guided-oauth-installed.png; /Users/pedronauck/dev/qa-labs/agh-marketplace-northstar-20260715-20260715-114240-757254-lab/qa-artifacts/qa/notes/mcp-guided-oauth-workspace-isolation.json; /Users/pedronauck/dev/qa-labs/agh-marketplace-task11-final-20260715-20260716-011529-818379-lab/qa-artifacts/qa/notes/marketplace-under-minute.json; /Users/pedronauck/Dev/compozy/agh/.tmp/bug-20260714-focus/focused.png
last_report: docs/qa/reports/2026-07-15-marketplace.md
overlaps: ET-cli-mcp-install; ET-api-mcp-catalog-install; ET-web-mcp-authorize
---

Added by marketplace Task 06. Walk typed, existing-ref, inline-create, remote-no-auth, and remote-OAuth branches; inspect network, DOM, logs, and fresh settings reads for plaintext-secret absence.

Task 10 planning note: persona moved from Ada to Bruno — the guided modal is a human web surface; Ada's structured equivalents are ET-cli-mcp-install and ET-api-mcp-catalog-install.

QA impact 2026-07-16: install success actions now navigate using the exact server identity returned
by the mutation; retest both Authorize and Manage destinations rather than only toast presence.
