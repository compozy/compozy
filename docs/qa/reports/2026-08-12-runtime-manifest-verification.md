# QA Run Report — 2026-08-12 — Runtime Manifest Verification

- **Scope:** Issue #362 release fix: signed runtime-manifest verification before macOS and Linux installer publication
- **Cadence tier:** targeted
- **Build:** `5786c79589d214907f92b3630d1997d03e8eebac` + working tree · **Environment:** isolated local QA lab; signed candidate packages required for production parity
- **Started:** 2026-08-13T00:43:14Z · **Status:** in-progress

## Personas

| Persona | Base | Device / Network / Locale | Sessions |
|---|---|---|---|
| Lea | clean install | macOS and Linux laptop / wifi-fast / en-US | CH-desktop-first-run-macos, CH-desktop-first-run-linux |

## Flows in Scope

- `J-desktop-first-run` — install CompozyOS and reach a working product without terminal setup (`../journeys/J-desktop-first-run.md`)

## Session Matrix & Results

| # | Charter | Journey / Scenario | Persona | Tour | Status | Issue | Fix commit |
|---|---|---|---|---|---|---|---|
| 1 | CH-desktop-first-run-macos | J-desktop-first-run / APP-install-first-run-provision | Lea | Feature Tour | Blocked (needs human verify) | No signed candidate DMG built from this change | working tree |
| 2 | CH-desktop-first-run-linux | J-desktop-first-run / APP-install-first-run-provision | Lea | Feature Tour | Blocked (needs human verify) | No signed candidate AppImage built from this change | working tree |

Status legend: `Pending | Pass | Fixed | Skipped | Blocked (needs human verify) | Blocked (human decision)`

## Session Debriefs

### CH-desktop-first-run-macos — Lea

- **Ran:** 2026-08-13T00:43Z → 2026-08-13T00:45Z (box respected: yes)
- **Findings:** The package precondition failed: the branch has no release run and local signing secrets are unavailable.
- **Bugs filed/updated:** GitHub issue #362; no new product symptom was observed.
- **Scenarios settled:** APP-install-first-run-provision → blocked-verify
- **Paper cuts:** Not evaluated because the signed DMG entry point was unavailable.
- **Surprises:** None.
- **Suggested next charter:** Re-run CH-desktop-first-run-macos against the next signed candidate.

### CH-desktop-first-run-linux — Lea

- **Ran:** 2026-08-13T00:43Z → 2026-08-13T00:45Z (box respected: yes)
- **Findings:** The package precondition failed: the branch has no release run and local signing secrets are unavailable.
- **Bugs filed/updated:** GitHub issue #362; no new product symptom was observed.
- **Scenarios settled:** APP-install-first-run-provision → blocked-verify
- **Paper cuts:** Not evaluated because the signed AppImage entry point was unavailable.
- **Surprises:** None.
- **Suggested next charter:** Re-run CH-desktop-first-run-linux against the next signed candidate.

## What Was Fixed

### GitHub issue #362: Runtime package cannot be verified or installed

- **Symptom:** A clean Linux AppImage launch cannot provision its runtime package.
- **Root cause:** The Go release producer signed compact JSON in struct-field order while the desktop verifier requires recursively sorted object keys.
- **Fix:** working tree; canonical producer output plus exact desktop-verifier enforcement before release upload.
- **Regression test:** `internal/desktoprelease/release_test.go`, `desktop/src-tauri/src/runtime/artifacts.rs`, and `internal/config/release_config_test.go` failed before their owning production changes and pass afterward.
- **Retested:** Exact producer and consumer contracts passed locally; signed candidate package walks are blocked.

## Paper Cuts

Not evaluated; package walks did not start.

## Runtime Errors Observed

No new runtime process was started. The published beta.13 remains the reported failing artifact and was not treated as evidence for the working-tree fix.

## Human Verifications Needed

- [ ] Download the next signed candidate DMG on a clean macOS machine, accept it through Gatekeeper, complete provisioning, relaunch, and confirm one daemon plus healthy `compozy status`. (row #1)
- [ ] Install the next signed candidate AppImage on a clean Linux machine, complete provisioning, relaunch, and confirm one daemon plus healthy `compozy status`; retain the tauri-driver transcript. (row #2)

## Decisions for a Human

None.

## Learnings

- A release signature proves byte integrity, not that the desktop accepts the signed bytes; publication must execute the exact consumer contract.
- The targeted evidence audit passed at `/Users/pedronauck/dev/qa-labs/compozy-issue-362-runtime-manifest-20260813-004336-922826-lab/qa-artifacts/qa/qa-audit-report.md`.

## Final Status

Pending.
