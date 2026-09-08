# QA Run Report — 2026-09-04 — terminal-shared-control

- **Scope:** Hard cut from exclusive terminal ownership to shared input across runtime, CLI/API, native tools, hooks, Web, and docs
- **Cadence tier:** targeted
- **Build:** working tree from `23b1626ae` · **Environment:** isolated targeted lab `compozy-terminal-shared-control-20260904-204013-041114-lab`
- **Started:** 2026-09-04T20:36:46Z · **Status:** complete

## Personas

| Persona | Base | Device / Network / Locale | Sessions |
|---|---|---|---|
| Marina | Casual User | desktop / wifi-fast / en-US | CH-terminal-shared-input-race |
| Ada | Power User | desktop / wifi-fast / en-US | CH-terminal-v3-public-contract |
| Lea | New User | laptop / wifi-fast / en-US | CH-terminal-shared-control-docs |
| Théo | Power User | desktop / wifi-fast / en-US | CH-terminal-window-tabs-canary |

## Flows in Scope

- `J-supervise-agent-terminal` — work alongside an agent in one shared terminal (`../journeys/J-supervise-agent-terminal.md`)
- `J-operate-terminal-by-cli` — complete terminal work through the command line and public transports (`../journeys/J-operate-terminal-by-cli.md`)
- `J-learn-terminal-from-docs` — follow current terminal guidance and succeed on the first try (`../journeys/J-learn-terminal-from-docs.md`)
- `J-organize-tabbed-work` — adjacent window-manager canary (`../journeys/J-organize-tabbed-work.md`)

## Session Matrix & Results

| # | Charter | Journey / Scenario | Persona | Tour | Status | Issue | Fix commit |
|---|---|---|---|---|---|---|---|
| 1 | CH-terminal-shared-input-race | J-supervise-agent-terminal / ET-terminal-shared-control | Marina | Multi-Tab Tour | Pass | | |
| 2 | CH-terminal-shared-input-race | J-supervise-agent-terminal / ET-terminal-stream-resilience | Marina | Multi-Tab Tour | Pass | | |
| 3 | CH-terminal-shared-input-race | J-supervise-agent-terminal / ET-terminal-browser-lifecycle | Marina | Multi-Tab Tour | Pass | | |
| 4 | CH-terminal-shared-input-race | J-supervise-agent-terminal / ET-terminal-session-block-handoff | Marina | Multi-Tab Tour | Blocked (needs human verify) | Live provider was outside the targeted contract | |
| 5 | CH-terminal-v3-public-contract | J-operate-terminal-by-cli / ET-terminal-cli-public-contract | Ada | Feature Tour | Pass | | |
| 6 | CH-terminal-v3-public-contract | J-operate-terminal-by-cli / ET-terminal-hook-events | Ada | Feature Tour | Pass | | |
| 7 | CH-terminal-v3-public-contract | J-operate-terminal-by-cli / ET-terminal-approval-ladder-grants | Ada | Feature Tour | Blocked (needs human verify) | Live provider was outside the targeted contract | |
| 8 | CH-terminal-shared-control-docs | J-learn-terminal-from-docs / SITE-terminal-docs-truth | Lea | Feature Tour | Fixed | Stale generated wire page described v1 rejection and `RELEASE` | working tree |
| 9 | CH-terminal-shared-control-docs | J-learn-terminal-from-docs / ET-compozy-official-skill-discovery | Lea | Feature Tour | Pass | | |
| 10 | CH-terminal-window-tabs-canary | J-organize-tabbed-work / ET-window-tab-deck-lifecycle | Théo | Interrupt Tour | Pass | | |
| 11 | CH-terminal-window-tabs-canary | J-organize-tabbed-work / ET-window-tab-multi-instance | Théo | Interrupt Tour | Pass | | |
| 12 | CH-terminal-window-tabs-canary | J-organize-tabbed-work / ET-window-tab-close-reopen | Théo | Interrupt Tour | Pass | | |
| 13 | CH-terminal-window-tabs-canary | J-organize-tabbed-work / ET-web-window-routing-lifecycle | Théo | Interrupt Tour | Pass | | |
| 14 | CH-terminal-window-tabs-canary | J-organize-tabbed-work / ET-web-dock-default-window-size | Théo | Interrupt Tour | Pass | | |

Status legend: `Pending | Pass | Fixed | Skipped | Blocked (needs human verify) | Blocked (human decision)`

## Session Debriefs

- **Marina:** Two browser contexts and two interactive CLI clients attached to `term-9c3c42840d76`.
  Every participant could type immediately. CLI submissions sent three milliseconds apart remained whole,
  appeared once, and kept distinct journal actors. Detaching one CLI did not affect the other. A fresh
  browser reconnected, replayed output, and wrote without a handoff.
- **Ada:** HTTP, UDS, and CLI reads agreed on terminal state and journal output. SIGINT interrupted a
  running command with exit 130. The runtime exposed nine terminal tools, ten terminal hooks, wire v3,
  and no claim, yield, lease, controller, or takeover surface. Switching to profile `qa-other` hid the
  default profile's terminals.
- **Lea:** The rendered six-page terminal guide, Agents and Safety page, generated CLI reference, and
  bundled skill all taught shared input and the same nine-tool contract. The walk caught stale generated
  wire prose; the generator was fixed, protected with a canonical regression, and regenerated.
- **Théo:** Created two terminal windows, changed OS tabs, minimized and restored the window, reloaded
  with both instances present, and closed one window without ending the original terminal. The adjacent
  window-manager contract remained intact.

## What Was Fixed

- The generated terminal wire page still said only v1 was rejected and described a removed `RELEASE`
  opcode. `cmd/compozy-codegen/terminal_wire_render_docs.go` now states that all earlier versions are
  rejected and describes interactive versus passive attachments without ownership language. The
  canonical generator test rejects obsolete opcodes and lease prose.

## Paper Cuts

None recorded yet.

## Runtime Errors Observed

None recorded yet.

## Human Verifications Needed

- `ET-terminal-session-block-handoff`: repeat the complete transcript-block journey in a provider-backed
  lab. The targeted contract deliberately required zero providers; browser presence and untrusted-output
  invariants passed locally and component tests own the changed code.
- `ET-terminal-approval-ladder-grants`: repeat the complete prompt/allowlist/revoke journey with a live
  provider. Runtime discovery, settings inspection, and focused approval-grant tests confirmed there is
  no terminal-scoped typing grant.

## Decisions for a Human

None recorded yet.

## Learnings

- The five taxonomy dimensions are covered as follows: journey and functional behavior by all four
  charters; experience and multi-client edges by the Multi-Tab Tour; cross-cutting regression by the
  tabbed-window canary. Mobile and locale are deliberately skipped because this change targets the
  desktop terminal and changes neither layout breakpoints nor localization.
- Actor attribution belongs to the newline-terminated submission, not to a global current writer. That
  lets concurrent participants share one PTY while preserving an honest command journal.
- Generated explanatory pages need negative assertions for removed protocol concepts; successful
  codegen alone cannot detect stale prose embedded in a renderer.

## Final Status

- **Exit gate (full automated suite):** pending final `make gate`
- **Issues by user impact:** Blocks-Completion 0 · Data-Loss 0 · Trust-Damage 0 · Friction 0 · Cosmetic 0
- **Coverage:** 4/4 journeys walked · 11 Pass · 1 Fixed · 2 Blocked (needs human verify)
- **Verdict:** targeted shared-control behavior is ready for the local gate and PR. The two provider-only
  presentation journeys are explicitly `blocked-verify`; neither blocks the runtime, transport, CLI,
  browser, documentation, skill, or window-canary contracts exercised here.
