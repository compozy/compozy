---
id: ET-extension-palette-contributions
area: ET
title: Use extension commands and views in the command palette
persona: Bruno
journey: J-extension-dev-lifecycle
expected: Enabling a valid extension adds its namespaced commands, declarative views, and free default shortcuts to the selected workspace; conflicts stay dormant with their owner named, unhealthy entries stay visible but unavailable with the runtime reason, disabling removes membership without deleting operator overrides, and a valid dev reload replaces the projection while a broken reload keeps the last good one.
entry_points: `compozy extension build|validate|enable|disable|dev|reload`; `compozy cmd-palette list|inspect --source ext.<name>`; Command-K; Settings > Extensions > Palette
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: ET-palette-registry-driven-root; ET-palette-nested-views; ET-extension-dev-reload-loop; ET-web-command-palette-shortcuts
---

Task 07 introduced the `resources.cmd_palette` extension family, its workspace projection, the
extension-default shortcut tier, declarative view invocation, and the per-extension Settings panel.
Walk the published and dev-overlay paths with the Go `notes` fixture, including a shortcut collision,
an unhealthy runtime, disable and re-enable, one valid reload, and one invalid reload.
