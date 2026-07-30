---
id: ET-web-mcp-guided-install
area: ET
title: Configure a curated MCP server with typed or Vault-backed values
persona: Bruno
journey: J-marketplace-acquisition
expected: Manifest-v2 typed inputs accept only their declared string, identifier, boolean, or secret values; secret inputs accept a typed value or present namespace=mcp Vault ref and inline creation stores without echo. Stdio launch entries and Streamable HTTP entries render only their supported fields, catalog default scope applies when omitted, and the next daemon step is announced.
entry_points: /marketplace/mcp/$entryId; MCP Install action
qa_status: skipped
bug_ids: BUG-20260714-keyboard-focus-invisible; BUG-20260715-mcp-install-null-values
fix_status: fixed
retest_status: pass
fix_commits: 8eeb8a38
evidence: /Users/pedronauck/dev/qa-labs/compozy-marketplace-northstar-20260715-20260715-114240-757254-lab/qa-artifacts/qa/screenshots/marketplace-guided-oauth-installed.png; /Users/pedronauck/dev/qa-labs/compozy-marketplace-northstar-20260715-20260715-114240-757254-lab/qa-artifacts/qa/notes/mcp-guided-oauth-workspace-isolation.json; /Users/pedronauck/dev/qa-labs/compozy-marketplace-task11-final-20260715-20260716-011529-818379-lab/qa-artifacts/qa/notes/marketplace-under-minute.json; /Users/pedronauck/Dev/compozy/compozy/.tmp/bug-20260714-focus/focused.png;/Users/pedronauck/dev/qa-labs/compozy-qa-et-current-source-20260730-061655-910372-lab/qa-artifacts/qa
last_report: docs/qa/reports/2026-07-30-mcp-2026-catalog-v2.md
overlaps: ET-cli-mcp-install; ET-api-mcp-catalog-install; ET-web-mcp-authorize
---

Skipped in the 2026-07-30 MCP 2026/catalog-v2 closeout: browser captures did not include independent installed-server reads after every typed and Vault-backed branch.

Added by marketplace Task 06. Walk typed, existing-ref, inline-create, remote-no-auth, and remote-OAuth branches; inspect network, DOM, logs, and fresh settings reads for plaintext-secret absence.

Historical QA note: exact invoked Authorize and Manage toast destinations remain pending.

Task 10 planning note: persona moved from Ada to Bruno — the guided modal is a human web surface; Ada's structured equivalents are ET-cli-mcp-install and ET-api-mcp-catalog-install.

QA impact 2026-07-16: install success actions now navigate using the exact server identity returned
by the mutation; retest both Authorize and Manage destinations rather than only toast presence.
