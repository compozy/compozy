# BUG-20260713-cursor-model-startup-contract: Cursor model selection creates delayed failed sessions

- **Status:** verified
- **Impact (user-side):** Blocks-Completion
- **Severity:** High · **Priority:** P0
- **Persona Affected:** Bruno
- **Journey Step:** J-17 create a session, step 2
- **Scenarios:** RT-new-session-fast-feedback
- **Found:** 2026-07-13 · **Report:** docs/qa/reports/2026-07-13-automation-features.md
- **Origin:** n/a

## Summary

Bruno selected Cursor Agent and Grok 4.5 through the New session runtime selector. The selector either preserved the onboarding alias `cursor-grok-4.5-high` or accepted the visible model id `grok-4.5`; both values created durable failed sessions only after a 14.7–19.5 second wait. The failure itself lists `grok-4.5[effort=high,fast=true]` as the valid Cursor descriptor. Entering that hidden descriptor manually eventually started a session, but the composer was not ready until approximately 18.4 seconds after submission.

## Reproduction

- **Charter:** CH-new-session-latency-title · **Tour:** Network Tour
- **Environment:** desktop / wifi-fast / en-US; isolated daemon at `http://127.0.0.1:58941`; in-app browser; live Cursor ACP.

1. Open Agents and click `New session with general`.
2. Select `Cursor Agent`, search for Grok 4.5, and use the model value presented by the selector.
3. Click `Start session` and measure feedback, navigation, and composer readiness.
4. Reopen the resulting failed session from the agent's Sessions tab.
5. Repeat with the exact descriptor copied from the error: `grok-4.5[effort=high,fast=true]`.

**Expected:** The provider filter exposes Cursor's live model choices, one click selects a valid canonical value, validation fails before spawning when invalid, and a valid session gives immediate progress feedback then reaches a composer within the power-user patience budget.
**Actual:** The checked Cursor filter initially showed OpenCode matches, accepted values that Cursor later rejected, persisted two failed sessions, and surfaced the mismatch only after long waits. The hidden canonical descriptor worked, but startup still took about 18.4 seconds.

## Evidence

- `/Users/pedronauck/dev/qa-labs/agh-automation-features-20260713-20260713-044543-173594-lab/qa-artifacts/qa/screenshots/ch-new-session-invalid-grok-persisted.dom.txt`
- `/Users/pedronauck/dev/qa-labs/agh-automation-features-20260713-20260713-044543-173594-lab/qa-artifacts/qa/screenshots/ch-new-session-grok-transcript.dom.txt`
- Failed sessions `sess-d5879464f13e2350` and `sess-8cdde6e564e0ac5c`; live session `sess-b1c980b86709053d`.
- `/Users/pedronauck/dev/qa-labs/agh-automation-features-20260713-20260713-044543-173594-lab/qa-artifacts/qa/journey-log.jsonl`

## Fix

- **Root cause:** Cursor had no authoritative AGH model catalog, so aliases survived until late ACP negotiation after process launch. The always-installed tool gateway also preferred `ask`/default mode instead of the provider-advertised `agent` mode. The Web selector and backend therefore disagreed on both the valid model identity and effective runtime configuration.
- **Fix commit:** pending
- **Regression test:** Canonical ACP negotiation, model-catalog, session-manager preflight, and Web session-create suites cover exact Cursor model acceptance, alias rejection before reservation/spawn, native-default preservation, reconciled runtime state, and the authoritative-provider boundary.

## Verification

- Live Cursor QA proved canonical and native-default sessions both ran in Agent mode and persisted `grok-4.5[effort=high,fast=true]`; a deprecated alias failed pre-spawn in 5.4 ms with no session/process side effect. Controller UI replay exposed one visible canonical `Grok 4.5 (High, Fast)` choice and created sessions `sess-54f0241c9aed8aec` and `sess-c09b90c914321946` successfully. The instrumented run reached HTTP 201 6.452 seconds after click and began destination loading 7 ms later. The separate post-success overlay defect is tracked and verified as `BUG-20260713-new-session-modal-lingers`.
