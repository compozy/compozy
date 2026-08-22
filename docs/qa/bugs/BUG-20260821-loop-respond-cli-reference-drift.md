# BUG-20260821-loop-respond-cli-reference-drift: Loop respond reference omits stdin input

- **Status:** verified
- **Impact (user-side):** Degrades-Operation
- **Severity:** Medium · **Priority:** P1
- **Persona Affected:** Ada, headless Loop operator
- **Journey Step:** J-operate-loop-run-headless, discover the safe response input path
- **Scenarios:** LP-run-read-agent-journey
- **Found:** 2026-08-21 · **Report:** docs/qa/reports/2026-08-21-loop-task-legibility.md
- **Origin:** task_07 release-grade QA

## Summary

The generated `compozy loop respond` reference omitted `--payload-stdin` even though the shipped
command exposes the flag. An operator following the public reference could not discover the safe
interactive path introduced for schema-bound human responses.

## Reproduction

- **Invariant:** Generated CLI reference pages exactly match the current Cobra command tree.
- **Owning layer:** CLI reference generation.
- **Canonical suite:** `make cli-docs-check`, already exercised by `make test-e2e-runtime`.

1. Run `make cli-docs-check` from the repository root.
2. Inspect the generated diff for `packages/site/content/docs/cli/loop/respond.mdx`.

**Expected:** The options list includes `--payload-stdin` with the command's current help text.
**Actual:** The checked-in page omits the flag and the canonical drift check fails.

## Evidence

- `/Users/pedronauck/dev/qa-labs/compozy-loop-task-legibility-task07-20260822-025427-168223-lab/qa-artifacts/qa/cli-docs-check-before.log`

## Fix

The canonical CLI documentation generator refreshed the checked-in reference. No new test was
warranted: the existing drift check owns this generated public contract.

## Verification

The canonical CLI documentation drift check and the full runtime E2E lane passed in the isolated
Task 07 envelope. The generated options now include `--payload-stdin`.
