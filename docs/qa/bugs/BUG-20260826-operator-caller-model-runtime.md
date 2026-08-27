# BUG-20260826-operator-caller-model-runtime: Completion delivery attached a model runtime to the operator caller

- **Status:** verified
- **Impact (user-side):** Blocks-Completion
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Bruno; Ada
- **Journey Step:** J-delegate-work-to-an-agent — reuse the durable operator caller across calls and daemon restart
- **Scenarios:** RT-agent-call-golden-path
- **Found:** 2026-08-26 · **Report:** `docs/qa/reports/2026-08-26-agent-comms.md`

## Summary

Completion delivery to the durable operator-caller session used the normal agent-session prompt
path. That bound an ACP model runtime to a session that must never own one. After a daemon crash,
the runtime became non-attachable and blocked every later operator-originated call in the workspace.

## Reproduction

1. Create operator-originated calls in a workspace and let their completion deliveries drain.
2. Inspect the operator-caller session: it has a model runtime and ACP session identity.
3. Restart after an interrupted daemon shutdown.
4. Run another operator-originated call in the same workspace.

**Expected:** The stable operator caller remains runtime-free; completion is visible through call
reads and operator attention, and later calls reuse the identity after restart.
**Actual:** Completion bound a model runtime, and the next call failed because that runtime was dead.

## Fix

- **Root cause:** The daemon session adapter did not distinguish an operator-caller recipient before
  applying normal `Status` / `Resume` / `SendPrompt` delivery behavior.
- **Production fix:** Calls runtime injects the authoritative operator-caller lookup into the session
  adapter. Operator completion deliveries are acknowledged as `operator_attention` without touching
  session runtime state; the durable call and attention read models remain the UI/CLI authority.
- **Regression:** The canonical daemon call-delivery suite proves an operator completion becomes an
  injected attention item with zero status, resume, or prompt operations.
- **Fix commit:** `18662244f`.

## Verification

- Focused regression: `go test -race ./internal/daemon -run 'TestDaemonCallDeliveryTracksDurableQueueState' -count=1` — 3 tests passed.
- Public reuse retest: two completed calls reused operator caller
  `ses_operator_d91c902184f7d67758c837aa`; session inspection reported it detached, healthy,
  inactive, and non-attachable after completion delivery.
- Retest evidence: `/Users/pedronauck/dev/qa-labs/compozy-agent-comms-20260826-20260826-065104-728050-lab/qa-artifacts/qa/qa-remediation-public-retest.md`.
- Reproduction evidence: `/Users/pedronauck/dev/qa-labs/compozy-agent-comms-20260826-20260826-065104-728050-lab/qa-artifacts/qa/operator-caller-runtime-reproduction.md`.
