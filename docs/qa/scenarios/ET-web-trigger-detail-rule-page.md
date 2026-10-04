---
id: ET-web-trigger-detail-rule-page
area: ET
title: Verify trigger detail rule page (When/If/Then, enable switch, rail, Inspect)
persona: Bruno
journey: J-24
expected: "`/triggers/$triggerId` renders the redesigned rule page: page-head sentence (When … if … run/start …) with a labeled Enable switch opposite it — PATCH `{enabled}` works for all sources, pending keeps the previous track state with an Enabling… label, disabled reveals the pause line; subhead = event pill · workspace · updated; main = RULE section with a When/If/Then card (webhook adds the local POST path with copy + curl; loop Then shows the loop link + mapping rows `←` from event / `=` static, no prompt) and RECENT RUNS as a single-open accordion (status pill + icon + meta + duration, drawer copy per status; Open session / Open loop run rendered only when the id exists — never disabled placeholders); rail = Properties / Public delivery (webhook only, gateway reachability copy) / Reliability / Identity collapsible cards + Inspect button + CLI hint; Inspect opens a right sheet with Diagnostics tiles and a Sample envelope JSON pane reconstructed from the trigger definition — signing secret reads presence only, never the value; config/package sources show the dashed lockbar + config.toml quiet note, hide Edit/Delete entirely, and keep the enable switch working; no Run now, no schedule/next-run anywhere."
entry_points: web `/triggers/$triggerId` (catalog row click or deep link)
qa_status: pass
bug_ids: BUG-20260815-trigger-detail-duplicate-key; BUG-20261003-trigger-error-hides-return
fix_status: fixed
retest_status: pass
fix_commits: self (the commit that records this fixed verdict)
evidence: docs/qa/evidence/2026-10-02-untested/trigger-preview-error-bruno-ended.json; docs/qa/evidence/2026-10-02-untested/trigger-recovery-bruno-ended.json; docs/qa/evidence/2026-10-02-untested/trigger-recovery-bruno-catalog-return-ready.json; docs/qa/evidence/2026-10-02-untested/trigger-recovery-bruno-managed-disabled-readback.json; docs/qa/evidence/2026-10-02-untested/trigger-rule-runs.png; docs/qa/evidence/2026-10-02-untested/trigger-rule-webhook-inspect.png; docs/qa/evidence/2026-10-02-untested/trigger-rule-compact-inspect.png; docs/qa/evidence/2026-10-02-untested/trigger-recovery-back-action.png; docs/qa/evidence/2026-10-02-untested/trigger-native-zoom-bruno.json
last_report: docs/qa/reports/2026-10-02-untested.md
overlaps: ET-web-jobs-triggers-catalog; TA-automation-crud-loop-target
---

Added by the triggers detail redesign (2026-08-15) — production translation of
`docs/design/opendesign/triggers/trigger-detail.html`,
`docs/design/opendesign/triggers/trigger-detail-loop.html`,
`docs/design/opendesign/triggers/trigger-detail-webhook.html`, and
`docs/design/opendesign/triggers/trigger-detail-states.html`.
Detail-surface expectations that previously rode along in `ET-web-jobs-triggers-catalog`
live here now.

Walk note: verify the Inspect sheet becomes visible in a real browser. A capture-time
probe found window-scoped `Sheet` stuck at `opacity: 0` on the OS-window portal path
(pre-existing platform finding, affects tasks/loops/vault sheets equally —
evidence in `.compozy/tasks/triggers-detail-redesign/evidence/visual/states/README.md`).

QA 2026-08-15: the first isolated Bruno walk confirmed the catalog → detail path,
Inspect visibility, sample envelope, and persisted enable switch, then found
`BUG-20260815-trigger-detail-duplicate-key`. The production fix gave the Delete
and Inspect overlays distinct identities. A fresh browser retest on port 4177
confirmed the detail and Inspect sheet render without reconciliation errors; the
dynamic and managed enable switches persisted across reloads, and compact 320x800,
deep-link, history, malformed-id, and keyboard/Escape probes passed.

QA 2026-08-15 post-rebase: a new isolated lab bootstrapped the final schema head,
recreated the workspace trigger through the real HTTP API, and repeated the
catalog → detail → Inspect path on Web port 4177. The final browser console had
no application errors.

2026-09-27 scope update: retired product surfaces were removed from this active scenario. Historical evidence remains in the dated reports; this revised contract requires a fresh walk.

QA 2026-10-03: Bruno completes rule, actual webhook runs, persisted dynamic/package toggles, Inspect, keyboard, history, compact viewport and native Chrome 200% zoom. The failed-detail return action was repaired and re-walked. Unchanged config-source evidence is reused as documented in the current report; authoring/catalog hardening remains owned by the overlapping catalog scenario.
