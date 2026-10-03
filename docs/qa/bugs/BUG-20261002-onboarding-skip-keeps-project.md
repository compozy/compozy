# BUG-20261002-onboarding-skip-keeps-project: Skip to home leaves the previous project active

- **Status:** open
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

Pending investigation after the persona walk. An earlier retry stopped on duplicate recent/catalog
model labels; it collected no Skip verdict and was replaced by the fresh recorded retry above.

## Verification

Pending repair, original-persona replay, and adjacent normal Finish setup canary.
