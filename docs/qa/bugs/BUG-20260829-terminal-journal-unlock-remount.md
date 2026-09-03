# BUG-20260829-terminal-journal-unlock-remount: Profile switch can strand the Terminal journal in loading

- **Status:** verified
- **Impact (user-side):** Blocks-Completion
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Ada
- **Journey Step:** J-switch-profile-terminal-scope, widen the Terminal journal to all profiles
- **Scenarios:** ET-terminal-profile-segmentation
- **Found:** 2026-08-29 · **Report:** docs/qa/reports/2026-08-28-integrated-terminal-rebase.md

## Summary

After the operator opened a profile journal and switched to the all-profiles view, the Terminal window
could remain on `Loading the journal` without sending the aggregate journal request. The failure was
intermittent because it depended on the OS shell rematerializing the Terminal window during the switch.

## Reproduction

- **Charter:** CH-terminal-profile-fence · **Tour:** Garbage Tour
- **Environment:** serial Playwright launch-mode runtime / Chromium / en-US

1. Open terminal work and a pending input request under the default profile.
2. Create a second profile, open a terminal there, and write a distinct journal row.
3. Return to the default profile, then switch to All profiles.
4. Open the Terminal journal.

**Expected:** The browser requests `terminals/journal?all_profiles=true` and renders rows from both
profiles with owner labels.

**Actual:** The journal spinner could remain indefinitely, with no aggregate journal request in the
browser network log.

## Evidence

- Focused reproduction: E2E-020 failed 1/3, then 1/5 and 2/5 on the same head before the fix.
- Failure artifacts: `.tmp/playwright/test-results/__tests__-terminal-agent-E-11840-nd-aggregate-journal-owners-repeat1/`.
- The browser network log contained the profile journal request but no `all_profiles=true` journal
  request during the failed pass.

## Fix in the working tree

- **Root cause:** The first-open journal gate lived in render-local state and was reset on profile scope
  changes. When the OS authority rematerialized the Terminal window, the new controller lost the gate
  even though the operator had already opened the journal.
- **Production fix:** Keep first-open state in an app-wide XState store keyed by workspace. TanStack
  Query continues to own journal data with separate workspace and profile keys.
- **E2E fix:** Follow the authority-backed Terminal locator instead of freezing an opaque window ID, and
  click the same visible terminal log that the control helper validated.
- **Regression suites:**
  `web/src/systems/os/apps/terminal/__tests__/use-terminal-window-controller-state.test.ts` and
  `web/e2e/__tests__/terminal-agent.spec.ts` (`E2E-020`).

## Verification

- **Focused retest:** 10/10 serial E2E-020 passes after the journal-state fix and terminal-log locator
  correction.
- **Unit evidence:** 6/6 controller host-gate tests pass.
- **Isolated real-user evidence:** The Web walk switched default to profile B, widened to All profiles,
  rendered both owner-labelled journal rows without loading residue, and switched back to the isolated
  default view. The matching CLI, HTTP, and UDS reads passed in the same lab.
- **Evidence:**
  `/Users/pedronauck/dev/qa-labs/compozy-integrated-terminal-profile-retest-20260829-172042-776889-lab/qa-artifacts/qa/profile-segmentation-walk.md`.
