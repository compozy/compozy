---
id: ET-window-manager-hooks-resources
area: ET
title: Extend layouts without exposing pointer or execution authority
persona: Ada
journey: J-administer-window-manager
expected: Extension `window_layout` resources are schema-validated (documents of version 4, 5 and 6, converted to 6), scope-filtered, immutable to consumers, and applicable only through the semantic service; async hooks fire after committed layout apply, desktop create/delete, window move, open and close, and the new `window_manager.tab.split`, `window_manager.tab.unsplit` and `window_manager.tab.activated` with workspace/revision/change metadata (`changes.tab_ids`, `tab_split`, `tab_unsplit`); `window_manager.stack.activated` fires together with `tab.activated`; hooks subscribed to `window_manager.stack.grouped` or `stack.ungrouped` load, never fire, and `compozy hooks list` prints a warning naming the replacement; preview, rejection, pointer motion, focus, and desktop switching emit no extension hook.
entry_points: extension window_layouts; compozy hooks list; hook events window_manager.*; compozy layout arrange --resource
qa_status: untested
bug_ids:
fix_status:
retest_status: pass
fix_commits:
evidence: docs/qa/evidence/2026-08-01-window-tabs/agent-02-layouts-applies-now.png;docs/qa/reports/2026-08-01-window-tabs.md;/Users/pedronauck/dev/qa-labs/compozy-devtool-oss-launch-20260802-195112-911343-lab/qa-artifacts/qa
last_report: docs/qa/reports/2026-08-02-bundles-removal.md
overlaps: ET-window-manager-layout-recovery; ET-window-manager-public-parity
---

story: As an extension author, I can contribute deterministic layout intent and observe durable lifecycle changes without receiving mutable internals or pointer churn.

qa-impact: 2026-08-02 extension enable is now the sole publication path for authored layouts.
Reset to verify extension ownership, disable cleanup, and semantic application.

qa-impact: 2026-07-31 added opened, closed, activated, grouped, and ungrouped hooks for committed tab
operations while retaining pointer and focus silence. Reset for hook observability.

qa-impact: 2026-11-13 window-tabs-split S10. Hook vocabulary moved to tabs. Walk: Preconditions: an isolated lab from `eng-qa-bootstrap` running this build, one workspace open in the Web shell. Configure three logging hooks on `window_manager.tab.split`, `window_manager.tab.unsplit`, `window_manager.tab.activated`, and one on `window_manager.stack.grouped`. `compozy hooks list` prints `warning: window_manager.stack.grouped is retired and never fires; use window_manager.tab.split or window_manager.tab.unsplit for split transitions. It will be deleted in v0.8.0.` Split two tabs with `compozy window move --placement right --target <a>`: `tab.split` fires with the tab in `changes.tab_split`. `compozy window move --placement tab`: `tab.unsplit` fires. `compozy window activate <id> --client <c>`: `tab.activated` and `stack.activated` both fire. `stack.grouped` never fires. Install an extension whose `window_layouts` resource holds a version 5 document (see `packages/site/content/docs/extensions/develop.mdx`); `compozy layout arrange --resource <id>` applies it as tabs.
