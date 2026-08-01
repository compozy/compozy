# BUG-20260801-loop-config-file-snake-case: Loop config files reject documented snake_case fields

- **Status:** verified
- **Impact (user-side):** Blocked
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Bruno
- **Journey Step:** Configure or start a Loop with reusable file-based overrides
- **Scenarios:** LP-loop-config-file-snake-case
- **Found:** 2026-08-01 · **Report:** docs/qa/reports/2026-08-01-loops-paper-adoption.md
- **Origin:** Task 07 real-scenario QA

## Summary

compozy loop run --config-file and compozy loop configure --file rejected the documented
iteration_cap, no_progress_window, and gate_max_revisions fields in both JSON and YAML. This made
reusable file-based Loop overrides unusable even though the equivalent --set flags and HTTP JSON
contract worked.

## Reproduction

1. Create a JSON or YAML config file with iteration_cap, no_progress_window, and
   gate_max_revisions.
2. Run compozy loop run --config-file PATH or compozy loop configure --file PATH.
3. Observe yaml unmarshal errors reporting iteration_cap as unknown in loop.LoopConfig.

**Expected:** Both file formats accept the public snake_case fields and preserve strict
unknown-field rejection.

**Actual:** Every documented field is reported as unknown before the daemon is called.

## Evidence

- Red regression: CGO_ENABLED=1 go test -race ./internal/cli with the
  TestLoopCommandShouldMapCLIVerbsToClient/Should_load_snake_case_loop_config filter.
- Isolated QA workspace:
  /Users/pedronauck/dev/qa-labs/compozy-northstar-pay-20260801-135009-390014-lab/project/loop-qa

## Fix

- **Root cause:** The strict yaml.v3 decoder uses YAML field metadata, while loop.LoopConfig
  exposed only JSON tags. It therefore treated the public snake_case names as unknown even for
  JSON, which YAML accepts as input syntax.
- **Fix:** Add matching YAML tags to every public LoopConfig field so JSON/YAML file decoding and
  the HTTP JSON contract share one field vocabulary.
- **Fix commit:** pending Task 07 commit
- **Regression test:** The canonical internal/cli/loop_test.go command suite covers JSON and YAML
  through both CLI verbs and retains the existing unknown-field invariant.

## Verification

The rebuilt CLI accepted both formats in the isolated QA lab. JSON started
looprun-04fb1c8afcbf8c55 with iteration_cap 3 and reached the expected exhausted state; YAML
configured iteration_cap 3, no_progress_window 10, and gate_max_revisions 10. The canonical
race-enabled CLI suite passed for both formats and retained strict unknown-field rejection.

- /Users/pedronauck/dev/qa-labs/compozy-northstar-pay-20260801-135009-390014-lab/qa-artifacts/qa/loop/config-file-json-run-status-v6.json
- /Users/pedronauck/dev/qa-labs/compozy-northstar-pay-20260801-135009-390014-lab/qa-artifacts/qa/loop/config-file-yaml-configure-v6.json
