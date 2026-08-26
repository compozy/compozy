# QA Run Report — 2026-08-26 — Profile identity and ownership

- **Scope:** `fix-adjustments` profile identity, selection restoration, scoped and aggregate ownership, session mutation, automation, run, and usage surfaces
- **Cadence tier:** targeted
- **Build:** branch worktree based on `d78747f` with the current remediation batch · **Environment:** isolated local daemon at `127.0.0.1:64424` and web app at `127.0.0.1:3001`
- **Started:** 2026-08-26 · **Status:** closed

## Personas

| Persona | Base | Device / Network / Locale | Sessions |
|---|---|---|---|
| Ada | fresh isolated home | macOS, 1440×900, local network, en-US | CH-profiles-final |

## Flows in Scope

- `J-operate-profiles` — create, identify, switch, and restore named profiles.
- `J-scope-work-by-profile` — distinguish scoped work from aggregate work without losing ownership.

## Session Matrix & Results

| # | Charter | Journey / Scenario | Persona | Tour | Status | Issue | Fix commit |
|---|---|---|---|---|---|---|---|
| 1 | CH-profiles-final | J-operate-profiles / ET-profile-identity-appearance-catalogs | Ada | Identity catalogs | Pass | | |
| 2 | CH-profiles-final | J-operate-profiles / ET-profile-switcher-restore | Ada | Selection restoration | Pass | | |
| 3 | CH-profiles-final | J-scope-work-by-profile / ET-profile-web-aggregate-owner-surfaces | Ada | Aggregate ownership | Fixed | BUG-20260826-profile-palette-session-owner; BUG-20260826-profile-palette-worktree-owner | this remediation batch |
| 4 | CH-profiles-final | J-operate-profiles / profile-owned session mutation | Ada | CLI session mutation | Fixed | BUG-20260826-session-cli-profile-scope | this remediation batch |

## Session Debriefs

### CH-profiles-final — Ada

- **Ran:** 2026-08-26, one continuous isolated lab session (box respected: yes)
- **Findings:**
  - Aggregate session results and cross-profile worktree results lacked owner labels, creating a medium trust risk when names overlap.
  - Profile-aware session prompt and stop returned not found for a valid session, blocking CLI operation.
- **Bugs filed/updated:** BUG-20260826-profile-palette-session-owner, BUG-20260826-profile-palette-worktree-owner, BUG-20260826-session-cli-profile-scope
- **Scenarios settled:** ET-profile-identity-appearance-catalogs → pass; ET-profile-switcher-restore → pass; ET-profile-web-aggregate-owner-surfaces → pass after fixes
- **Paper cuts:** Canceled loop probes retained expected `Needs You` history after their intentionally missing `qa-empty` capability failed to import; no product invariant failed.
- **Surprises:** The browser intentionally kept its open profile after a CLI selection change while Settings immediately reflected the new remembered value.
- **Suggested next charter:** Exercise the remaining profile-selection precedence matrix, including environment overrides and invalid-profile exceptions for machine commands.

## What Was Fixed

### BUG-20260826-profile-palette-session-owner: Aggregate session search hid its owning profile

- **Symptom:** Aggregate session results were indistinguishable by owner.
- **Root cause:** The entity mapper discarded `profile_name` before rendering and search indexing.
- **Fix:** current remediation batch, one logical palette ownership fix.
- **Regression test:** `web/src/systems/os/hooks/__tests__/use-os-palette-root.test.tsx` — failed before, passes after.
- **Retested:** aggregate and scoped palette sessions in fresh browser loads.

### BUG-20260826-profile-palette-worktree-owner: Worktree search omitted its profile owner

- **Symptom:** A cross-profile worktree result did not say which profile owned it.
- **Root cause:** Worktree search models and row renderers omitted `profile_name`.
- **Fix:** current remediation batch, one logical palette ownership fix.
- **Regression test:** `web/src/systems/os/hooks/__tests__/use-os-palette-root.test.tsx` — failed before, passes after.
- **Retested:** worktree search from the owning and non-owning profile lenses.

### BUG-20260826-session-cli-profile-scope: Session prompt and stop ignored the selected profile

- **Symptom:** Prompt and stop returned not found for a valid non-default profile session.
- **Root cause:** Both commands lacked the profile-aware mutation hook.
- **Fix:** current remediation batch, one logical CLI scope fix.
- **Regression test:** `internal/cli/session_test.go` — failed before, passes after.
- **Retested:** live provider prompt and stop under `archive`, plus adjacent default and research session reads.

## Paper Cuts

| Persona | Where (journey/step) | Felt | Sharpness | Outcome |
|---|---|---|---|---|
| Ada | J-scope-work-by-profile, loop-run setup | Controlled canceled probes leave attention records after the missing capability fails. | dull | watching; expected real-runtime history |

## Runtime Errors Observed

- Three controlled loop probes failed to import the intentionally absent `qa-empty` capability before cancellation. The failure was confined to test setup and produced truthful terminal history; no registry bug was filed.

## Human Verifications Needed

None.

## Decisions for a Human

None.

## Learnings

- Ownership must be carried by search-result models as well as list models; a correct backend field is not enough when the command palette maps it away.
- Every session mutation command must install the same selected-profile client hook as session creation and listing.

## Final Status

- Smoke evidence: `/Users/pedronauck/dev/qa-labs/compozy-profiles-final-20260826-081429-551001-lab/qa-artifacts/qa/profile-trigger-research.png`
- Behavioral evidence: `/Users/pedronauck/dev/qa-labs/compozy-profiles-final-20260826-081429-551001-lab/qa-artifacts/qa/aggregate-loop-runs-owner-tags.png`
- **Final verification evidence:** `/Users/pedronauck/dev/qa-labs/compozy-profiles-final-20260826-081429-551001-lab/qa-artifacts/qa/final-make-verify.log`
- **Exit gate (full automated suite):** focused Go race tests and web Turbo tests passed; the repository-scoped `make gate` is recorded in `final-make-verify.log`.
- **Issues by user impact:** Blocks-Completion 1 · Data-Loss 0 · Trust-Damage 2 · Friction 0 · Cosmetic 0
- **Coverage:** 3/3 named profile scenarios walked; all adjacent regression checks passed.
- **Verdict:** PASS — ready after all three findings were fixed and re-walked.
