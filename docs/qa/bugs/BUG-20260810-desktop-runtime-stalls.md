# BUG-20260810-desktop-runtime-stalls: The app never reaches the product after starting its runtime

- **Status:** verified
- **Impact (user-side):** Blocks-Completion
- **Severity:** Critical · **Priority:** P1
- **Persona Affected:** Dora
- **Journey Step:** J-desktop-attach-daily, step 2
- **Scenarios:** APP-start-installed-daemon; APP-attach-running-daemon; APP-quit-contract
- **Found:** 2026-08-10 · **Report:** `docs/qa/reports/2026-08-10-desktop-app-release.md`

## Summary

Dora opened CompozyOS with an installed runtime, but the shell could not establish a trusted compatible attachment and never reached the product.

## Reproduction

- **Charter:** CH-desktop-attach-quit-macos · **Tour:** Interrupt Tour
- **Environment:** macOS 26.5.1 arm64 / isolated home / wifi-fast / en-US

1. Install the current Compozy runtime with app-owned provenance in the isolated home.
2. Open CompozyOS while the runtime is stopped.
3. Read the desktop state and runtime process evidence.

**Expected:** The app starts the runtime, verifies its identity and version, and reaches `product`.
**Actual:** Successive contract mismatches rejected the real process: the obsolete launch verb, npm launcher PID lineage, operator-home metadata, and the current beta version floor.

## Evidence

- `/Users/pedronauck/dev/qa-labs/compozy-desktop-app-release-20260810-110811-513872-lab/qa-artifacts/qa/app-control-product.txt`
- `/Users/pedronauck/dev/qa-labs/compozy-desktop-app-release-20260810-110811-513872-lab/qa-artifacts/qa/quit-contract.txt`

## Fix

- **Root cause:** The desktop supervisor and identity checks had drifted from the current daemon command, launcher process tree, status payload meaning, and beta version contract.
- **Fix commit:** `b415f24b`, `b3aa3d27`, `bd610cfa`, `02b55a46`
- **Regression test:** `runtime::supervisor::tests::should_launch_the_current_foreground_daemon_child_contract`; `runtime::readiness::tests::should_accept_a_bound_daemon_descended_from_the_spawned_launcher`; `runtime::readiness::tests::should_reject_a_bound_daemon_outside_the_spawned_launcher_lineage`; `runtime::probe::tests::should_treat_user_home_dir_as_operator_metadata`; `runtime::probe::tests::should_apply_minimum_version_and_date_schema_handshake`.

## Verification

- **Retested:** 2026-08-10, same persona/journey · **Report:** `docs/qa/reports/2026-08-10-desktop-app-release.md`
- **Result:** The installed app reported `state: product`, `runtime.attached: true`, and `runtime.owned: true`; the detached daemon remained healthy after app exit.
