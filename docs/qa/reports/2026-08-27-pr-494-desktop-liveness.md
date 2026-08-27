# QA Run Report — 2026-08-27 — PR 494 desktop liveness

- **Scope:** Verify authenticated desktop liveness for `compozy app status` and `compozy app open` after removing PID start-time matching.
- **Cadence tier:** targeted
- **Build:** working tree based on `eca54e8a5` · **Environment:** isolated Linux CLI lab; packaged Electron headless; browser unavailable
- **Started:** 2026-08-27T18:43:18-03:00 · **Status:** closed

## Personas

| Persona | Base | Device / Network / Locale | Sessions |
|---|---|---|---|
| Ada | Agent operator | desktop / offline / en-US | CH-desktop-agent-headless-cli |

## Flows in Scope

- `J-desktop-agent-headless` — inspect, navigate, and stop a live desktop instance through structured CLI output (`../journeys/J-desktop-agent-headless.md`)

## Session Matrix & Results

| # | Charter | Journey / Scenario | Persona | Tour | Status | Issue | Fix commit |
|---|---|---|---|---|---|---|---|
| 1 | CH-desktop-agent-headless-cli | J-desktop-agent-headless / APP-agent-cli-app-verbs | Ada | Feature Tour | Blocked (needs human verify) | Local runner has no `DISPLAY`, Wayland session, or `xvfb-run`; packaged Electron exited with `SIGSEGV` before opening `app.sock`. | working tree |

Status legend: `Pending | Pass | Fixed | Skipped | Blocked (needs human verify) | Blocked (human decision)`

## Session Debriefs

### CH-desktop-agent-headless-cli — Ada

- **Ran:** 2026-08-27T18:43:18-03:00 → 2026-08-27T18:46:00-03:00 (box respected: yes)
- **Findings:**
  - `compozy app status -o json` returned schema-v2 `installed:false, running:false` before launch.
  - The packaged desktop could not enter the graphical lifecycle because this Linux runner has no display server. A single headless launch attempt exited with `SIGSEGV` before `app.sock` existed.
- **Bugs filed/updated:** none; the missing graphical prerequisite is local environment state, not a product finding.
- **Scenarios settled:** APP-agent-cli-app-verbs → blocked-verify
- **Paper cuts:** none
- **Surprises:** the canonical local E2E target packages and may download Electron before checking for `xvfb-run`.
- **Suggested next charter:** rerun CH-desktop-agent-headless-cli on the PR Linux runner, which provides Xvfb and Openbox.

## What Was Fixed

### Desktop liveness false negative

- **Symptom:** `compozy app status` could return `running:false` while the authenticated desktop instance was already serving the product.
- **Root cause:** liveness compared an Electron wall-clock timestamp with `ps` process start time instead of probing the authenticated control channel.
- **Fix:** working tree; `app status` and `app open` now share an authenticated `diagnose` probe.
- **Regression test:** `internal/cli/app_test.go` — failed before and passes after under `go test -race ./internal/cli/...`.
- **Retested:** unit and scoped repository gates passed; packaged graphical replay awaits the PR runner.

## Paper Cuts

None.

## Runtime Errors Observed

- Packaged Electron exited with `SIGSEGV` before opening a window or `app.sock` because no graphical display was available locally. No product process survived the attempt.

## Human Verifications Needed

- [ ] In the PR's `E2E desktop` check, confirm `E2E-001 E2E-002: first run provisions offline and exposes every boot phase` passes with the direct, non-polling `app status` assertion. (row #1)

## Decisions for a Human

None recorded yet.

## Learnings

- Local `make test-e2e-desktop` requires Xvfb and Openbox on Linux; this machine has neither, so the packaged graphical journey cannot be claimed locally.
- The canonical target checks that prerequisite only after the Web and Electron packaging steps.

## Final Status

- **Exit gate (full automated suite):** `make gate` — PASS: all affected local lanes passed; exact-head PR CI owns full verification. Evidence: `/home/francisross/dev/qa-labs/compozy-pr-494-desktop-liveness-20260827-214305-641789-lab/qa-artifacts/qa/final-make-verify.log`.
- **Issues by user impact:** Blocks-Completion 0 · Data-Loss 0 · Trust-Damage 0 · Friction 0 · Cosmetic 0
- **Coverage:** 0/1 journeys completed; the CLI preflight was observed, but the packaged graphical liveness leg was blocked by the missing local display server.
- **Verdict:** BLOCKED — the fix is locally gate-clean, but the packaged Desktop assertion must pass in the PR's Xvfb-backed `E2E desktop` check before merge readiness can be claimed.
