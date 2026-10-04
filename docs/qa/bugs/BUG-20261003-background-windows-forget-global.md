# BUG-20261003-background-windows-forget-global: Background windows erase the saved Global scope

- **Status:** verified
- **Impact (user-side):** Trust-Damage
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Bruno
- **Journey Step:** Keep Global selected while another desktop document is open, then reload
- **Scenarios:** MS-web-menubar-global-scope-toggle; ET-web-jobs-triggers-catalog
- **Found:** 2026-10-03 · **Report:** docs/qa/reports/2026-10-02-untested.md

## Summary

Bruno selects Global in one Chrome tab. Another tab remains in a project. Opening an app changes
the shared window catalog; the first tab still displays Global, but reloading returns to the
other tab's project and its remembered profile. The user made no new project selection there.

## Reproduction

- **Discovery charter:** CH-038; **replay owner:** CH-global-scope-regression · Feature Tour.
- **Environment:** real target daemon/Web on 55651; two Chrome tabs, desktop en-US.
- **Build:** 61181c15d, current production Web build; no scope repair applied.

1. Keep one tab in Recovery Editorial and the default profile.
2. In another tab, explicitly enable Global and confirm the Global chip.
3. Open Home from the second tab's dock, adding an app window to the shared desktop.
4. The background project tab receives that window-catalog update.
5. Reload the Global tab. It returns to Recovery Editorial.

**Expected:** background window maintenance cannot replace an explicitly saved scope choice.
An already-open tab keeps its own view until the user changes or reloads it.
**Actual:** background housekeeping writes its whole old workspace snapshot to shared storage.

## Evidence

All artifacts are under docs/qa/evidence/2026-10-02-untested/:

- automation-global-detail-config-trigger-reload-state.json/.png: original persona failure.
- automation-global-detail-bruno-ended.json: session ended before source inspection.
- automation-scope-persistence-single-document.json: Global survives after old owned tabs close.
- automation-scope-persistence-two-document-{setup,observed}.json: two current-build tabs reproduce
  the overwrite; the Global view remains visible while the saved snapshot already says workspace.

The second pair is engineering diagnosis, not a completed persona-scenario verdict. No unrelated
browser tabs were closed; the cleanup receipts identify only this lab's previous documents.

## Fix

- **Root cause:** useDesktopWorktreeScope dispatches window-scope pruning when the daemon's window
  set changes. The persisted workspace store writes a full snapshot for every event, including
  unchanged transitions. A stale document therefore republishes its navigation choice during
  housekeeping. A newly mounted workspace reader also dispatches a redundant desktop observation.
- **Fix commit:** 62b58628b62a03048967a91543323d3cb427bb7b
- **Regression test:** existing workspace hook/persistence suite, use-workspaces.test.tsx. Cover
  window additions and removals while another document has saved Global, then rehydrate.

## Verification

The persistence subscription now excludes derived window-pruning events. A workspace reader
also avoids dispatching an already-resolved desktop observation. Explicit user preferences keep
the existing storage key, version and migration; each open document retains its current view.

Both new owning regression cases fail before the repair and pass afterward. The focused hook
and store suites pass 42 tests. Root Turbo build, lint and typecheck pass; React Doctor retains
the same two pre-existing automation complexity warnings with no workspace warning.

Fresh Bruno replay verifies Global across window addition, window removal and reload while a
second document stays in its project. Keyboard and command-palette toggles work, editable search
skips the shortcut, explicit profile selection preserves Global, and the compact leading cluster
remains usable. Final desktop and compact screenshots were inspected; the 30-frame recording
is stopped. No source or storage inspection occurred during the persona walk.

Evidence: workspace-background-scope-{red,green,web,doctor}.json,
workspace-background-scope-bruno-ended.json and workspace-background-scope-{final-global,compact}.png.
The required delivery gate passes every selected lane, including 693 Web files / 6,910 tests.
Receipt: workspace-background-scope-delivery-gate.json/.log. The full scope scenario retains its catalog-loading,
zero/no-remembered selection and Global-session deep-link legs; its matrix row remains Pending.

Final delivery receipts: workspace-background-scope-{final-gate,final-gate-status,audit}.json.

## Regressed (2026-10-04)

Bruno cannot enable Global while a scoped Loop run remains open in another OS tab. Unlike
the earlier cross-document storage overwrite, this path immediately restores the project
inside the same document. A clean browser document reproduces it. Closing only the owned
Loop run window makes Global and Home navigation work on the unchanged build.

The remaining writer is LoopsWindow: its layout effect adopts the route workspace whenever
the active workspace changes, including explicit Global selection and background mounts.
The earlier persistence correction remains valid; this is a separate route-adoption path
under the same user-visible background-window symptom.

- **Discovery:** CH-automation-catalog-recovery, Interrupt Tour, Bruno desktop en-US.
- **Evidence:** docs/qa/evidence/2026-10-02-untested/catalog-recovery-bruno-retry-global.json;
  catalog-global-retry.png; catalog-global-loop-window-diagnostic.json.
- **Owning invariant:** explicit Loop navigation may adopt its known workspace once; retained
  windows cannot undo a later shell scope choice or change another window's destination.
- **Canonical regression:** web/e2e/__tests__/loops.spec.ts, scoped Loop windows preserve later
  Global scope choices. Uses real daemon reads and a pure transform Loop; no agent runs.
- **Repair:** LoopsWindow adopts a known route owner when foreground route ownership/focus
  resolves. The effect no longer responds to later active-workspace changes, and a retained
  background window cannot adopt a project. Current source is verified; commit stamp pending.
- **Regression:** the focused real-daemon Loops E2E fails before the repair and passes afterward.
  An initial post-fix run used home instead of the registered dashboard app identity; its
  screenshot already shows Home with Global active. Correcting that selector preserves the
  assertions and passes the complete case.
- **Original-persona replay:** a fresh Bruno document opens the saved scoped Loop run, adopts
  Recovery Editorial, then explicitly selects Global. Home, Jobs and reload preserve Global
  with the Loop still retained. A failed global job also preserves its detail and scope, agreeing
  with CLI history. Later explicit scoped Loop navigation still selects its owner.
- **Evidence:** catalog-loop-scope-{red,green,green-corrected}.json/.log;
  catalog-recovery-replay-{global,global-failed-job,global-failed-history,loop-seed-entry,
  bruno-ended}.json; catalog-recovery-global-fixed.png. Evidence paths use this cycle's directory.

The full Global toggle scenario remains Pending for its separate pending-resolution,
zero/no-remembered-selection and session-deep-link legs; the repaired defect is verified.
