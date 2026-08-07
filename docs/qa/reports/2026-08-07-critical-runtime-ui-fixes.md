# QA Run Report — 2026-08-07 — critical-runtime-ui-fixes

- **Scope:** Codex native-tool binding on macOS CGO-disabled builds, visible session streaming, bundled extension trust, and Marketplace Installed defaults.
- **Cadence tier:** targeted
- **Build:** `a46b424e` + worktree changes · **Environment:** fresh isolated lab `compozy-critical-runtime-ui-fixes-20260807-225222-371495-lab`
- **Started:** 2026-08-07T19:50:57-03:00 · **Status:** pass

## Personas

| Persona | Base | Device / Network / Locale | Sessions |
|---|---|---|---|
| Ada | Power User | desktop / wifi-fast / en-US | CH-codex-native-tools-cgo0 |
| Théo | Power User | desktop / wifi-fast / en-US | CH-visible-session-window-streams |
| Vera | Power User | desktop / wifi-fast / en-US | CH-bundled-extension-trust |
| Bruno | Power User | desktop / wifi-fast / en-US | CH-marketplace-installed-default |

## Flows in Scope

- `J-validate-compozy-hard-cut` — managed sessions and hosted MCP expose the same native Compozy tools (`../journeys/J-validate-compozy-hard-cut.md`).
- `J-13` — every visible session remains live while hidden windows suspend and catch up (`../journeys/J-13-follow-a-live-run.md`).
- `J-extension-policy-admin` — bundled trust remains first-party and independent of side-load policy (`../journeys/J-extension-policy-admin.md`).
- `J-marketplace-acquisition` — Installed is the default and explicit Marketplace scope survives navigation (`../journeys/J-marketplace-acquisition.md`).

## Session Matrix & Results

| # | Charter | Journey / Scenario | Persona | Tour | Status | Issue | Fix commit |
|---|---|---|---|---|---|---|---|
| 1 | CH-codex-native-tools-cgo0 | J-validate-compozy-hard-cut / ET-compozy-native-tool-invocation | Ada | Feature Tour | Pass | | |
| 2 | CH-codex-native-tools-cgo0 | J-validate-compozy-hard-cut / ET-048 | Ada | Feature Tour | Pass | | |
| 3 | CH-visible-session-window-streams | J-13 / RT-visible-session-streaming | Théo | Multi-Tab Tour | Pass | | |
| 4 | CH-bundled-extension-trust | J-extension-policy-admin / ET-bundled-extension-trust | Vera | Feature Tour | Pass | | |
| 5 | CH-bundled-extension-trust | J-extension-policy-admin / ET-web-extension-detail | Vera | Feature Tour | Pass | | |
| 6 | CH-bundled-extension-trust | J-extension-policy-admin / ET-web-extension-kit-inventory | Vera | Feature Tour | Pass | | |
| 7 | CH-marketplace-installed-default | J-marketplace-acquisition / ET-web-marketplace-landing-browse | Bruno | Back-Button Tour | Pass | | |
| 8 | CH-marketplace-installed-default | J-marketplace-acquisition / ET-web-marketplace-kind-navigation | Bruno | Back-Button Tour | Pass | | |
| 9 | CH-marketplace-installed-default | J-marketplace-acquisition / ET-web-marketplace-installed-management | Bruno | Back-Button Tour | Pass | | |
| 10 | CH-marketplace-installed-default | J-marketplace-acquisition / ET-web-extensions-manage | Bruno | Back-Button Tour | Pass | | |
| 11 | CH-marketplace-installed-default | J-marketplace-acquisition / ET-web-mcp-status-matrix | Bruno | Back-Button Tour | Pass | | |

Status legend: `Pending | Pass | Fixed | Skipped | Blocked (needs human verify) | Blocked (human decision)`

## Session Debriefs

- Ada: a real Codex `gpt-5.6-sol` session resolved the hosted tool catalog, loaded the bundled Compozy skill, called `compozy__extensions_list`, and ended normally. Persisted evidence: `sess-6a65dc44a435a72c`.
- Théo: with two Codex session windows tiled side by side and keyboard focus held in the left window, the visible right transcript advanced from 89 to 200 to 359 live tokens while it still reported `Working…`.
- Vera: the installed `dev-cycle` detail and CLI provenance agreed on `bundled`, `official`, checksum verified, trust verified, enabled, active, healthy, and zero policy warnings.
- Bruno: Skills, MCPs, and Extensions opened in Installed scope with `tab` omitted; an explicit Marketplace selection persisted through kind navigation, detail, and Back.

## What Was Fixed

- macOS peer verification no longer depends on CGO, so Codex can bind the hosted native-tool projection in shipped CGO-disabled builds.
- Web live-data ownership follows actual window visibility, not focus; every visible tiled session consumes stream events.
- Bundled extensions reconcile to first-party provenance and bypass the unrelated unverified side-load policy.
- Marketplace kind routes treat Installed as the implicit default and use `tab=market` only for an explicit catalog choice.

## Paper Cuts

None recorded yet.

## Runtime Errors Observed

- The first iteration gate exposed a stale Dashboard visibility test; the shared visibility contract and canonical Dashboard suite were aligned before QA. The focused Web suite then passed 82 tests.
- The first long-stream probe attempted to read repository files from the intentionally empty QA workspace and produced expected file-not-found tool results. The streaming verdict uses the later tool-free live-token probe, not those failed reads.

## Human Verifications Needed

None known.

## Decisions for a Human

None.

## Learnings

- Focus is not visibility in a tiled desktop; live-data ownership must follow the window manager's visible-state model.

## Final Status

- **Exit gate (full automated suite):** `make gate-full` — pass
- **Issues by user impact:** Blocks-Completion 0 · Data-Loss 0 · Trust-Damage 0 · Friction 0 · Cosmetic 0
- **Coverage:** 4/4 journeys walked; 11/11 scenarios passed
- **Verdict:** PASS — targeted real-provider, browser, CLI, and automated verification passed.
