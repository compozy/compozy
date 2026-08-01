# QA Run Report — 2026-07-31 — Session prompt idempotency

- **Scope:** Durable prompt identity, exact replay, provider-echo suppression, live transcript reconciliation, and cold reload across Web plus structured session-prompt surfaces.
- **Cadence tier:** targeted
- **Build:** current working tree · **Environment:** isolated lab `compozy-session-prompt-idempotency-20260801-040518-847041-lab`; daemon `http://127.0.0.1:53003`; Web proxy derives from the bootstrap manifest.
- **Started:** 2026-08-01T03:57:49Z · **Status:** in progress

## Personas

| Persona | Base | Device / Network / Locale | Sessions |
|---|---|---|---|
| Théo | Power User | desktop / wifi-fast / en-US | pending |

## Flows in Scope

- `J-11` — send one authored prompt, observe live settlement, and reload the exact permalink.
- `RT-session-prompt-idempotency` — replay one command by its original identities and reject conflicting reuse.
- `RT-session-message-reload` — preserve one authored row in strict chronology through streaming and cold reload.

## Session Matrix & Results

| # | Journey / Scenario | Entry point | Persona | Status | Evidence |
|---|---|---|---|---|---|
| 1 | CH-session-prompt-identity / RT-session-message-reload | Web session thread · Interrupt Tour | Théo | Pending | |
| 2 | CH-session-prompt-identity / RT-session-prompt-idempotency | HTTP or CLI plus independent transcript read | Théo | Pending | |
| 3 | CH-session-prompt-identity / RT-session-prompt-idempotency | conflicting identity reuse | Théo | Pending | |

Status legend: `Pending | Pass | Fixed | Skipped | Blocked (needs human verify) | Blocked (human decision)`

## Runtime Errors Observed

- Pending execution.

## Human Verifications Needed

- None planned.

## Decisions for a Human

- None planned.

## Final Status

- **Exit gate:** pending isolated walk, screenshot evidence, teardown, and `make gate-full`.
- **Verdict:** in progress.
