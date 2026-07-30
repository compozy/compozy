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
evidence: web/src/systems/settings/lib/mcp-status-view-model.ts; web/src/systems/settings/components/mcp-servers-table.tsx; /Users/pedronauck/dev/qa-labs/compozy-mcp-2026-catalog-v2-20260730-172217-425770-lab/qa-artifacts/qa/evidence/playwright-runtime-status.json; /Users/pedronauck/dev/qa-labs/compozy-mcp-2026-catalog-v2-20260730-172217-425770-lab/qa-artifacts/qa/evidence/auth-status-parity.json; /tmp/eng-ui-screenshot.ePSjW2/captures/marketplace-mcps.png
last_report: docs/qa/reports/2026-07-30-mcp-2026-catalog-v2.md
overlaps: MS-029
---

Passed in the 2026-07-30 MCP 2026/catalog-v2 retest: the final daemon reported Playwright `ready`
with 24 tools and negotiated protocol `2025-11-25`, preserved explicit unavailable and dead states
for invalid/login-required targets, and completed the dense collection in 13.7 seconds. Named auth
status matched over CLI, HTTP, and UDS without collection probing or credential/provenance leakage.

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
