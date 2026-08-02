# QA Run Report — 2026-08-02 — knowledge-refresh-on-wake

- **Scope:** Sanity re-walk of the fixed live-turn workspace-knowledge contract on one real synthetic Heartbeat wake.
- **Cadence tier:** sanity
- **Build:** `v0.3.0-beta.3-6-g741d3563-dirty` · **Environment:** fresh isolated lab `compozy-knowledge-refresh-on-wake-20260803-025914-822792-lab`, native Codex provider, CLI/UDS/runtime surfaces, no Web server
- **Started:** 2026-08-02T23:56:14-03:00 · **Status:** closed

## Personas

| Persona | Base | Device / Network / Locale | Sessions |
|---|---|---|---|
| Bruno | Delivery Builder | desktop / wifi-fast / en-US | `CH-refresh-agent-knowledge` |

## Flows in Scope

- `J-refresh-agent-knowledge` — an active agent receives current workspace reference bytes on its next eligible synthetic wake (`../journeys/J-refresh-agent-knowledge.md`).

The broader `J-one-kickoff-collaboration` replay is not part of this sanity round. Its autonomous
completion failure remains owned by `BUG-0028`; mixing that 30-minute multi-agent run into this
five-minute freshness verdict would hide which boundary failed.

## Session Matrix & Results

| # | Charter | Journey / Scenario | Persona | Tour | Status | Issue | Fix commit |
|---|---|---|---|---|---|---|---|
| 1 | `CH-refresh-agent-knowledge` | `J-refresh-agent-knowledge` / `TA-agent-knowledge-refresh-on-wake` | Bruno | Feature Tour | Fixed | `BUG-20260729-agent-knowledge-refresh-missed` | pending final whole-diff commit |

Status legend: `Pending | Pass | Fixed | Skipped | Blocked (needs human verify) | Blocked (human decision)`

## Session Debriefs

### CH-refresh-agent-knowledge — Bruno

- **Ran:** 2026-08-03T03:00:41Z → 2026-08-03T03:05:12Z (box respected: yes)
- **Findings:**
  - The live behavior passed. The initial provider response reported `CURRENT_CANDIDATE_MS=410`. After the workspace Markdown changed to `500 ms`, the first public Heartbeat wake produced `CURRENT_CANDIDATE_MS=500` 19.594 seconds after the file write and 1.801 seconds after wake admission.
  - `session events` and a fresh `session recap` independently returned the assembled `500` response. Session health returned to `idle` and `healthy`, remained attachable and wake-eligible, and Heartbeat status retained `wake_sent`.
  - Session history contains exactly one `user_message` and one `synthetic_reentry`; no second operator prompt was sent.
  - The strict generic feature-profile auditor correctly failed with nine blockers because this one-agent sanity charter does not supply the release-wide minimums of four actors, three differentiated roles, three channels, a Task run, API/Web evidence, cross-surface object parity, or downstream artifact reuse. It also reports the intentionally deferred final gate. These blockers qualify the overall QA round but do not contradict the observed knowledge-freshness behavior.
- **Bugs filed/updated:** `BUG-20260729-agent-knowledge-refresh-missed` verified; no new bug.
- **Scenarios settled:** `TA-agent-knowledge-refresh-on-wake → pass`.
- **Paper cuts:** none.
- **Surprises:** provider output arrived as two durable `agent_message` chunks; the public recap correctly reassembled them into one response.
- **Suggested next charter:** re-run `CH-helix-one-kickoff-bruno` after `BUG-0028` is fixed so the release-grade multi-agent profile can close without mixing completion debt into this sanity verdict.

#### Feature Tour and edge probes

- The documented happy path — agent definition, managed Heartbeat policy, live session, one wake, events, recap, health, and status — matched the public contract.
- Relevant session/timing probes were clean: an idle session before wake; a file change that did not self-wake; stale transcript context coexisting with current prompt data; chunked provider output reassembled by recap; repeated public reads returning the same settled value; and post-turn health returning to idle.
- Browser, viewport, locale, form, and navigation edges do not apply to this CLI/runtime-only charter. The two-journey experiential lens pass was not claimed for this one-journey sanity round.

## What Was Fixed

### BUG-20260729-agent-knowledge-refresh-missed: Active worker misses changed workspace knowledge

- **Symptom:** An active worker used stale workspace knowledge until a much later reread.
- **Root cause:** Workspace knowledge was absent from prompt composition, synthetic turns bypassed every augmenter, and prompt dispatch did not reconstruct ACP synthetic metadata for harness policy.
- **Fix:** current worktree; final whole-diff commit pending.
- **Regression test:** `internal/daemon/prompt_input_composite_integration_test.go` — `TestPromptInputCompositeIntegrationRefreshesWorkspaceKnowledgeOnSyntheticWake` failed before the fix and now passes under `-race` with the integration tag.
- **Retested:** `J-refresh-agent-knowledge` passed with native Codex session `sess-f75e342442812f4d`; evidence is `/home/pedronauck/dev/qa-labs/compozy-knowledge-refresh-on-wake-20260803-025914-822792-lab/qa-artifacts/qa/knowledge-refresh-evidence.json`.

## Paper Cuts

None.

## Runtime Errors Observed

None. The strict audit blockers are scope/evidence-profile gaps, not runtime errors.

## Human Verifications Needed

None. The native Codex provider was reachable.

## Decisions for a Human

None.

## Learnings

- Planning separated knowledge freshness from the existing autonomous completion failure so each bug retains one observable owner.
- A generic bootstrap always emits the release-grade `feature` profile. Narrow sanity walks can settle their named behavior, but they cannot claim release-grade completion from that auditor without the wider actor/channel/surface contract.
- Mandatory teardown passed. `/home/pedronauck/dev/qa-labs/compozy-knowledge-refresh-on-wake-20260803-025914-822792-lab/qa-artifacts/qa/teardown.json` records `clean=true`, daemon PID `969933` stopped, and zero survivors.

## Final Status

- **Exit gate (full automated suite):** BLOCKED for this round — intentionally deferred until the modernization workstream's last mutation; focused race/integration evidence is green.
- **Strict evidence audit:** FAIL with nine blockers — `/home/pedronauck/dev/qa-labs/compozy-knowledge-refresh-on-wake-20260803-025914-822792-lab/qa-artifacts/qa/qa-audit-report.json`; the remaining blockers are eight generic release-grade profile minimums plus final-gate evidence.
- **Lab teardown:** PASS — `/home/pedronauck/dev/qa-labs/compozy-knowledge-refresh-on-wake-20260803-025914-822792-lab/qa-artifacts/qa/teardown.json` has `clean=true` and no survivors.
- **Issues by user impact:** Blocks-Completion 0 · Data-Loss 0 · Trust-Damage 0 open / 1 verified · Friction 0 · Cosmetic 0
- **Coverage:** 1/1 in-scope journeys walked; the separate release-grade one-kickoff journey remains outside this sanity round.
- **Verdict:** BLOCKED — the named knowledge-refresh behavior is fixed and verified, but this narrow lab cannot satisfy the generic release-grade evidence profile and the workstream's final gate is intentionally pending.
