# BUG-20260729-loop-resume-restart-stuck: Native Resume after restart left Loop running without a coordinator

- **Status:** fixed
- **Impact (user-side):** Blocks-Completion
- **Severity:** Critical · **Priority:** P0
- **Persona Affected:** Ada
- **Journey Step:** J-07 native Loop control after a daemon restart
- **Scenarios:** TA-076
- **Found:** 2026-07-29 · **Report:** docs/qa/reports/2026-07-28-untested-full.md

## Summary

A Loop paused at a generation boundary could be resumed successfully through the native tool after
the daemon restarted, but no durable coordinator was reserved. The run changed to `running` and then
remained stuck at generation 1 indefinitely, preventing the agent from finishing the workflow.

## Reproduction

- **Charter:** CH-untested-008-07-ada · **Tour:** Feature Tour
- **Environment:** isolated macOS lab / native tools + HTTP + UDS / real ACP mock subprocess / en-US

1. Start a Loop through `compozy__loop_run` and request Pause before generation 1 completes.
2. Wait for the run to settle at `paused`, generation 1, then stop and restart the daemon while
   preserving the same isolated `COMPOZY_HOME`.
3. Invoke `compozy__loop_resume` and poll the public run status.

**Expected:** Resume atomically makes the run executable, reserves one coordinator wake, and the run
progresses to generation 2 and its terminal verdict.

**Actual before the fix:** Resume returned `ok`, but the run remained `running` at generation 1 with
all generation-1 outputs already succeeded and no coordinator available to finish the boundary.

## Evidence

- Stuck public status:
  `/Users/pedronauck/dev/qa-labs/compozy-loop-native-controls-20260729-20260729-205221-682163-lab/qa-artifacts/qa/evidence/053-loop-native-controls/ta076-resume-stuck-cli.json`
- Repaired restart replay: `ta076-fix-paused-before-restart-cli.json`,
  `ta076-fix-paused-restart-stop.json`, `ta076-fix-paused-restart-start.json`,
  `ta076-fix-resume-after-restart-native.json`, `ta076-fix-final-native-status.json`, and
  `ta076-fix-replay-assertions.json` in the same evidence directory.

## Fix

- **Root cause:** non-Goal Resume and Approve changed only the Loop control state. The first repair
  incorrectly pre-reserved the deterministic next-generation coordinator; when that coordinator
  correctly acted as the previous generation's finisher, it attempted to materialize the same next
  generation again and hit the task-run uniqueness constraint.
- **Correction:** the control transaction now validates the expected state, writes approval
  decisions when applicable, performs the status CAS, and reserves one generic finisher wake without
  claiming a generation identity. After commit, the daemon wakes the existing task backstop; the
  finisher remains the sole owner of next-generation snapshot and task creation.
- **Fix commit:** `103192e4`
- **Regression test:** `internal/daemon/loop_run_events_e2e_integration_test.go` owns the real
  pause → daemon restart → HTTP Resume → generation-2 terminal progression invariant.

## Verification

- The canonical daemon E2E reproduced the stuck restart path and then passed twice after the generic
  finisher-wake correction.
- Focused Loop service, native Loop tool, daemon E2E, and tools suites pass under `-race`; scoped
  golangci-lint reports zero issues, and the rebuilt candidate passes `make build`.
- The rebuilt public replay preserved the paused generation-1 run across PID 67382 → 74307, native
  Resume returned `ok`, and status reached `done`, generation 2. HTTP and UDS detail bodies had the
  same SHA-256 digest; the daemon log contained no uniqueness or wake failure.
- **Retested:** 2026-07-29 in the original isolated stateful-control lab.
- **Result:** Pass. Governed fix commit `103192e4`; public pause → restart → native Resume reached
  done generation 2.
