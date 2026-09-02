# QA Run Report — 2026-09-01 — Window management hardening

- **Scope:** Window zoom model (in place or on its own desktop), minimize and restore ladder, edge
  and head drop gestures, version 3 arrangement migration, and layout stream liveness across
  Web, HTTP, CLI, and native-tool surfaces.
- **Cadence tier:** targeted
- **Build:** `a1baedd3a` on branch `wm-hardening` · **Binary SHA-256:** `aa2bd15d2cda92cbbb2df19932c1dc4cf5b5dbefa8d37a2e34e15ca6657303f3`
- **Environment:** isolated `COMPOZY_HOME`, HTTP port `65481`, UDS socket, Vite dev server on port
  `3000` proxied to the lab daemon, workspace `wm-lab` (`ws_f30dc74355bb5da8`), default profile.
- **Status:** complete; delivery CI follows the push

## Session

| Scenario | Persona | Walk | Status |
|---|---|---|---|
| ET-window-zoom-in-place | Bruno | zoom walk A0–A8 (Playwright against the lab) | Pass |
| ET-window-manager-layout-gestures | Bruno | zoom walk A0b, A2, A4 and parity walk P4 | Pass |
| RT-desktop-pager-overview | Théo | zoom walk pager counts and parity walk P2 | Pass |
| ET-window-manager-public-parity | Ada | parity walk P1, P2 and zoom walk A7 (CLI) | Pass |
| ET-window-tab-v3-discard | Ada | parity walk P5a, P5b, P5c | Pass |
| ET-window-manager-multi-client | Bruno | parity walk P2 (two browser clients) | Pass |
| ET-web-desktop-shell-lifecycle | Bruno | stream walk B1–B7 and parity walk P5a page follow | Pass |
| RT-window-manager-stream-liveness | Bruno | stream walk B1–B7 | Pass |

## Results

### Zoom is structural again

Pedro reported two defects after live use of the flag-only zoom that landed earlier in this
workstream: tiling a second window to a screen edge dropped the zoomed window back to its old
floating rect instead of giving it the remaining half, and zooming again with a tiled neighbour
covered that neighbour. The model now follows the behaviour he described. The zoomed unit (a
window or its whole tab frame) becomes the only full-frame island of a desktop. When the window's
desktop shows nothing else it zooms in place; when another window is visible it moves to a fresh
regular desktop inserted right after the current one, and the issuing client follows it. Unzoom
returns the unit to the slot it left through the existing return-anchor ladder and removes the
desktop the zoom created once it is empty; closing the zoomed unit removes that desktop the same
way. Tiling another window to a screen edge shrinks the zoomed island to the free zone and ends the
zoom. Minimizing a solo zoomed window keeps its zoom origin so restore re-zooms it on the same
desktop and unzoom still takes it home; a zoomed tab leaves its zoom and origin with the deck.

The zoom walk passed 11 of 11 checks on the final binary: A0a solo zoom in place, A0b edge tile
hands the zoomed window the remaining half, A0c zoom beside a tiled neighbour lifts and unzoom
returns, A1 lift with the client following, A2 head drop folds into a still-zoomed deck on the
lifted desktop, A3 unzoom returns the deck to its split slot and drops the lifted desktop, A4 edge
tile over a zoomed desktop shrinks the deck island and keeps the desktop, A5 a minimized tab
restores into the zoomed deck, A6 closing the zoomed tab keeps the deck zoomed, A7 `compozy window
zoom` without `--client` toggles and `compozy window list` shows `zoomed`, A8 closing the zoomed
window releases the desktop the zoom created.

### Version 3 arrangements migrate under a new revision

The parity re-walk exposed a second defect: the daemon migrated a stored version 3 arrangement in
memory without changing its revision, so a browser that reconnected after the restart saw the same
revision in the stream fence and kept the pre-migration layout in its cache. The migration now
yields the next revision and the daemon persists the migrated arrangement once at load, and the
web accepts an authoritative fence even when its revision equals the cache. A former focus desktop
stays as a regular desktop hosting its owner as a lifted zoom, so unzoom takes the owner home and
drops that desktop. Parity walk P5a proves the load, the page follow without reload, and the CLI
unzoom back into the exact source split; P5b and P5c keep the version 3 apply rejection and the
version 2 discard.

### Restore ladder and earlier live findings

The return ladder now skips an origin neighbour that is no longer tiled instead of refusing the
whole return. The two defects found by the earlier live walks in this workstream stay fixed: a
structural removal prunes an empty split so the island cannot block a frame placed over it, and a
stream client whose remembered revision is ahead of the daemon receives the current snapshot.

### Stream liveness

The stream walk passed 7 of 7 checks on the final binary: B1 heartbeat frames carry the current revision, B2 a 409 conflict refreshes the snapshot with a transient notice and reopens the surface, B3 a heartbeat carrying a newer revision triggers a refetch, B4 an online event after 30 seconds of silence reconnects at once, B5 the stall watchdog reconnects a socket that stops delivering frames after 75 seconds, B6 a drop released after another client advanced the revision still applies and the next drag works, B7 a daemon restart reconnects and re-renders the windows without a reload.

### Cross-surface parity

Parity walk P1 (CLI desktop create/delete without purpose flags, list carries `zoomed`), P1b
(`compozy layout watch` stays attached across a heartbeat interval), P2 (two browser clients: one
follows the lift, the peer stays, a clientless CLI unzoom returns the window and only repairs the
lifted client's view), and P4 (corner tile keeps the larger band, a middle cut is refused with a
typed diagnostic) passed. The Playwright shell specs E2E-003 and E2E-137 were rewritten for the
structural model and pass with E2E-041.

## Verification

- Go race suites passed for `internal/windowmanager` (new `TestZoom` suite of 15 cases plus the
  migration cases), the daemon window-manager repository and native-tool suites, `internal/api/core`,
  `internal/api/contract`, `internal/api/spec`, `internal/cli`, and `internal/tools/builtin`.
- The web stream hook suite passed (18 cases) including the new equal-revision fence regression,
  which was mutation-checked by reverting the fix; web typecheck and repo-root Bun lint passed.
- Scoped `golangci-lint` reported zero issues in the touched packages.
- `make gate` passed on the committed tree; the record lives under `.cache/gate/` and a copy of the
  log under `qa/logs/make-gate.log`.
- The strict real-scenario evidence audit passed.
- The bootstrap-provided teardown command completed with `qa/teardown.json` reporting
  `clean: true`.

Evidence is rooted at the lab path
`/Users/pedronauck/dev/qa-labs/compozy-window-manager-hardening-20260901-200758-934379-lab/qa-artifacts/qa/`;
the receipts are `test-cases/walk-window-manager-zoom-results.json`,
`test-cases/walk-parity-results.json`, `test-cases/walk-window-manager-stream-results.json`,
`logs/walk-window-manager.jsonl`, `logs/walk-parity.jsonl`, `logs/cli-parity.log`,
`logs/daemon-migration-lines.log`, `screenshots/`, `journey-log.jsonl`, `qa-audit-report.json`, and
`teardown.json`.

## Runtime observations

The lab had no provider credentials, so provider health probes were degraded; no provider session
was needed for these window-manager charters. Two pre-existing `gosec` G118 findings in untouched
daemon files appear under the local `golangci-lint` v2.11.4 and not under the pinned version the
gate runs.

## Compozy Impact Audit

- **Native tools:** `compozy__window_zoom` keeps its ID and input schema; its description now says
  the window zooms in place or on a fresh desktop and that unzoom returns it. No digest, risk flag,
  or capability gate changed; the native-tool catalog golden is unchanged.
- **Extensibility and hooks:** the stream contract is unchanged in this pass (the `heartbeat` frame
  landed earlier in the workstream); no hook, extension resource, bridge SDK, or MCP surface changed.
  A stored version 3 arrangement now persists as version 4 under the next revision at load.
- **Workspace data isolation:** `zoomed` and `return_anchor` stay window-scoped inside the
  per-(workspace, profile) arrangement; the desktop a zoom creates lives in the same document. No
  cross-workspace list, read, cache, stream, or event path changed.
- **Official Compozy skill:** `skills/compozy/references/window-management.md` describes the
  structural zoom, the desktop it creates and releases, and the migration revision bump.

## Final status

- **Scenario verdict:** PASS
- **Issues by user impact:** Blocks-Completion 0 · Data-Loss 0 · Trust-Damage 0 · Friction 0 · Cosmetic 0
- **Human verification needed:** Pedro should confirm the desktop the zoom creates matches his
  intent when the zoomed window's desktop is left with only minimized windows.
- **Cleanup:** Complete and clean
