# BUG-20261003-approval-cli-profile-owner: A profile owner cannot inspect or cancel its pending approval

- **Status:** verified
- **Impact (user-side):** Blocks-Completion
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Dora
- **Journey Step:** J-operate-profiles, resolve an approval before retiring a profile
- **Scenarios:** ET-profile-approval-owner-resume; ET-profile-lifecycle-race-guards
- **Found:** 2026-10-03 · **Report:** docs/qa/reports/2026-10-02-untested.md

## Summary

Dora creates an empty release-drafts profile and invokes its destructive delete palette command
against the attached browser. The command returns approval_pending. Both profile plans name that
approval as a blocker, and archive/delete correctly refuse, but approvals show and cancel return
approval_not_found even with the owning --profile. The profile cannot be retired through that CLI.

## Reproduction

Use the continuing isolated lab through public CLI and browser. Create release-drafts, select it
in Studio, and invoke profile.delete with profile=release-drafts and the attached browser client.
For returned apr_c0cfa104-d59d-4647-8667-73a6838b598e, run approvals show and approvals cancel with
--profile release-drafts. Both fail. Repeat show with the root flag before the subcommand: same
failure. HTTP/UDS archive-plan and delete-plan still list the identical executable approval ID.

**Expected:** the explicit owning profile can inspect and cancel its approval; another profile
cannot. **Actual:** the owner receives approval_not_found.

## Evidence

Under docs/qa/evidence/2026-10-02-untested/: profile-race-pending-delete.json,
profile-race-approval-owner.json, profile-race-approval-owner-root-selector.json,
profile-race-cancel-owned.json, profile-race-archive-plan-pending.json,
profile-race-delete-plan-pending.json, profile-race-owned-final-plan.json, and profile-race-ended.json.
The empty profile keeps work_items=0 while both lifecycle mutations refuse profile_approvals_pending.
Recording: /Users/pedronauck/.config/browser-harness/agent-workspace/recordings/profile-race-approval.

## Fix

The show and cancel command constructors never installed the existing selected-profile wrapper,
so the transport received no owning profile. Reuse configureSingleProfileCommand for show and
configureProfileMutationCommand for cancel. The API's ownership checks remain unchanged.

Invariant, owning layer, canonical suite: CLI approval commands transport the selected owner from
flag, environment or remembered selection. TestCmdPaletteCommands in internal/cli/cmd_palette_test.go
owns this binding; six new cases use its existing client I/O boundary.

## Verification

All six cases fail before the production repair with an empty transported profile
(approval-owner-red.log). The complete CLI race suite then passes in 13.139s
(approval-owner-green.log), the convention checker passes, the binary builds, and every affected
make gate lane passes with zero lint issues (approval-owner-gate.log).

Fresh approval apr_d40ab7d4-32a6-4a32-ae64-68422cf66e53 is readable by the owner through the flag,
root flag, environment and remembered selection. After switching to studio, foreign show/cancel
both refuse approval_not_found. HTTP and UDS plans name the approval, and archive/delete refuse
profile_approvals_pending. Explicit owner cancellation changes its status to canceled, clears both
plans, and keeps work_items=0. Archive, unarchive and deletion of the owned empty profile succeed;
all pre-existing profiles remain. The older approval is correctly reported as timeout.

Receipts: approval-owner-replay-*.json; recording:
/Users/pedronauck/.config/browser-harness/agent-workspace/recordings/approval-owner-replay.
The recording is stopped. Full owner-resume and concurrency scenario legs remain Pending.
A separate visual observation retained a deleted profile until reload; it does not invalidate the
CLI/API replay and is tracked independently for live-stream diagnosis.
