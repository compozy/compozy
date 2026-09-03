# QA Run Report — 2026-09-01 — integrated terminal review round 2

- **Scope:** Deep-review round 2 remediation for terminal runtime, CLI/UDS, browser lifecycle, stream safety, profile isolation, and desktop routing
- **Cadence tier:** targeted
- **Build:** `29705c65faee` plus current review remediation worktree · **Environment:** isolated lab `integrated-terminal-review-r2-20260902-020216-937662`, daemon `127.0.0.1:51632`, browser-use; local source build, no production-parity external provider
- **Started:** 2026-09-01T23:01:07-03:00 · **Status:** in-progress

## Personas

| Persona | Base | Device / Network / Locale | Sessions |
|---|---|---|---|
| Bruno | isolated seeded project | desktop / wifi-fast / en-US | CH-terminal-operator-shell |
| Marina | isolated seeded project | desktop / wifi-fast and flaky / en-US | CH-terminal-stream-flow-control, CH-terminal-lease-fencing-takeover |
| Dora | isolated seeded project | desktop / wifi-fast / en-US | CH-terminal-redaction-osc-boundary |
| Ada | isolated seeded project with two profiles | desktop / wifi-fast / en-US | CH-terminal-profile-fence |
| Théo | isolated seeded project | desktop / wifi-fast / en-US | CH-terminal-window-tabs-canary |

## Flows in Scope

- `J-operate-integrated-terminal` — open, use, recover, and observe persistent project terminals (`../journeys/J-operate-integrated-terminal.md`)
- `J-supervise-agent-terminal` — supervise agent control, handoff, and hidden input safely (`../journeys/J-supervise-agent-terminal.md`)
- `J-switch-profile-terminal-scope` — preserve terminal isolation while switching profile scope (`../journeys/J-switch-profile-terminal-scope.md`)
- `J-organize-tabbed-work` — adjacent desktop routing and tab restoration canary (`../journeys/J-organize-tabbed-work.md`)

## Session Matrix & Results

| # | Charter | Journey / Scenario | Persona | Tour | Status | Issue | Fix commit |
|---|---|---|---|---|---|---|---|
| 1 | CH-terminal-operator-shell | J-operate-integrated-terminal / ET-terminal-browser-lifecycle | Bruno | Feature Tour | Pass | | |
| 2 | CH-terminal-stream-flow-control | J-operate-integrated-terminal / ET-terminal-stream-resilience | Marina | Multi-Tab Tour | Pass | | |
| 3 | CH-terminal-lease-fencing-takeover | J-supervise-agent-terminal / ET-terminal-lease-fencing | Marina | Interrupt Tour | Pass | | |
| 4 | CH-terminal-lease-fencing-takeover | J-supervise-agent-terminal / ET-terminal-agent-handoff-input | Marina | Interrupt Tour | Fixed | BUG-20260901-private-passphrase-session-composer; BUG-20260902-private-input-shell-leak | pending remediation batch |
| 5 | CH-terminal-redaction-osc-boundary | J-supervise-agent-terminal / ET-terminal-redaction-boundaries | Dora | Garbage Tour | Fixed | BUG-20260902-private-input-shell-leak | pending remediation batch |
| 6 | CH-terminal-profile-fence | J-switch-profile-terminal-scope / ET-terminal-profile-selectors | Ada | Garbage Tour | Pass | Archived-read wording excluded by review directive; current product returns typed `profile_archived` | |
| 7 | CH-terminal-window-tabs-canary | J-organize-tabbed-work / ET-web-window-routing-lifecycle | Théo | Interrupt Tour | Fixed | BUG-20260902-background-window-stream-starvation | pending remediation batch |

Status legend: `Pending | Pass | Fixed | Skipped | Blocked (needs human verify) | Blocked (human decision)`

## Session Debriefs

### CH-terminal-operator-shell — Bruno

- **Ran:** 2026-09-01T23:08:56-03:00 → 2026-09-01T23:11:20-03:00 (box respected: yes)
- **Findings:**
  - No functional divergence. The dock created a working terminal directly; two terminals survived reload as separate tabs; closing the window preserved both processes; dock reopening selected the newest running terminal; an already-ended terminal closed quietly.
- **Bugs filed/updated:** none
- **Scenarios settled:** ET-terminal-browser-lifecycle → pass
- **Paper cuts:** none
- **Surprises:** none
- **Suggested next charter:** CH-terminal-stream-flow-control

### CH-terminal-profile-fence — Ada

- **Ran:** 2026-09-01T23:12:01-03:00 → 2026-09-01T23:14:20-03:00 (box respected: yes)
- **Findings:**
  - No scope leak. CLI, HTTP, and UDS agreed on default, named-profile, and aggregate rows; conflicting and missing selectors failed closed; a foreign terminal id appeared as absent.
  - The old archived-read expectation was excluded under the explicit review boundary. Current read and write behavior both return an actionable `profile_archived` refusal.
- **Bugs filed/updated:** none
- **Scenarios settled:** ET-terminal-profile-selectors → pass for the round-2 scope
- **Paper cuts:** none
- **Surprises:** the current archive policy is stricter than the old scenario wording; not treated as a defect in this review.
- **Suggested next charter:** CH-terminal-lease-fencing-takeover

### CH-terminal-agent-handoff-input — Marina (initial walk)

- **Ran:** 2026-09-01T23:19:31-03:00 → 2026-09-01T23:21:35-03:00 (box respected: yes)
- **Findings:**
  - The agent opened the visible terminal and completed the requested command, but then asked for the
    private passphrase in ordinary session chat instead of creating a redacted terminal input request.
- **Bugs filed/updated:** BUG-20260901-private-passphrase-session-composer
- **Scenarios settled:** none; ET-terminal-agent-handoff-input remains untested pending the fix replay.
- **Paper cuts:** none
- **Surprises:** the terminal reference described program-driven input but did not bind agent-driven
  private input to the same protected surface.
- **Suggested next charter:** repeat the same prompt with a fresh agent after the skill fix.

### CH-terminal-agent-handoff-input — Marina (fresh-agent replay)

- **Ran:** 2026-09-02T02:42:00-03:00 → 2026-09-02T02:47:00-03:00 (box respected: yes)
- **Findings:**
  - The fresh agent loaded the changed official skill and correctly created a redacted terminal input
    request instead of using session chat.
  - The request was created at an ordinary fish prompt. Submitting the protected card displayed and
    executed the passphrase, then public journal and quote reads returned the raw value.
- **Bugs filed/updated:** BUG-20260901-private-passphrase-session-composer progressed to fix replay;
  BUG-20260902-private-input-shell-leak filed.
- **Scenarios settled:** none; both handoff and redaction remain failed until the runtime boundary is
  fixed and replayed from scratch.
- **Paper cuts:** none
- **Surprises:** temporary kernel echo suppression cannot protect input from a shell line editor that
  renders its own buffer.
- **Suggested next charter:** replay with a foreground program that has already entered hidden-input
  mode after the runtime rejects idle-shell requests.

### CH-terminal-lease-fencing-takeover — Marina

- **Ran:** 2026-09-02T00:08:00-03:00 → 2026-09-02T01:02:00-03:00 (box respected: yes)
- **Findings:**
  - A human-owned terminal rejected the bound agent's write with structured `lease_revoked`; the
    refused marker never reached the terminal, journal, or quote surfaces and the agent did not open a
    replacement terminal.
  - Closing one of two controlling views preserved control. Closing the last view released it after
    the documented grace period. A later agent run remained fenced from a terminal bound to the
    earlier run, again with no write side effect.
  - Separate human clients exercised confirmed takeover, cancellation, forced CLI takeover, and
    watch-only attach without producing overlapping writers or watcher input.
- **Bugs filed/updated:** none
- **Scenarios settled:** ET-terminal-lease-fencing → pass
- **Paper cuts:** none
- **Surprises:** release follows the attachment grace period rather than the visual window-close edge.
- **Suggested next charter:** CH-terminal-stream-flow-control

### CH-terminal-stream-flow-control — Marina

- **Ran:** 2026-09-02T01:06:00-03:00 → 2026-09-02T01:25:00-03:00 (box respected: yes)
- **Findings:**
  - Three browser viewers attached to one terminal. With one watcher offline, the controller completed
    a 6,000-line command and the public CLI quote independently ended at `6000`.
  - The watcher first showed `Reconnecting…`, then recovered the retained stream through `6000` with
    no browser error, no control transfer, and no change from the controller's `80×24` dimensions.
  - Disconnecting the acknowledging controller did not freeze the remaining watcher or the program;
    both the watcher and controller converged on the completed eight-packet command after reconnect.
  - Catalog cursor `10` replayed only event `11`; cursor `0` returned a full `terminal.snapshot`.
    Attach passes upgraded once, then refused reuse, expiry, another terminal, and another mode before
    opening a connection.
- **Bugs filed/updated:** none
- **Scenarios settled:** ET-terminal-stream-resilience → pass
- **Paper cuts:** none
- **Surprises:** xterm accepts real key events from the automation driver, while direct textarea fills
  do not model user typing and were discarded from the session evidence.
- **Suggested next charter:** CH-terminal-redaction-osc-boundary

### CH-terminal-redaction-osc-boundary — Dora

- **Ran:** 2026-09-02T01:26:00-03:00 → 2026-09-02T01:38:00-03:00 (box respected: yes)
- **Findings:**
  - A recording started before a 16-character hidden answer retained only
    `[8.038709,"m",{"characters":16,"kind":"redacted_input"}]`. Screen, quote, journal,
    daemon log, and the isolated runtime tree contained no raw value.
  - A 300,012-byte output forced spill artifact `art-8b9d1e5467fba9541e20ca626a19610b`.
    The stored file was mode `0600`, contained `[REDACTED]`, and matched its public content digest.
    Replacing its path temporarily with a symbolic link to `/etc/hosts` made the public download
    refuse the artifact; the original file was restored and its digest rechecked.
  - A recorded hostile-output terminal preserved `OSC-TITLE-VISIBLE`, `OSC52-VISIBLE`, and
    `DCS-VISIBLE`, removed the OSC52 and DCS payloads, bounded the terminal title to 256 characters,
    and returned an untrusted quote containing only the visible markers.
- **Bugs filed/updated:** BUG-20260901-private-passphrase-session-composer and
  BUG-20260902-private-input-shell-leak → fixed after fresh replay.
- **Scenarios settled:** ET-terminal-agent-handoff-input → pass; ET-terminal-redaction-boundaries → pass
- **Paper cuts:** none
- **Surprises:** the protected-input boundary must be established by the foreground program; the
  runtime cannot make an interactive shell's own line editor private after the fact.
- **Suggested next charter:** CH-terminal-window-tabs-canary

### CH-terminal-window-tabs-canary — Théo

- **Ran:** 2026-09-02T01:38:00-03:00 → 2026-09-02T01:52:00-03:00 (box respected: yes)
- **Findings:**
  - The initial walk exposed browser connection starvation: three covered Session windows retained
    transcript streams, a window command stayed locally pending, and no matching request reached the
    daemon. The same zero-request stall reproduced in a second clean browser client.
  - After active-desktop ownership became a bounded pair of the focused window plus the most recent
    eligible background window, Terminal Dock activation reached `term-55c8821372eb`, Home minimized
    and restored with the correct successor, and URL focus stayed aligned.
  - Reload restored the same terminal route and all ten windows. The window-manager stream moved from
    disconnected to connected without creating a duplicate instance or leaving a partial topology.
- **Bugs filed/updated:** BUG-20260902-background-window-stream-starvation → fixed
- **Scenarios settled:** ET-web-window-routing-lifecycle → pass
- **Paper cuts:** none
- **Surprises:** a covered floating window was retained as visually mounted and incorrectly treated
  as entitled to a continuous network stream.
- **Suggested next charter:** none; targeted matrix complete

## Experiential Lens Pass

The two widest changed journeys, `J-operate-integrated-terminal` and
`J-supervise-agent-terminal`, received the six-lens re-read:

| Lens | J-operate-integrated-terminal | J-supervise-agent-terminal |
|---|---|---|
| Usability | Pass — open, reconnect, minimize, and restore states remained visible and reversible. | Pass — watcher, controller, protected answer, and decline states named the current owner and action. |
| Accessibility | Pass — tested controls exposed stable button names, headings, statuses, and keyboard-operable actions. | Pass — the protected card, takeover controls, and terminal traffic lights remained labelled and keyboard reachable. |
| Perceived performance | Fixed — the connection-starvation stall was removed; subsequent Dock and traffic-light commands settled promptly. | Pass — stream flood, offline recovery, and protected input preserved immediate state feedback. |
| Compatibility | Pass (targeted scope) — latest Chrome at the desktop viewport; no layout or styling change was introduced. | Pass (targeted scope) — latest Chrome at the desktop viewport; terminal control remained pointer and keyboard operable. |
| Error recoverability | Pass — reconnect preserved the retained output, route, focused terminal, and ten-window topology. | Pass — visible-input misuse failed closed, decline stayed final, and reconnect never reassigned control. |
| Production parity | Pass with qualification — real local daemon, SQLite, HTTP, WebSocket, and SSE; Vite remained a documented local-build difference. | Pass with qualification — real terminal processes and public transports; provider behavior was limited to the isolated local agent run. |

## What Was Fixed

- Official terminal skill routing now sends private agent-driven questions to the protected terminal
  request surface.
- Redacted input now fails closed unless the foreground program is already hiding input, so private
  values cannot be delivered to an ordinary shell line editor.
- Live-data ownership is limited to the focused window and one most-recent eligible background
  window, preserving useful background updates without letting retained windows starve desktop
  commands of browser connections.

## Paper Cuts

| Persona | Where (journey/step) | Felt | Sharpness | Outcome |
|---|---|---|---|---|

## Runtime Errors Observed

- No browser console or page errors during CH-terminal-operator-shell.
- The first extra-tab attempt crossed a Vite HMR invalidation and logged failed dynamic imports for the
  Terminal and Session window modules. A single clean-session retry loaded both modules, the daemon
  stream was independently healthy, and the CPU profile was idle; this is recorded as a development
  server artifact, not product evidence.
- The routing canary reproduced a real zero-request stall from background-window connection
  starvation. BUG-20260902-background-window-stream-starvation records the fix and fresh-browser
  replay; it is not classified as a Vite artifact.

## Human Verifications Needed

- None identified before execution.

## Decisions for a Human

None identified before execution.

## Learnings

- A browser window close and a terminal process close are visibly distinct and remained consistent with CLI reads.

## Final Status

- **Exit gate (full automated suite):** pending
- **Issues by user impact:** Blocks-Completion 0 · Data-Loss 0 · Trust-Damage 0 · Friction 0 · Cosmetic 0
- **Coverage:** 4/4 journeys walked; 0 scenario rows pending
- **Verdict:** QA passed after three in-session fixes; source freeze, repository gates, commit, and
  exact-head delivery checks remain pending.
