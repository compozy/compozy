---
id: ET-web-mcp-status-matrix
area: ET
title: MCP management status matrix renders daemon truth
persona: Bruno
journey: J-mcp-authorize-repair
expected: The `/marketplace/mcps?tab=installed` cards and detail management render daemon-owned auth, runtime, probe, and negotiated protocol version with truthful success/warning/danger/neutral tones. Failed, unavailable, denied, absent, and unknown runtime states are never labeled running. `probe=skipped` is never treated as a failure. Authorize/Reauthorize is offered only to OAuth Streamable HTTP remotes not authenticated (or auth_refresh_failed); stdio and non-OAuth remotes never get it.
entry_points: web `/marketplace/mcps?tab=installed`; `GET /api/settings/mcp-servers`
qa_status: pass
bug_ids:
fix_status: fixed
retest_status: pass
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-mcp-2026-catalog-v2-final-rerun-20260730-204949-514647-lab/qa-artifacts/qa/notes/web-status-matrix.json; /Users/pedronauck/dev/qa-labs/compozy-mcp-2026-catalog-v2-final-rerun-20260730-204949-514647-lab/qa-artifacts/qa/screenshots/mcp-status-matrix.png
last_report: docs/qa/reports/2026-07-30-mcp-2026-catalog-v2.md
overlaps: MS-029
---

Passed in the 2026-07-30 final rerun: GitHub rendered dead with its probe skipped and never appeared
green; Linear rendered authorization-required without being labeled running. The settings payload
contained neither tokens nor Vault refs. The authenticated-remote 401 branch remains unit-covered
but was not newly induced against a vendor endpoint in this walk.

Planning note: workspace-guard action-gating verification remains pending; no bug fix is associated with this scenario.

story: As a builder I read every configured MCP server's real configuration, authorization, runtime, and probe status at a glance without a plausible green masking a broken server.

src: docs/design/opendesign/mcp-management.html

inventory: Needs QA

QA impact 2026-07-15: new behavior from Task 08 (ADR-006). The `/mcp` page replaced the hardcoded success dot with the composed status matrix. Flagged untested for the next QA cycle.

QA impact 2026-07-16: the Add server action is now absent while the workspace guard cannot
resolve a valid scope, eliminating the previous visible-but-inert control.

QA impact 2026-07-18: the consolidated Installed card now derives its label and tone from the
canonical settings status model instead of defaulting non-auth states to `running`.

QA impact 2026-07-18: OAuth authorization now targets `source_metadata.effective_source`; verify a
global-only MCP remains authorizable when it appears in a workspace-scoped collection response.

QA impact 2026-07-30 deep-review remediation: reset after authenticated remote HTTP 401 responses
were reclassified from unreachable to `mcp_auth_required`. Verify cards and detail management offer
Authorize/Reauthorize without claiming the server is running, while transport/network failures remain
unavailable and no token, binding ref, or OAuth redirect appears in status payloads.
