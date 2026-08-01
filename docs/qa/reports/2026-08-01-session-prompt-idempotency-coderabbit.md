# QA Run Report — 2026-08-01 — Session prompt idempotency CodeRabbit remediation

- **Scope:** Targeted revalidation of exact Goal replay after the CodeRabbit remediation batch.
- **Cadence tier:** targeted
- **Build:** current working tree · **Environment:** isolated lab `compozy-session-prompt-idempotency-coderabbit-20260801-093002-740482-lab` serving current `web/dist`.
- **Started:** 2026-08-01T09:30:02Z · **Finished:** 2026-08-01T09:46:33Z · **Status:** pass

## Personas

| Persona | Base | Device / Network / Locale | Sessions |
|---|---|---|---|
| Théo | Power User | desktop / wifi-fast / en-US | `sess-801f53c434470dcc` (HTTP admission); `sess-7e6ffa1211bbf445` (clean Web rendering) |

## Flows in Scope

- `J-11` — submit one Goal control and preserve one authored row.
- `RT-session-prompt-idempotency` — replay the exact command without converting a stored non-OK response into a synthetic success.

## Session Matrix & Results

| # | Journey / Scenario | Entry point | Persona | Status | Evidence |
|---|---|---|---|---|---|
| 1 | J-11 / RT-session-prompt-idempotency | HTTP exact replay | Théo | Pass | `goal-replay-coderabbit.json` |
| 2 | J-11 / RT-session-prompt-idempotency | Current compiled Web transport | Théo | Pass | `screenshots/coderabbit-goal-replay-clean.png` |

## Session Debrief

- The first Goal admission used `message_id=msg-coderabbit-goal-replay` and `idempotency_key=idem-coderabbit-goal-replay`. It returned `goal_not_active` with HTTP 404 and `replayed=false`.
- The exact retry returned the same identities, diagnostic, and HTTP status with `replayed=true`.
- A clean browser session exercised the compiled Web transport against the deterministic replayed 404 response. It rendered exactly one `/goal status` row plus `Start a Goal before using this control, then try again.` and did not synthesize a successful Goal completion.
- The screenshot was captured at 1440×1000 through the project CDP screenshot flow and passed visual inspection.
- The remediation batch addressed all 16 valid CodeRabbit findings. Two alias/fallback requests were rejected because they would add obsolete compatibility contrary to the Greenfield Alpha contract.

## Evidence Root

`/Users/pedronauck/dev/qa-labs/compozy-session-prompt-idempotency-coderabbit-20260801-093002-740482-lab/qa-artifacts/qa/`

## Runtime Errors Observed

- The intentional `goal_not_active` response was the behavior under test. No unexpected runtime error occurred.

## Human Verifications Needed

- None planned.

## Decisions for a Human

- None planned.

## Final Status

- **QA exit gate:** pass — the exact non-OK replay preserved its error semantics and one-row rendering.
- **Teardown:** `qa/teardown.json` completed with `clean=true`, no survivors, and both registered daemon/browser PIDs stopped.
- **Verdict:** pass.
