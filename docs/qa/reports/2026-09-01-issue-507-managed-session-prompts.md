# QA Run Report — 2026-09-01 — issue-507-managed-session-prompts

- **Scope:** Web prompts and busy-turn intervention for public daemon-managed sessions, with user-session continuity as the adjacent canary.
- **Cadence tier:** targeted
- **Build:** `1881a9f54` + issue #507 working tree · **Environment:** isolated local daemon and Web at `http://127.0.0.1:61849`; real provider required; desktop Chrome, wifi-fast, en-US
- **Started:** 2026-09-01T13:21:55Z · **Status:** pass

## Personas

| Persona | Base | Device / Network / Locale | Sessions |
|---|---|---|---|
| Théo | Power User | desktop / wifi-fast / en-US | CH-managed-session-intervention, CH-stopped-session-prompt-continuity |

## Flows in Scope

- `J-13` — Follow, correct, and continue a live durable session (`../journeys/J-13-follow-a-live-run.md`).

## Session Matrix & Results

| # | Charter | Journey / Scenario | Persona | Tour | Status | Issue | Fix commit |
|---|---|---|---|---|---|---|---|
| 1 | CH-managed-session-intervention | J-13 / RT-018, RT-019 | Théo | Feature Tour | Pass | | |
| 2 | CH-stopped-session-prompt-continuity | J-13 / RT-018 (user-session canary) | Théo | Interrupt Tour | Pass | | |

Status legend: `Pending | Pass | Fixed | Skipped | Blocked (needs human verify) | Blocked (human decision)`

## Session Debriefs

- **Managed session:** Théo opened Loop-owned Codex system session
  `sess_48c88cfca7f77a33411837a527fc38f9` through the workspace Sessions picker. The active composer
  exposed Queue, Steer, Interrupt, and Stop generation. Each action affected the live turn; Stop
  generation returned the session to ordinary Send. Refresh and a new Send preserved the permalink,
  session ID, prior transcript, and runtime selection. Rename, clear, attach, delete, archive, and
  whole-session stop were absent.
- **User-session canary:** Théo opened `sess-3ca568309e02cf23`, started and stopped a turn, then sent a
  follow-up from the stopped composer. The follow-up resumed in the same permalink and history.
- **Provider proof:** Codex ACP session `01a05d29-4f87-7e70-a39a-2481b030d1a7` changed its plan after
  operator steer and interrupt input, prioritizing recovery verification before continuing the guide.

## What Was Fixed

No additional production fix was needed during the walk.

## Paper Cuts

None recorded.

## Runtime Errors Observed

Expected prompt cancellations appeared after Steer, Interrupt, and Stop generation. The owning Loop
later reported `prompt already in progress` while the operator-controlled replacement turn was active;
the session itself remained healthy and resumable, so this did not invalidate the changed Web contract.

## Human Verifications Needed

None identified.

## Decisions for a Human

None identified.

## Learnings

- Taxonomy plan: the J-13 journey owns functional and continuity coverage; the Feature Tour covers the changed managed-session authority boundary; refresh and close/reopen cover abandonment and recovery; the existing user-session charter is the regression canary. Responsive layout and locale changes are skipped because this diff changes no layout or copy.
- The managed session authority boundary remained clear in practice: prompt intervention was available,
  but session ownership actions remained absent.
- Stop generation and whole-session stop are visibly and behaviorally separate; cancellation returned
  the same session to Send without ending or replacing it.

## Final Status

- **Exit gate (full automated suite):** PASS — `make gate`; final evidence:
  `/Users/pedronauck/dev/qa-labs/compozy-issue-507-managed-session-prompts-20260901-132128-970654-lab/qa-artifacts/qa/evidence/issue-507/final-make-verify.log`.
- **Strict evidence audit:** PASS —
  `/Users/pedronauck/dev/qa-labs/compozy-issue-507-managed-session-prompts-20260901-132128-970654-lab/qa-artifacts/qa/qa-audit-report.md`.
- **Teardown:** PASS — `teardown.json` records `clean: true` with no survivors.
- **Issues by user impact:** Blocks-Completion 0 · Data-Loss 0 · Trust-Damage 0 · Friction 0 · Cosmetic 0
- **Coverage:** 1/1 journeys walked; 2/2 session rows passed.
- **Verdict:** QA PASS — exact-head PR CI remains the delivery gate.
