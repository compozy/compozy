# BUG-20260713-loop-automation-shown-as-agent: Loop-target automations are displayed as empty agent automations

- **Status:** verified
- **Impact (user-side):** Trust-Damage
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Bruno
- **Journey Step:** J-24 manage automations, create/read/run Loop target
- **Scenarios:** TA-automation-crud-loop-target
- **Found:** 2026-07-13 · **Report:** docs/qa/reports/2026-07-13-automation-features.md
- **Origin:** n/a

## Summary

Bruno created a Job from `software-delivery`'s Add schedule action, with `Run loop` selected and typed Loop inputs. The runtime correctly stored and dispatched the Loop target: `Run now` produced automation run `run-f4489762ac431856` and Loop run `looprun-aeb24d4f17cf1feb`. Yet both Job and Trigger create/edit previews said `run agent`, their displayed request payloads omitted `target_kind`, `loop_target`, and typed inputs, and the persisted Job detail showed `Agent:` and Prompt as blank. Operators cannot audit which Loop or inputs an automation will run even though execution is correct.

## Reproduction

- **Charter:** CH-automation-crud-loop-target · **Tour:** Garbage Tour
- **Environment:** desktop / wifi-fast / en-US; isolated daemon at `http://127.0.0.1:58941`; in-app browser.

1. Open `software-delivery` and choose Add schedule.
2. In Create job, keep `Run loop` selected, enter name `software-delivery-daily-qa` and slug `helix-v1-launch`.
3. Observe the Live preview and displayed request payload, then create the Job.
4. Observe the Job detail and reopen Edit to independently confirm that the Loop target and slug really persisted.
5. Choose Run now and inspect the Loop Runs catalog.
6. Open Create trigger, select `Run loop` and `software-delivery`, and inspect its Live preview.

**Expected:** Every create/edit/detail/read surface labels the target as Loop, names `software-delivery`, shows the typed input mapping, and correlates delegated automation runs with their `loop_run_id`.
**Actual:** Preview/detail surfaces describe an agent with empty agent/prompt and omit Loop target fields, while the runtime still delegates to `looprun-aeb24d4f17cf1feb`.

## Evidence

- `/Users/pedronauck/dev/qa-labs/agh-automation-features-20260713-20260713-044543-173594-lab/qa-artifacts/qa/screenshots/ch-009-loop-job-missing-target.png`
- `/Users/pedronauck/dev/qa-labs/agh-automation-features-20260713-20260713-044543-173594-lab/qa-artifacts/qa/screenshots/ch-automation-trigger-loop-start-kind-error.png` (also shows the Trigger preview saying `run an agent` while Run loop is selected).
- Daemon correlation: `automation.dispatch.loop_delegated`, job `job-6a0a00830d60c1c0`, automation run `run-f4489762ac431856`, Loop run `looprun-aeb24d4f17cf1feb`.
- Independent Web read: Loops Runs showed `looprun-aeb24d4f17cf1feb · automation`, 2/3 generations, after Run now.

## Fix

- **Root cause:** The shared automation form preview, request projection, detail read model, and edit hydration assumed every target was an agent even when the runtime contract carried a typed Loop target.
- **Fix commit:** pending final whole-diff commit.
- **Regression test:** Canonical shared automation form/detail suites assert target-aware Loop labels, `target_kind`, `loop_target`, typed inputs, edit round-trip, and correlated delegated history for both Jobs and Triggers.

## Verification

- Same-persona browser replay passed create, edit, detail read-back, Run now, and correlated Loop history for `software-delivery-qa`, including its typed input. The later Trigger replay independently preserved workspace scope, `reviews-watch`, typed `pr=2`, and correlated exactly one delegated Loop run.
