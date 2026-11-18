# QA report — provider-auth-verdict-cache (#1267)

Date: 2026-11-04 · Build: `df98dd0b6` (branch `fix/provider-auth-verdict-guards`, base v0.5.1)
Verdict: **PASS**

## Scope

QA-1 from `.compozy/tasks/provider-auth-verdict-cache/_tests.md`, scenario
`docs/qa/scenarios/MS-web-provider-auth-gate.md` § "A stored verdict never hides a current failure
(#1267)". Auth-home scoping (US-002) and the ACP deadline guard (US-003) have no reproducible
real-user trigger without live provider accounts; they are owned by `TestAuthVerdicts`,
`TestPreStarter`, `TestProviderAuthHandlers` (IT-001), and `TestProviderAGYACPProbe` (IT-002),
each shown to fail when its guard is reverted.

## Environment

Isolated `eng-qa-bootstrap` lab (targeted profile, surfaces cli + api), lab-only `COMPOZY_HOME`,
port, and sockets; no operator daemon, state, or accounts. Synthetic provider `qa-native`
(`auth_mode = "native_cli"`) whose command and `auth_status_command` point at a fake CLI script
that always reports a signed-in user. Teardown: `teardown.json` `clean: true`.

Lab evidence (outside the repository, kept after teardown):
`~/dev/qa-labs/compozy-auth-verdict-1267-20261104-172002-886911-lab/qa-artifacts/qa/` —
`bootstrap-manifest.json`, `qa1-walk.log` (raw CLI/API captures for every step),
`journey-log.jsonl`, `verification-report.md`, `qa-audit-report.md` (strict audit, no blockers),
`teardown.json`, and `logs/make-gate-task01.log`.

## Walk

| Step | `GET /api/providers` | `GET /api/settings/providers` | `compozy provider list --json` |
| --- | --- | --- | --- |
| Before probe | `unknown` (run a probe), no `last_probe_at` | same | same |
| `POST /api/providers/qa-native/auth/probe` → `authenticated` | `authenticated`, `last_probe_at` set | same | same |
| `chmod a-x` on the fake CLI | `unknown`, message `… is not executable: permission denied`, no `last_probe_at` | same | same |
| Permission restored | `authenticated`, original `last_probe_at` | same | same |

Before this change, the stored verdict replaced the declared resolver error, so the same step
showed `authenticated` with the earlier check time.

## Findings

None.
