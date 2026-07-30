---
id: ET-web-mcp-authorize
area: ET
title: MCP OAuth authorize/repair flow (browser auto path)
persona: Bruno
journey: J-mcp-authorize-repair
expected: Authorize on a needs_login OAuth remote begins an explicit `mode=automatic` daemon PKCE session and always renders the live `auth/begin` authorization_url copyable (browser open optional). The waiting dialog polls the scoped list and confirms ONLY on `authenticated && token_present`. A tools/list-style success alone never flips the UI. Automatic begin failure offers retry and a manual-callback fallback; cancel/failure preserves the prior public status and token.
entry_points: web `/mcp` Authorize action; `POST /api/settings/mcp-servers/{name}/auth/begin`; daemon callback `GET /api/mcp/oauth/callback`
qa_status: blocked-verify
bug_ids: BUG-20260715-mcp-oauth-name-segment
fix_status: fixed
retest_status: pass
fix_commits: 8eeb8a38
evidence: /Users/pedronauck/dev/qa-labs/compozy-mcp-2026-catalog-v2-final-rerun-20260730-204949-514647-lab/qa-artifacts/qa/screenshots/mcp-guided-linear-installed.png; /Users/pedronauck/dev/qa-labs/compozy-mcp-2026-catalog-v2-final-rerun-20260730-204949-514647-lab/qa-artifacts/qa/notes/mcp-status-after-linear.json; /Users/pedronauck/dev/qa-labs/compozy-mcp-2026-catalog-v2-final-rerun-20260730-204949-514647-lab/qa-artifacts/qa/screenshots/mcp-status-matrix.png
last_report: docs/qa/reports/2026-07-30-mcp-2026-catalog-v2.md
overlaps: ET-api-mcp-oauth-endpoints; ET-cli-mcp-authorize; ET-web-mcp-authorize-manual
---

story: As a builder I authorize a remote MCP server from the product, following the browser handoff, and see it confirmed only when the credential is truly present.

src: docs/design/opendesign/mcp-management.html

inventory: Needs QA

Historical QA note: the begin-attempt race and truthful begin-failure recovery remain pending.

QA impact 2026-07-15: new behavior from Task 08 (ADR-006/016). Flagged untested for the next QA cycle.

QA impact 2026-07-16: OAuth begin attempts now have monotonic ownership; an older response cannot
replace a retry, and a begin failure offers Retry authorization without exposing an unusable exchange.

QA impact 2026-07-17: automatic begin now sends `mode=automatic`; begin failure offers a manual
fallback that creates a new `mode=manual` session rather than exchanging against the failed attempt.

QA impact 2026-07-18: global MCP definitions included in workspace reads authorize against their
global effective source instead of an unavailable workspace target.

QA impact 2026-07-26: browser and manual authorization completion is fenced and delivered by the
store even if the initiating hook unmounts; retries cannot settle an older attempt. Status remains
untested; no QA replay ran.

QA impact 2026-07-30 deep-review remediation: configured scopes missing from the granted token now
open an explicit review dialog. Verify no OAuth begin occurs before approval, only newly configured
scopes are requested, cancel preserves the existing token, and approval sends
`approve_scope_escalation=true` with normalized deduplicated scopes. Human vendor consent remains
`blocked-verify`; the local fixture and deterministic Web flow must still pass.

QA result 2026-07-30: the installed Linear target truthfully exposed the authorization handoff and
remained authorization-required; the UI did not claim completion or expose token material. Human
Linear consent and the resulting authenticated refetch remain `blocked-verify`.
