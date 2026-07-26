---
id: ET-web-mcp-authorize-manual
area: ET
title: MCP authorize manual completion (remote operator paste)
persona: Iris
journey: J-mcp-authorize-repair
expected: Entering manual completion starts a fresh `mode=manual` PKCE session. The dialog accepts a bare code OR a full redirect URL and sends exactly one value to `auth/exchange`. Success requires the refetched `authenticated && token_present`; a non-confirmed exchange leaves the dialog failed with the prior status intact. The live URL stays copyable throughout.
entry_points: web `/mcp` Authorize -> Enter code or redirect; `POST /api/settings/mcp-servers/{name}/auth/exchange`
qa_status: untested
bug_ids: BUG-20260715-mcp-oauth-name-segment
fix_status: fixed
retest_status: pending begin-failure versus exchange-failure recovery states
fix_commits:
evidence: web/src/hooks/routes/use-mcp-authorize.ts; web/e2e/__tests__/mcp.spec.ts; .compozy/tasks/marketplace/evidence/visual/task-08/authorize-manual-desktop; /Users/pedronauck/dev/qa-labs/agh-marketplace-task11-final-20260715-20260716-011529-818379-lab/qa-artifacts/qa/notes/mcp-non-loopback-callback.json; /Users/pedronauck/dev/qa-labs/agh-marketplace-task11-final-20260715-20260716-011529-818379-lab/qa-artifacts/qa/notes/mcp-manual-tty-redaction.json
last_report: docs/qa/reports/2026-07-15-marketplace.md
overlaps: ET-web-mcp-authorize; ET-cli-mcp-auth-manual-exchange
---

story: As a remote admin whose browser cannot reach the daemon host, I paste the authorization code or full redirect URL to complete authorization.

src: docs/design/opendesign/mcp-management.html

inventory: Needs QA

QA impact 2026-07-15: new behavior from Task 08 (ADR-011 authorization floor). Flagged untested for the next QA cycle.

QA impact 2026-07-16: manual exchange remains available only after a successful begin session;
begin failures now retry the begin step instead of presenting an exchange that cannot succeed.

QA impact 2026-07-17: switching from automatic to manual must issue a second begin request with
`mode=manual` and exchange only against that new session.
