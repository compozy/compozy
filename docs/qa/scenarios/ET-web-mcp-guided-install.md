---
id: ET-web-mcp-guided-install
area: ET
title: Configure a curated MCP server with typed or Vault-backed values
persona: Bruno
journey: J-marketplace-acquisition
expected: Manifest-v2 typed inputs accept only their declared string, identifier, boolean, or secret values; secret inputs accept a typed value or present namespace=mcp Vault ref and inline creation stores without echo. Stdio launch entries and Streamable HTTP entries render only their supported fields, catalog default scope applies when omitted, and the next daemon step is announced.
entry_points: /marketplace/mcp/$entryId; MCP Install action
qa_status: pass
bug_ids: BUG-20260714-keyboard-focus-invisible; BUG-20260715-mcp-install-null-values
fix_status: fixed
retest_status: pass
fix_commits: 8eeb8a38
evidence: /Users/pedronauck/dev/qa-labs/compozy-mcp-2026-catalog-v2-final-rerun-20260730-204949-514647-lab/qa-artifacts/qa/screenshots/mcp-guided-github-input.png; /Users/pedronauck/dev/qa-labs/compozy-mcp-2026-catalog-v2-final-rerun-20260730-204949-514647-lab/qa-artifacts/qa/screenshots/mcp-guided-github-installed.png; /Users/pedronauck/dev/qa-labs/compozy-mcp-2026-catalog-v2-final-rerun-20260730-204949-514647-lab/qa-artifacts/qa/screenshots/mcp-guided-linear-installed.png; /Users/pedronauck/dev/qa-labs/compozy-mcp-2026-catalog-v2-final-rerun-20260730-204949-514647-lab/qa-artifacts/qa/notes/web-guided-github-settings.json
last_report: docs/qa/reports/2026-07-30-mcp-2026-catalog-v2.md
overlaps: ET-cli-mcp-install; ET-api-mcp-catalog-install; ET-web-mcp-authorize
---

Passed in the 2026-07-30 final rerun: GitHub's typed stdio form installed the exact returned server,
and Linear's hosted install stopped at the explicit Authorize handoff without beginning OAuth.
Independent settings evidence contains no secret value. The Web-only existing-Vault branch was not
repeated; its equivalent CLI ownership and redaction contract passed in ET-cli-mcp-install.

Added by marketplace Task 06. Walk typed, existing-ref, inline-create, remote-no-auth, and remote-OAuth branches; inspect network, DOM, logs, and fresh settings reads for plaintext-secret absence.

Historical QA note: exact invoked Authorize and Manage toast destinations remain pending.

Task 10 planning note: persona moved from Ada to Bruno — the guided modal is a human web surface; Ada's structured equivalents are ET-cli-mcp-install and ET-api-mcp-catalog-install.

QA impact 2026-07-16: install success actions now navigate using the exact server identity returned
by the mutation; retest both Authorize and Manage destinations rather than only toast presence.

QA impact 2026-07-30 deep-review remediation: reset after OAuth install copy was corrected to an
explicit post-install Authorize handoff. Verify installation itself does not begin OAuth, the success
state points to the exact installed target, and the subsequent Authorize action owns browser or manual
authorization without exposing secrets.
