# QA Run Report — 2026-08-12 — post-merge regression

- **Scope:** Scheduled agent-session recovery across a daemon restart after the performance branch integrated current `main`.
- **Cadence tier:** sanity
- **Build:** `1f5501cb-dirty` · **Environment:** production Web bundle and real isolated daemon with a deterministic ACP boundary fixture; no external provider was required.
- **Started:** 2026-08-12T10:31:00-03:00 · **Status:** closed

## Personas

| Persona | Base | Device / Network / Locale | Sessions |
|---|---|---|---|
| Bruno | Power User | Desktop / Wi-Fi-fast / en-US | CH-scheduled-session-restart-recovery |

## Flows in Scope

- `J-recover-scheduled-job-restart` — Restart while a recurring agent job is active and recover the same durable schedule (`../journeys/J-recover-scheduled-job-restart.md`).

## Session Matrix & Results

| # | Charter | Journey / Scenario | Persona | Tour | Status | Issue | Fix commit |
|---|---|---|---|---|---|---|---|
| 1 | CH-scheduled-session-restart-recovery | J-recover-scheduled-job-restart / TA-scheduled-session-restart-recovery | Bruno | Interrupt | Pass | | |

Status legend: `Pending | Pass | Fixed | Skipped | Blocked (needs human verify) | Blocked (human decision)`

## Session Debriefs

### CH-scheduled-session-restart-recovery — Bruno

- **Ran:** 2026-08-12T10:43:00-03:00 → 2026-08-12T10:46:00-03:00 (box respected: yes)
- **Findings:** None after the production recovery fix. The replacement daemon reported healthy, the scheduler remained registered, and five unique completed fire ids were visible through CLI history after restart.
- **Bugs filed/updated:** None. The earlier failure was found by the automated engineering gate before this QA walk and is owned by the production regression below.
- **Scenarios settled:** TA-scheduled-session-restart-recovery → Pass.
- **Paper cuts:** None.
- **Surprises:** A terminal metadata snapshot is the authoritative recovery input when shutdown cancels the catalog transaction after the provisional row was written.
- **Suggested next charter:** Interrupt a manually prompted session at the same restart boundary; it was outside this scheduled-job recovery scope.

## What Was Fixed

### Scheduled session identity recovery during restart

- **Symptom:** A job firing during restart could leave a provisional identityless session row; the replacement daemon then exited with `session_creation_identity_mismatch`.
- **Root cause:** Recovery required both the stored row and the incoming metadata snapshot to remain `starting`, even though canceled startup had already persisted terminal metadata.
- **Fix:** A complete same-workspace snapshot may bind identity only while the stored row is still the provisional `starting` row. Existing workspace, immutable-identity, and non-provisional fences remain.
- **Regression test:** `internal/store/globaldb/global_db_goal_binding_integration_test.go` reproduces the terminal-snapshot/provisional-row boundary and rejects a settled identityless row.
- **Retested:** `web/e2e/__tests__/jobs-hardening.spec.ts` restarted the real daemon, reloaded Jobs, and captured browser plus HTTP/UDS/CLI parity.

## Coverage Notes

- **Journey and functional:** The public restart path reached the true end state, and independent structured reads agreed.
- **Experiential:** The refreshed Jobs surface returned without a stalled reconnect state or browser console error.
- **Edge and recovery:** The restart landed next to a one-second schedule boundary; a fresh load, rather than optimistic state, proved recovery.
- **Cross-cutting:** The complete Web lane also kept the 12-window drag budget and provider-override selector journeys green as adjacent canaries.

## Paper Cuts

None observed.

## Runtime Errors Observed

None. The captured browser console is empty.

## Human Verifications Needed

None.

## Decisions for a Human

None.

## Learnings

- Session metadata is the recovery source after startup cancellation; the global catalog row is a provisional projection until its immutable creation identity is bound.
- Restart readiness must be confirmed from the replacement daemon and a fresh public read, not from the restart request response.

## Final Status

- **Exit gate (targeted public-interface suite):** `make test-e2e-web` exited 0; `.tmp/playwright/test-results/.last-run.json` records `status: passed` and zero failed tests.
- **Issues by user impact:** Blocks-Completion 0 · Data-Loss 0 · Trust-Damage 0 · Friction 0 · Cosmetic 0
- **Coverage:** 1/1 journey walked; the full Web lane supplied adjacent browser canaries.
- **Verdict:** ready — scheduled restart recovery and the complete daemon-served Web lane passed; the workstream-wide full gate follows this report.
