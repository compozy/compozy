# QA Run Report — 2026-10-30 — Directory picker input and step-strip numbers

- Scope: issue #1063, the directory picker input in Add project and onboarding step 2, the desktop
  native folder dialog, and step-strip number centring.
- Cadence: targeted.
- Build: working tree on `feat/issue-1063-directory-picker`.
- Environment: isolated temporary home, HTTP listener and UDS; real daemon, built Web bundle,
  headless Chromium, and the Electron desktop shell from this branch.
- Status: PASS for the targeted scenarios; PR CI and bot reviews remain delivery gates.

## Session Matrix

| Charter | Persona | Journey / scenario | Tour | Status |
| --- | --- | --- | --- | --- |
| Add project with the picker, browser and desktop | Dora, desktop, en-US | J-operate-workspace-context / MS-web-workspace-add-directory-browser | Feature | PASS |
| First-run project step with the picker | Lea, desktop, en-US | J-19 / RT-004 | Feature | PASS |

## Plan

Seed a temporary tree with two folders, a file, and an unreadable folder. In onboarding step 2 and
Add project, type a partial path, complete it from the suggestion list with Tab and with
ArrowDown+Enter, and add or submit it. Type each invalid path and confirm the message and the blocked
primary action. In the desktop shell, confirm the bridge exposes `dialogs.pickDirectory`, open the
native dialog from Browse…, choose a folder, and cancel a second time. Measure the step-strip glyph
offset before and after the change at 1x, 2x and 8x in light and dark.

## Session Debrief

- Onboarding: `…/projects/a` listed `alpha`; Tab produced `…/projects/alpha/` in the operator's own
  spelling (no symlink-resolved prefix); Add registered it once and cleared the field.
- Validation in both surfaces: missing path → "No folder at this path.", file → "This path is a file,
  not a folder.", mode 000 folder → "This folder can't be read. Check its permissions.", registered
  folder → "This folder is already a project.", relative → "Use an absolute path, like ~/code/app.",
  `~` → "Your home folder can't be a project. Choose a folder inside it."; the primary stayed disabled.
- Add project: ArrowDown+Enter chose `gamma/`, the name followed the folder, and one
  `POST /api/workspaces` carried `{"root_dir":"…/projects/delta","name":"delta"}`; the dialog closed.
- Desktop shell: `window.compozyShell.dialogs.pickDirectory` was a function; Browse… opened the
  native sheet at the typed folder with New Folder available; Open filled the field and enabled
  submit; Cancel left the field unchanged and re-enabled Browse….
- Plain web: no Browse… button renders.
- Step strip: the number sat 1.25–1.31 px below centre before and 0.25–0.31 px after (sub-pixel
  rounding of a 9 px figure in an 18 px badge), identical in light and dark; horizontal offset ≤0.13 px.
- Heavy test/lint/build delivery gates are delegated to PR CI as instructed; captures are linked from
  the PR.
