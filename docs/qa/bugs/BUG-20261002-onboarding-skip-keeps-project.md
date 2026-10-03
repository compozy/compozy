# BUG-20261002-onboarding-skip-keeps-project: Skip to home leaves the previous project active

- **Status:** verified
- **Impact (user-side):** Trust-Damage
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Dora
- **Journey Step:** J-operate-workspace-context, onboarding handoff
- **Scenarios:** MS-web-workspace-add-directory-browser
- **Found:** 2026-10-02 · **Report:** docs/qa/reports/2026-10-02-untested.md

## Summary

After adding Research notes, Dora reruns setup and chooses “Skip — use my home folder”. Compozy
returns to Research notes with Global scope off, including after refresh. The named destination
does not match the project in which subsequent work would start.

## Reproduction

- **Charter:** CH-untested-071-operate-workspace-context-dora · **Tour:** Back-Button Tour
- **Environment:** isolated macOS lab; desktop, wifi-fast, en-US; three registered projects.

1. Add Research notes through the desktop project browser and leave it selected.
2. Run the public `compozy onboarding reset --json` command and reload the desktop.
3. Select the advertised Codex GPT-6-Luna model, continue to Project, and press Skip — use my home folder.
4. Read the Global scope control and reload. Repeat once from fresh setup.

**Expected:** Explicit Skip selects Global scope, preserving registered projects and creating no
home-folder registration. The selected scope survives refresh.

**Actual:** Global scope has `aria-pressed=false`; Research notes remains active immediately and
after refresh in both walks. No session prompt was submitted. The earlier zero-project Skip walk
remains valid evidence for its distinct empty-catalog precondition.

## Evidence

- `docs/qa/evidence/2026-10-02-untested/workspace-browser-onboarding-global-scope-control.json`
- `docs/qa/evidence/2026-10-02-untested/workspace-browser-skip-scope-fresh-selector-retry.json`
- `docs/qa/evidence/2026-10-02-untested/workspace-browser-onboarding-skip-clean-retry.png`
- Recording: `/Users/pedronauck/.config/browser-harness/agent-workspace/recordings/compozy-onboarding-skip-scope-retry-20261002` (closed, 8 frames).

## Fix

- **Root cause:** Skip and Finish setup invoke the same wizard action. Completion updates the
  onboarding status but never carries the explicit Skip choice into the persisted workspace scope.
- **Repair:** Give Skip a distinct wizard action that enables Global after completion succeeds,
  using the existing scope store. Normal Finish retains the current scope; a failed completion
  preserves the draft and project selection. Remembered projects are not cleared or deleted.
- **Regression owner:** `use-onboarding-wizard.test.tsx`, successful Skip/Finish scope handoff and
  failed completion, plus existing busy-workspace guards. The browser replay owns button wiring
  and scope persistence across refresh.
- **Fix commit:** pending.

An earlier retry stopped on duplicate recent/catalog model labels; it collected no Skip verdict
and was replaced by the fresh recorded retry above.

## Verification

- **Retested:** 2026-10-02, Dora through fresh setup and production-page reloads.
- **Result:** Skip selects Global with three registered projects and retains it after refresh.
  Disabling Global returns to the remembered Research notes project. A separate normal Finish setup
  walk preserves that project scope across refresh. Neither path creates/resolves a workspace, and
  independent UDS reads confirm all three complete project records are unchanged. CLI status confirms
  completed setup. No model runtime result is claimed from selecting the advertised default.
- **Checks:** The three affected suites pass 22 tests, including successful Skip/Finish and failed
  completion. All 693 Web suites / 6,903 tests, typecheck, and production build pass; React Doctor is
  100/100. `make gate` passes all affected lanes with zero lint warnings/errors.
- **Evidence:** `onboarding-skip-scope-fixed-replay.json`, `onboarding-skip-global-fixed.png`,
  `onboarding-normal-finish-canary.json`, `onboarding-skip-scope-final-catalog.json`,
  `onboarding-skip-scope-final-status.json`, and `onboarding-skip-scope-*.log` under the dated directory.
- **Recording:** `/Users/pedronauck/.config/browser-harness/agent-workspace/recordings/compozy-onboarding-skip-scope-fixed-20261002`
  (closed, 15 frames).
