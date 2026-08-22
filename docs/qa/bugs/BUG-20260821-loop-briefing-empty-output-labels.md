# BUG-20260821-loop-briefing-empty-output-labels: Finished Loop shows blank produced outputs

- **Status:** fixed
- **Impact (user-side):** Trust-Damage
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Lea, Loop supervisor
- **Journey Step:** J-supervise-loop-steady-state, step 6
- **Scenarios:** LP-web-run-default-read-briefing
- **Found:** 2026-08-21 · **Report:** docs/qa/reports/2026-08-21-loop-task-legibility.md
- **Origin:** n/a

## Summary

After a Loop finished, Lea could see a malformed briefing such as
`Run finished: done. Produced: , .` when the produced outputs did not declare artifact names. The
briefing preserved the output count but presented empty labels, making the result look corrupted.

## Reproduction

- **Charter:** CH-loop-legibility-run-default-read · **Tour:** Feature Tour
- **Environment:** desktop · wifi-fast · en-US

1. Finish a Loop run that persists produced outputs without `artifact_name` values.
2. Open the terminal run's default briefing.
3. Read the produced-output portion of the headline.

**Expected:** Every produced output has a stable, non-empty label and every output remains listed.
**Actual:** Unnamed outputs render as empty comma entries in the terminal headline.

## Evidence

- QA reproduction: `Run finished: done. Produced: , .`
- Regression reproduction: `TestBriefingContract/Should_label_mixed_named_and_unnamed_terminal_outputs`
  and `TestBriefingContract/Should_number_all_unnamed_terminal_outputs_without_empty_headline_entries`

## Fix

- **Root cause:** The terminal briefing joined optional artifact names directly. The producer
  permits blank `artifact_name` values and can still carry `output_id` or `output_ref`, but the
  briefing projection did not derive a display label from those identifiers.
- **Fix commit:** this bug file's introducing commit
- **Regression test:** `internal/loop/briefing_test.go` — both cases failed before the production
  change and pass after it.

## Verification

- **Retested:** focused automated projection checks on 2026-08-21; persona re-walk remains owned by
  the active QA session.
- **Result:** Mixed outputs use artifact name, output id, content reference, then positional fallback;
  all-unnamed outputs render as `output 1, output 2` with no empty entries.
