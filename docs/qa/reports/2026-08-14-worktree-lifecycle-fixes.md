# QA Run Report — 2026-08-14 — Worktree lifecycle fixes

- **Scope:** Web creation completion, canonical name/ID lifecycle references, and dismissed-name reuse with tombstone history preservation.
- **Cadence tier:** targeted
- **Build:** `6fb2afe0` plus working-tree changes · **Environment:** fresh isolated lab `compozy-worktree-lifecycle-fixes-20260815-004729-655016-lab`, daemon `http://127.0.0.1:51834`, production-like local Web proxy; no providers required.
- **Started:** 2026-08-14T21:47:48-03:00 · **Completed:** 2026-08-14T22:22:39-03:00 · **Status:** QA complete

## Personas

| Persona | Base | Device / Network / Locale | Sessions |
|---|---|---|---|
| Ada | operator | desktop / wifi-fast / en-US | CH-worktree-lifecycle-surface-parity |
| Lea | operator | laptop / wifi-fast / en-US | CH-add-workspace-from-root |

## Flows in Scope

- `J-worktree-management` — create, select, inspect, remove, dismiss, and reuse a worktree name while every public surface preserves one identity (`../journeys/J-worktree-management.md`).
- `J-add-workspace-by-browsing` — adjacent canary proving the workspace picker still reaches the isolated project (`../journeys/J-add-workspace-by-browsing.md`).

## Session Matrix & Results

| # | Charter | Journey / Scenario | Persona | Tour | Status | Issue | Fix commit |
|---|---|---|---|---|---|---|---|
| 1 | CH-worktree-lifecycle-surface-parity | J-worktree-management / RT-worktree-web-create-adopt | Ada | Feature Tour | Pass | | |
| 2 | CH-worktree-lifecycle-surface-parity | J-worktree-management / RT-worktree-cli-lifecycle | Ada | Feature Tour | Fixed | BUG-20260814-worktree-mutation-output-loses-identity | working tree |
| 3 | CH-worktree-lifecycle-surface-parity | J-worktree-management / RT-worktree-api-surface-parity | Ada | Feature Tour | Pass | | |
| 4 | CH-add-workspace-from-root | J-add-workspace-by-browsing / RT-038 | Lea | Feature Tour | Pass | | |

Status legend: `Pending | Pass | Fixed | Skipped | Blocked (needs human verify) | Blocked (human decision)`

## Session Debriefs

### CH-worktree-lifecycle-surface-parity — Ada (first session)

- **Ran:** 2026-08-14T21:50:54-03:00 → 2026-08-14T21:58:10-03:00 (box respected: yes)
- **Findings:** Web creation completed and persisted. CLI name-addressed remove/dismiss transitioned successfully, but returned untruthful structured identities; same-name recreation was blocked by the retained created branch.
- **Bugs filed/updated:** BUG-20260814-worktree-name-reuse-blocked; BUG-20260814-worktree-mutation-output-loses-identity
- **Scenarios settled:** RT-worktree-web-create-adopt → pass; RT-worktree-cli-lifecycle → fail
- **Paper cuts:** none
- **Surprises:** storage name release alone was insufficient because the default branch derived from the name remained reserved.
- **Suggested next charter:** fresh Ada replay after the governed fix, followed by HTTP/UDS parity.

### CH-worktree-lifecycle-surface-parity — Ada (fix replay)

- **Ran:** 2026-08-14T22:08:00-03:00 → 2026-08-14T22:22:39-03:00 (box respected: yes)
- **Findings:** Rebuilt CLI receipts preserved the canonical ID through status, removal, and dismissal. Direct HTTP calls showed the same identity, hid dismissed rows from name lookup, preserved old rows by ID, and created distinct ready rows with the freed catalog names.
- **Bugs filed/updated:** BUG-20260814-worktree-mutation-output-loses-identity → verified; BUG-20260814-worktree-name-reuse-blocked → invalid because branch preservation is the documented contract.
- **Scenarios settled:** RT-worktree-cli-lifecycle → pass after fix; RT-worktree-api-surface-parity → pass.
- **Paper cuts:** none.
- **Surprises:** A dismissed catalog name and a retained Git branch are separate resources. Reusing both requires `--existing-branch`; silently deleting the branch would destroy operator history.
- **Suggested next charter:** none for this lifecycle fix.

### CH-add-workspace-from-root — Lea

- **Ran:** 2026-08-14T21:51:35-03:00 → 2026-08-14T21:55:55-03:00 (box respected: yes)
- **Findings:** The Locations-backed picker reached the isolated project, registered it only on submit, and retained the selected workspace after refresh.
- **Bugs filed/updated:** none
- **Scenarios settled:** RT-038 → pass
- **Paper cuts:** the large QA-lab directory is naturally dense, but navigation remained usable; dull.
- **Surprises:** none
- **Suggested next charter:** none for this adjacent canary.

## What Was Fixed

- Name-addressed CLI mutations now resolve the canonical row before mutation and return truthful structured receipts instead of fabricating an empty row from the entered name.
- Status responses and the native removal tool now preserve the same canonical ID across their public boundaries.
- The initial retained-branch finding was closed as invalid; the corrected lifecycle reuses the freed catalog name while explicitly adopting the intentionally retained branch.

## Paper Cuts

None observed yet.

## Runtime Errors Observed

No runtime errors remained after the rebuilt-daemon replay. The expected `404` for name lookup after dismissal and the initial `branch_held_by_worktree` refusal were deterministic contract outcomes, not daemon failures.

## Human Verifications Needed

None identified.

## Decisions for a Human

None identified.

## Learnings

- Catalog identity and Git branch identity have different lifecycles: dismissal releases the former, while removal preserves the latter.
- Mutation receipts must come from the resolved canonical row; reconstructing them from an operator-entered reference corrupts structured output even when the state transition succeeds.

## Final Status

- **Exit gate (full automated suite):** scheduled as the final immutable step after this QA report and teardown are frozen.
- **Issues by user impact:** 1 Trust-Damage finding fixed and verified; 1 Blocks-Completion finding invalidated after contract review; 0 open findings.
- **Coverage:** Web creation and refresh; CLI/UDS name-addressed status/remove/dismiss/recreate; direct HTTP lifecycle and tombstone checks; runtime checkout inventory; adjacent workspace-picker canary.
- **Verdict:** **PASS** for behavioral QA; ready for the final full gate with no QA blocker remaining.
