# BUG-20260812-successful-update-recommends-retry: Successful update still recommends another update

- **Status:** verified
- **Impact (user-side):** Trust-Damage
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Dora
- **Journey Step:** J-evaluate-compozy-beta, step 4
- **Scenarios:** REL-beta-self-update
- **Found:** 2026-08-12 · **Report:** docs/qa/reports/2026-08-12-issue-359-auto-update.md

## Summary

After CompozyOS reported that the isolated binary had updated to beta.13, the same structured result
still told Dora to run `compozy update` again.

## Reproduction

- **Charter:** CH-beta-self-update-artifact-contract · **Tour:** Feature Tour
- **Environment:** desktop / wifi-fast / en-US, isolated direct binary using the real beta.13 release

1. Build the fixed candidate as beta.8 into the isolated QA lab.
2. Run `compozy update -o json` from that candidate path.
3. Read the successful structured result.

**Expected:** The result reports beta.13 as installed and contains no further update recommendation.
**Actual:** The result reports `status: updated` and also recommends `Run compozy update`.

## Evidence

- `/Users/pedronauck/dev/qa-labs/compozy-issue-359-auto-update-20260812-211235-947224-lab/qa-artifacts/qa/update-apply.json`
- `/Users/pedronauck/dev/qa-labs/compozy-issue-359-auto-update-20260812-211235-947224-lab/qa-artifacts/qa/candidate-after-version.txt`

## Fix

- **Root cause:** The successful update paths reused the available-state record but changed only status, version, and message, leaving its pre-update recommendation intact.
- **Fix commit:** working-tree
- **Regression test:** `internal/cli/update_command_test.go` — the canonical successful local-update flow now starts with the available recommendation and requires it to be empty after completion; it failed before the fix and passed after it.

## Verification

- **Retested:** 2026-08-12, same persona/journey · **Report:** docs/qa/reports/2026-08-12-issue-359-auto-update.md
- **Result:** A fresh isolated beta.8 candidate updated through the real beta.13 release; its successful JSON omitted `recommendation`, and the replaced binary reported beta.13.
