# BUG-20260730-loop-automation-target-cli: CLI could not author Loop automation targets

- **Status:** verified
- **Impact (user-side):** Blocks-Completion
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Ada
- **Journey Step:** J-09 automate a Loop, step 2
- **Scenarios:** TA-063; TA-064
- **Found:** 2026-07-30 · **Report:** docs/qa/reports/2026-07-28-untested-full.md

## Summary

Ada could inspect Loop-target jobs and triggers but could not create them from the CLI, leaving automation authoring incomplete for agents.

## Reproduction

- **Charter:** CH-loop-automation-authorship · **Tour:** Feature Tour
- **Environment:** isolated current-source daemon / CLI / en-US

1. Create a scheduled job targeting `software-delivery` with a static `slug` input.
2. Create an event trigger targeting `review-and-fix` with a static `task_name` input.

**Expected:** Both records persist a typed Loop target and empty prompt.
**Actual:** The pre-fix CLI exposed only agent target flags.

## Evidence

- `/Users/pedronauck/dev/qa-labs/compozy-root-fixes-retest-20260730-072350-328928-lab/qa-artifacts/qa/evidence/root-fixes/public-replay.md`

## Fix

- **Root cause:** Job and trigger create commands had no shared Loop-target flag/request builder.
- **Fix commit:** e45affe
- **Regression test:** `internal/cli/automation_test.go`

## Verification

- **Retested:** 2026-07-30, same persona/journey · **Report:** docs/qa/reports/2026-07-28-untested-full.md
- **Result:** Public CLI creation persisted one scheduled job and one event trigger with typed Loop targets and inputs.
