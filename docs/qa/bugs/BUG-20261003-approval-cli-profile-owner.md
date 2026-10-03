# BUG-20261003-approval-cli-profile-owner: A profile owner cannot inspect or cancel its pending approval

- **Status:** open
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

Pending investigation after the persona walk. No source or database was inspected during the walk.

## Verification

Pending. The owned profile and approval remain for diagnosis; the original CLI selection is restored.
