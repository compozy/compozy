---
id: ET-web-trigger-detail-rule-page
area: ET
title: Verify trigger detail rule page (When/If/Then, enable switch, rail, Inspect)
persona: Bruno
journey: J-24
expected: "`/triggers/$triggerId` renders the redesigned rule page: page-head sentence (When … if … run/start …) with a labeled Enable switch opposite it — PATCH `{enabled}` works for all sources, pending keeps the previous track state with an Enabling… label, disabled reveals the pause line; subhead = event pill · workspace · updated; main = RULE section with a When/If/Then card (webhook adds the local POST path with copy + curl; loop Then shows the loop link + mapping rows `←` from event / `=` static, no prompt) and RECENT RUNS as a single-open accordion (status pill + icon + meta + duration, drawer copy per status; Open session / Open loop run rendered only when the id exists — never disabled placeholders); rail = Properties / Public delivery (webhook only, gateway reachability copy) / Reliability / Identity collapsible cards + Inspect button + CLI hint; Inspect opens a right sheet with Diagnostics tiles and a Sample envelope JSON pane reconstructed from the trigger definition — signing secret reads presence only, never the value; config/package sources show the dashed lockbar + config.toml quiet note, hide Edit/Delete entirely, and keep the enable switch working; no Run now, no schedule/next-run anywhere."
entry_points: web `/triggers/$triggerId` (catalog row click or deep link)
qa_status: pass
bug_ids: BUG-20260815-trigger-detail-duplicate-key
fix_status: fixed
retest_status: pass
fix_commits: self (the commit that records this fixed verdict)
evidence: docs/qa/evidence/2026-08-15-triggers-ui/webhook-detail.png; docs/qa/evidence/2026-08-15-triggers-ui/inspect-sample-envelope.png; docs/qa/evidence/2026-08-15-triggers-ui/webhook-disabled-after-reload.png; docs/qa/evidence/2026-08-15-triggers-ui/compact-320x800.png; docs/qa/evidence/2026-08-15-triggers-ui/port-4177-retest.png
last_report: docs/qa/reports/2026-08-15-triggers-ui.md
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
(pre-existing platform finding, affects tasks/loops/vault/sandbox sheets equally —
evidence in `.compozy/tasks/triggers-detail-redesign/evidence/visual/states/README.md`).

QA 2026-08-15: the first isolated Bruno walk confirmed the catalog → detail path,
Inspect visibility, sample envelope, and persisted enable switch, then found
`BUG-20260815-trigger-detail-duplicate-key`. The production fix gave the Delete
and Inspect overlays distinct identities. A fresh browser retest on port 4177
confirmed the detail and Inspect sheet render without reconciliation errors; the
dynamic and managed enable switches persisted across reloads, and compact 320x800,
deep-link, history, malformed-id, and keyboard/Escape probes passed.
