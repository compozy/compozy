# QA Run Report — 2026-08-03 — session-input-coderabbit

- **Scope:** PR #304 CodeRabbit remediation for durable busy-session inputs and working-state feedback
- **Cadence tier:** targeted
- **Build:** `6c884f4c` + uncommitted PR #304 remediation · **Environment:** fresh isolated Compozy lab on HTTP `127.0.0.1:55434`; live Codex provider; actual Web route plus public CLI and HTTP reads
- **Started:** 2026-08-03T21:42:52-03:00 · **Status:** in-progress

## Personas

| Persona | Base | Device / Network / Locale | Sessions |
|---|---|---|---|
| Théo | Returning operator | Desktop Chrome, local network, pt-BR | CH-016 targeted subset, CH-session-calm-transcript |

## Flows in Scope

- `J-13` — Follow a live session while preserving queued input and visible working state (`../journeys/J-13-follow-a-live-run.md`)
- `J-14` — Review a settled session transcript without lifecycle warning noise (`../journeys/J-14-read-a-finished-transcript.md`)

## Session Matrix & Results

| # | Charter | Journey / Scenario | Persona | Tour | Status | Issue | Fix commit |
|---|---|---|---|---|---|---|---|
| 1 | CH-016 | J-13 / RT-019 | Théo | Multi-Tab | Blocked (needs human verify) | Full two-tab/FIFO/stop charter not walked | |
| 2 | CH-016 + CH-020 | J-13 / RT-059 | Théo / Sol | Multi-Tab + Back-Button | Blocked (needs human verify) | Full multi-tab and accessibility charters not walked | |
| 3 | CH-session-calm-transcript | J-14 / ET-web-session-transcript-calm-grammar | Théo | Feature | Fixed | [BUG-20260803-prompt-cancel-warning-noise](../bugs/BUG-20260803-prompt-cancel-warning-noise.md) | PR #304 remediation batch |

Status legend: `Pending | Pass | Fixed | Skipped | Blocked (needs human verify) | Blocked (human decision)`

## Session Debriefs

### CH-016 targeted subset — Théo

- **Ran:** 2026-08-03T21:47:12-03:00 → 2026-08-03T21:50:23-03:00 (box respected: yes)
- **Findings:** Queue and edit preserved exact text plus runtime selection; a stale active-turn fence was rejected; current-fence steer and interrupt each canceled the exact turn, dispatched one replacement, and left no pending input.
- **Bugs filed/updated:** none
- **Scenarios settled:** RT-019 → blocked-verify; targeted queue/edit/fence/steer/interrupt evidence retained
- **Paper cuts:** none
- **Surprises:** The first promoted steer attempt raced the natural turn boundary and correctly returned an active-turn mismatch instead of mutating the newer turn.
- **Suggested next charter:** Complete CH-016 with two tabs, two queued prompts in FIFO order, cross-tab stop reflection, and the scroll-follow probes.

### CH-session-calm-transcript — Théo

- **Ran:** 2026-08-03T21:51:23-03:00 → 2026-08-03T21:59:01-03:00 (box respected: yes)
- **Findings:** One Working status and one queued row survived fresh navigation; edit reused the row and cleared the composer after acknowledgement. The first walk exposed one duplicate prompt-cancel warning, which was fixed and re-walked.
- **Bugs filed/updated:** [BUG-20260803-prompt-cancel-warning-noise](../bugs/BUG-20260803-prompt-cancel-warning-noise.md)
- **Scenarios settled:** ET-web-session-transcript-calm-grammar → pass after fix; RT-059 → blocked-verify because its full live and accessibility charters were not walked
- **Paper cuts:** none
- **Surprises:** The daemon’s canonical marker is singular `prompt_cancel`; the Web filter only covered the alternate spellings `prompt_canceled` and `prompt_cancelled`.
- **Suggested next charter:** Complete CH-020 as Sol with keyboard, screen-reader announcement, and reduced-motion probes.

## What Was Fixed

### BUG-20260803-prompt-cancel-warning-noise: Successful prompt replacement showed a cancellation warning

- **Symptom:** A settled steer or interrupt displayed “Prompt canceled by operator.” as a warning row beside the successful replacement response.
- **Root cause:** The Web lifecycle filter omitted the daemon’s canonical singular `prompt_cancel` marker kind.
- **Fix:** PR #304 remediation batch
- **Regression test:** `web/src/systems/session/components/__tests__/runtime-activity-notice.test.tsx` — 17/17 passed after adding the singular marker to the canonical lifecycle matrix.
- **Retested:** CH-session-calm-transcript on a fresh Web load; the warning disappeared while the replacement transcript stayed visible.

## Paper Cuts

None recorded.

## Runtime Errors Observed

- The first laboratory daemon boot had no default provider because the generic bootstrap writes only HTTP/UDS settings. The provider was set through `compozy config set defaults.provider codex`, the owned daemon was restarted, and no user journey had begun.
- A stale steer fence returned `active turn mismatch` during the edge probe. This was the expected safety result; the current fence then succeeded once.
- The strict real-scenario auditor returned `fail` under the generated `feature` profile because it requires four agents, three channels, a task run, a provider-tagged journey entry, and final gate evidence. This run is an explicitly targeted remediation walk, not a feature-grade charter pass. The observed session behavior is retained as narrow evidence; the earlier full session-input report remains the release-grade baseline.

## Human Verifications Needed

- [ ] Walk CH-016 end to end in two tabs: queue two prompts, confirm FIFO dispatch, steer, stop in one tab, and confirm the second tab converges without refresh (rows 1-2).
- [ ] Walk CH-020 as Sol with keyboard-only controls, screen-reader announcements, and reduced-motion enabled (row 2).

## Decisions for a Human

None.

## Learnings

- The persisted lifecycle kind is part of the UI contract: alternate English spellings in a renderer are not a substitute for covering the daemon’s canonical value.
- A stale fence rejection during a natural turn boundary is useful user evidence that steer cannot silently target a newer turn.

## Final Status

- **Exit gate (full automated suite):** pending final `make gate-full`
- **Issues by user impact:** Blocks-Completion 0 · Data-Loss 0 · Trust-Damage 0 open (1 fixed and re-walked) · Friction 0 · Cosmetic 0
- **Coverage:** targeted behavior observed across live Codex, CLI, HTTP, Web, runtime, fresh navigation, stale-fence edge, and visual inspection; 1 scenario fixed and passed, 2 broader charters blocked for human verification
- **Verdict:** pending
