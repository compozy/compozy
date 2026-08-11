# QA Run Report — 2026-08-11 — desktop startup diagnostics

- **Scope:** Desktop self-start reliability, ownership-safe recovery, visible startup errors, and offline diagnostic export
- **Cadence tier:** targeted
- **Build:** working tree · **Environment:** isolated macOS arm64 lab; signed DMG and Linux AppImage candidate unavailable locally
- **Started:** 2026-08-11T20:03:36-03:00 · **Status:** blocked — signed package verification required

## Personas

| Persona | Base | Device / Network / Locale | Sessions |
|---|---|---|---|
| Lea | clean desktop home | macOS arm64 / wifi-fast / en-US | CH-desktop-first-run-macos |
| Dora | isolated installed runtime | macOS arm64 / wifi-fast / en-US | J-desktop-attach-daily |
| Ada | isolated CLI/runtime home | terminal / wifi-fast / en-US | CH-desktop-agent-headless-cli |

## Flows in Scope

- `J-desktop-first-run` — install and reach the product without terminal setup (`../journeys/J-desktop-first-run.md`)
- `J-desktop-attach-daily` — start or attach to exactly one runtime (`../journeys/J-desktop-attach-daily.md`)
- `J-desktop-agent-headless` — diagnose the desktop through structured CLI (`../journeys/J-desktop-agent-headless.md`)

## Session Matrix & Results

| # | Charter | Journey / Scenario | Persona | Tour | Status | Issue | Fix commit |
|---|---|---|---|---|---|---|---|
| 1 | CH-desktop-first-run-macos | J-desktop-first-run / APP-install-first-run-provision | Lea | Feature Tour | Blocked (needs human verify) | Signed DMG/AppImage candidate unavailable locally | working tree |
| 2 | local macOS runtime walk | J-desktop-attach-daily / APP-start-installed-daemon | Dora | Feature Tour | Blocked (needs human verify) | Local runtime path passed; packaged progress UI and Linux artifact remain | working tree |
| 3 | local macOS runtime walk | J-desktop-attach-daily / APP-attach-running-daemon | Dora | Feature Tour | Blocked (needs human verify) | Local one-PID attach passed; browser parity and shipping OS matrix remain | working tree |
| 4 | CH-desktop-agent-headless-cli | J-desktop-agent-headless / APP-agent-cli-app-verbs | Ada | Feature Tour | Blocked (needs human verify) | Diagnose/export paths passed; full packaged update matrix remains | working tree |

Status legend: `Pending | Pass | Fixed | Skipped | Blocked (needs human verify) | Blocked (human decision)`

## Session Debriefs

- Opening the working-tree desktop with the isolated runtime stopped started daemon PID `89043`, reached `state:"product"`, and left exactly one listener on port `63771`.
- Quitting the desktop left PID `89043` healthy. Reopening attached to the same PID and did not create a second listener.
- Live diagnostics returned the shared schema-version-1 report. With the app absent, fallback occurred only when `app.sock` was absent; an intentionally stale socket returned `app_control_unavailable` as designed.
- Bundle export required `--yes`, created a mode-`0600` archive, rejected overwrite without changing the destination, and contained only allowlisted redacted entries. The quiet-boot archives contained only `manifest.json`.
- An abrupt prior desktop exit appeared as `previous_crash` without exposing the home path. The reported `stream shutdown bridge not provided` warning did not appear.

Behavioral evidence: `/Users/pedronauck/dev/qa-labs/compozy-desktop-startup-diagnostics-20260811-200336-439901-lab/qa-artifacts/qa/runtime-cli-walk.md`

## What Was Fixed

- The daemon now records its real OS start time and publishes `daemon.json` only after fallible startup work and transports are ready.
- Desktop recovery waits for a live recorded process and retries only after proving bounded timestamp drift, process identity, executable hash, and desktop provenance.
- Expected startup/control-window failures remain visible instead of aborting the Tauri event loop.
- Desktop and CLI now share a redacted diagnostic report, consent-gated local export, crash correlation, and offline fallback.
- Release configuration is mandatory for release builds, and signed artifact smoke runs before publish.
- QA exposed stale pre-hard-cut CLI fixtures; the canonical status suite now requires `diagnostic_report` on every schema-version-1 app record.

## Paper Cuts

The direct debug executable could not be addressed by the macOS UI automation driver as an application bundle. This is a tooling boundary, not a product verdict.

## Runtime Errors Observed

An intentionally retained stale `app.sock` produced the expected `app_control_unavailable` error. No unexpected runtime error was observed.

## Human Verifications Needed

- Run the mandatory `desktop-smoke` job against the signed macOS Apple Silicon DMG and Linux AppImage produced by `desktop-build` before `desktop-publish`.
- On the packaged boot window, observe truthful download/verify/install/start progress, retry after a controlled startup failure, copy diagnostics, and explicit local export.
- Capture process evidence proving one daemon, correct app/runtime/provenance versions, healthy status after app quit, and attach-without-spawn.
- Complete the full packaged `compozy app status|open|update|diagnose` JSON matrix and browser/session parity walk on the shipping OS matrix.

## Decisions for a Human

None. The remaining work is verification against release artifacts, not a product decision.

## Learnings

- The user-reported warning was construction-time noise and not the crash cause.
- The failing behavior came from daemon identity timing, startup publication order, and incomplete release provisioning guarantees.
- A stale control socket must remain a named failure; silently falling back could hide a live but unresponsive desktop.

## Final Status

Blocked pending signed-package verification. The isolated macOS runtime and CLI walk passed the exercised legs, and release CI now blocks publication until DMG/AppImage smoke succeeds. The QA teardown is clean with no survivors: `/Users/pedronauck/dev/qa-labs/compozy-desktop-startup-diagnostics-20260811-200336-439901-lab/qa-artifacts/qa/teardown.json`. The repository full gate is recorded at workstream close.
