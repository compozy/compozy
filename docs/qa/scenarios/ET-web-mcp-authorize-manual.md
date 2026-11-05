---
id: ET-web-mcp-authorize-manual
area: ET
title: MCP authorize manual completion (remote operator paste)
persona: Iris
journey: J-mcp-authorize-repair
expected: Entering manual completion starts a fresh `mode=manual` PKCE session. The dialog accepts only a full redirect URL and sends it to `auth/exchange`; state and issuer mismatch burn the attempt. Success requires the refetched `authenticated && token_present`; a non-confirmed exchange leaves the dialog failed with the prior status intact. The live URL stays copyable throughout.
entry_points: web `/mcp` Authorize -> Enter redirect URL; `POST /api/settings/mcp-servers/{name}/auth/exchange`
qa_status: skipped
bug_ids: BUG-20260715-mcp-oauth-name-segment
fix_status: fixed
retest_status: pass
fix_commits: 8eeb8a38
evidence: web/src/hooks/routes/use-mcp-authorize.ts; web/e2e/__tests__/mcp.spec.ts; .compozy/tasks/marketplace/evidence/visual/task-08/authorize-manual-desktop; /Users/pedronauck/dev/qa-labs/compozy-marketplace-task11-final-20260715-20260716-011529-818379-lab/qa-artifacts/qa/notes/mcp-non-loopback-callback.json; /Users/pedronauck/dev/qa-labs/compozy-marketplace-task11-final-20260715-20260716-011529-818379-lab/qa-artifacts/qa/notes/mcp-manual-tty-redaction.json;/Users/pedronauck/dev/qa-labs/compozy-qa-et-current-source-20260730-061655-910372-lab/qa-artifacts/qa
last_report: docs/qa/reports/2026-10-02-untested.md
overlaps: ET-web-mcp-authorize; ET-cli-mcp-auth-manual-exchange
---

Historical QA note: begin-failure versus exchange-failure recovery coverage remains pending.

story: As a remote admin whose browser cannot reach the daemon host, I paste the full redirect URL to complete authorization.

src: docs/design/opendesign/mcp-management.html

inventory: Needs QA

QA impact 2026-07-15: new behavior from Task 08 (ADR-011 authorization floor). Flagged untested for the next QA cycle.

QA impact 2026-07-16: manual exchange remains available only after a successful begin session;
begin failures now retry the begin step instead of presenting an exchange that cannot succeed.

QA impact 2026-07-17: switching from automatic to manual must issue a second begin request with
`mode=manual` and exchange only against that new session.

2026-10-05: Deferred from this QA cycle by the user's explicit scope reduction.
Coverage remains outstanding; this skip is not a passing result. Resume from the dated
report's session matrix in a future QA cycle.

Gap 1 rule (issue #1305): configure a bare HTTP remote with no `auth`, `headers`, or
`secret_headers` against an OAuth PRM / WWW-Authenticate challenge. Settings must offer
**Sign in** through the existing authorization dialog. Complete a fresh manual redirect,
then confirm `authenticated && token_present` through a fresh status read and a successful
runtime tool probe. Reload Settings to verify persistence. An open HTTP remote and remotes
with plain or secret headers and no explicit OAuth policy must never receive this action. In Advanced editor mode,
**Use OAuth** must toggle the saved `auth.registration=auto` block; editing a header server
must preserve its headers without returning their values.

Gap 1 retest 2026-11-05: the changed bare-remote, header exclusion, header preservation,
and OAuth-toggle branches passed in the isolated daemon lab. See
`docs/qa/reports/2026-11-05-issue-1305-gap1.md`. This does not replace historical coverage or
vendor-consent blockers for unrelated branches in this scenario.

Review regression: an existing explicit OAuth server with auxiliary headers must let **Use OAuth**
switch off and back on before saving. Save and reload must retain both the OAuth policy and
opaque header presence. A header-only server's switch remains disabled.
