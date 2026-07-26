---
id: ET-web-mcp-authorize
area: ET
title: MCP OAuth authorize/repair flow (browser auto path)
persona: Bruno
journey: J-mcp-authorize-repair
expected: Authorize on a needs_login OAuth remote begins an explicit `mode=automatic` daemon PKCE session and always renders the live `auth/begin` authorization_url copyable (browser open optional). The waiting dialog polls the scoped list and confirms ONLY on `authenticated && token_present`. A tools/list-style success alone never flips the UI. Automatic begin failure offers retry and a manual-callback fallback; cancel/failure preserves the prior public status and token.
entry_points: web `/mcp` Authorize action; `POST /api/settings/mcp-servers/{name}/auth/begin`; daemon callback `GET /api/mcp/oauth/callback`
qa_status: untested
bug_ids: BUG-20260715-mcp-oauth-name-segment
fix_status: BUG-20260715-mcp-oauth-name-segment fixed
retest_status: pending begin-attempt race and truthful begin-failure recovery
fix_commits:
evidence: web/src/hooks/routes/use-mcp-authorize.ts; web/src/systems/settings/components/mcp-authorize-dialog.tsx; web/e2e/__tests__/mcp.spec.ts; .compozy/tasks/marketplace/evidence/visual/task-08/authorize-waiting-desktop; /Users/pedronauck/dev/qa-labs/agh-marketplace-task11-final-20260715-20260716-011529-818379-lab/qa-artifacts/qa/notes/mcp-oauth-name-segment.json; /Users/pedronauck/dev/qa-labs/agh-marketplace-task11-final-20260715-20260716-011529-818379-lab/qa-artifacts/qa/web/mcp-oauth-confirmed.png
last_report: docs/qa/reports/2026-07-15-marketplace.md
overlaps: ET-api-mcp-oauth-endpoints; ET-cli-mcp-authorize; ET-web-mcp-authorize-manual
---

story: As a builder I authorize a remote MCP server from the product, following the browser handoff, and see it confirmed only when the credential is truly present.

src: docs/design/opendesign/mcp-management.html

inventory: Needs QA

QA impact 2026-07-15: new behavior from Task 08 (ADR-006/016). Flagged untested for the next QA cycle.

QA impact 2026-07-16: OAuth begin attempts now have monotonic ownership; an older response cannot
replace a retry, and a begin failure offers Retry authorization without exposing an unusable exchange.

QA impact 2026-07-17: automatic begin now sends `mode=automatic`; begin failure offers a manual
fallback that creates a new `mode=manual` session rather than exchanging against the failed attempt.

QA impact 2026-07-18: global MCP definitions included in workspace reads authorize against their
global effective source instead of an unavailable workspace target.
