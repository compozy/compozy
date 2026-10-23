# QA Run Report — 2026-10-23 — Session total active work

- **Scope:** Issue #920, own-turn active duration in the session inspector.
- **Cadence tier:** targeted.
- **Build:** d68ba8f1c plus the issue implementation (uncommitted).
- **Environment:** isolated daemon and Web development server; native Codex authentication remains operator-owned.
- **Started:** 2026-10-23T17:15:00Z · **Status:** closed.

## Mission and persona

Rafa operates a session across several turns and idle gaps. Confirm that Activity distinguishes the current turn from total measured work, without attributing child work or idle time to the session. Inspect the rail at 1440px and the drawer at 1280px, including reload and cancellation.

## Session Matrix & Results

| Journey / Scenario | Probe | Status |
|---|---|---|
| J-14 / ET-web-session-context-sidebar | Completed turns match durable delivery-to-terminal intervals | Pass |
| J-14 / ET-web-session-context-sidebar | Live total advances; idle total freezes; reload preserves total | Pass |
| J-14 / ET-web-session-context-sidebar | Rail and drawer render current and total duration clearly | Pass |

## Verification evidence

- Real SQLite recorder/reopen integration coverage passed with the race detector.
- Usage endpoint contract suite passed with the race detector.
- Root Turbo: 68 focused Web tests and Web typecheck passed.
- React Doctor changed-file scan: 100/100, no findings.

## Live results

- Two completed native Codex turns measured 5,574 ms and 2,299 ms; `/usage` reported 7,873 ms across two turns. The idle gap was excluded.
- At 1440px the rail showed `Worked for 2s` and `Total work 7s across 2 turns`: [idle rail](../evidence/2026-10-23-issue-920-total-work/idle-1440.png).
- After reload at 1280px, the third turn showed current/total pairs of 2s/10s and 16s/24s, with three turns: [live drawer](../evidence/2026-10-23-issue-920-total-work/live-1280.png). The existing clock drives both labels; no interval fetch was added.
- Cancellation was accepted. A subsequent daemon reopen reported 38,953 ms across three turns: 5,574 + 2,299 + 31,080 ms. The last interval ends at durable `session.turn_quiesced`, after provider cancellation completes. The immediate cancellation response can precede that persisted terminal boundary.
- The session list retained its existing last-activity label.
- This targeted walk covers the new total-work probes in step 6, not every pre-existing scenario leg. Full CI validation is pending.

## Validation ownership

The operator moved heavy validation to CI to protect host responsiveness. The local gate was interrupted; it is not claimed green. Earlier focused race tests and code generation had completed before that override. Subsequent local work is limited to inexpensive scoped checks; full Go/Web/E2E gates belong to PR CI.

## Runtime observations

- The fresh lab initially refused session creation because no default provider was configured. Set the lab's provider through the public configuration CLI and restarted only the lab daemon.

## Teardown

Registered teardown completed with `TEARDOWN_CLEAN=true` and `TEARDOWN_ALL_CLEAN=true`. Both the daemon and Web server were stopped.
