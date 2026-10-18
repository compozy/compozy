---
id: MS-web-session-deeplink-global-confirm
area: MS
title: Deep link to a Global session turns Global scope on instead of selecting the home row
persona: Bruno
journey: J-operate-desktop-shell
expected: With a project workspace active, opening a link to retained operator-home history migrated to Global shows the switch confirmation in its Global variant — title "Turn on Global scope?", confirm "Turn on Global scope", and the description names Global (`~`), never the home folder name. Confirming turns Global scope on and opens the read-only transcript; the remembered project selection is untouched (persist key keeps the project id) and the toggle can still switch back. Declining stays on the not-found rendering. The old home workspace id is never written into `selectedWorkspaceId`.
entry_points: web /agents/$name/sessions/$id deep link; /session/$id permalink
qa_status: pass
bug_ids: BUG-20260906-stopped-history-schema-upgrade
fix_status: fixed
retest_status: pass
fix_commits: 7a9d15e2f
evidence: docs/qa/evidence/2026-10-02-untested/scope-legacy-crossdoc-bruno-confirm.json; docs/qa/evidence/2026-10-02-untested/scope-legacy-history-delivery-gate-retry.json; docs/qa/evidence/2026-10-02-untested/scope-legacy-history-evidence-audit-final.json
last_report: docs/qa/reports/2026-10-02-untested.md
overlaps: MS-web-menubar-global-scope-toggle; RT-missing-workspace-pruned
---

story: As a builder following a shared link to a Global session, I confirm a scope change — not a fake "workspace switch" to a folder I never registered.

Introduced 2026-08-12 by the deep-link hardening pass: pre-fix, confirming wrote the home row id into `selectedWorkspaceId`, permanently poisoning the remembered project and locking the toggle.

src: web/src/routes/_app/-session-workspace-switch.tsx; web/src/routes/_app/-session-workspace-switch-action.ts; web/src/systems/session/components/session-workspace-switch-dialog.tsx

2026-08-12 walk: blocked-verify. Unit coverage exercises the confirm action's store effects and the dialog variant renders in component scope; an isolated QA lab with a live daemon was not started, so a persona walk through the public deep-link entry points could not meet the qa-execution evidence standard.

QA 2026-10-04: a genuine released home-owned history upgraded to the current daemon cannot
resolve the desktop or session owner. The retained-history upgrade bug is reopened; current
scope-control repairs remain verified, while this historical link path requires repair and
fresh replay. See the dated report and scope-legacy evidence.

QA 2026-10-04 verified: the genuine beta.19 history upgrades losslessly and opens read-only through
both links after Global confirmation. Decline, reload, remembered-project restoration, a concurrent
project document and editable-target shortcut suppression all pass. HTTP/UDS/CLI preserve the
original three lifecycle events and physical database owner. Current affected gates and strict
lab evidence audit pass; both exact-manifest Global labs have clean teardown receipts. The fixture
has no authored provider narrative, and no such rendering claim is made. See the final Global
closure in the cycle report for the complete earlier scope-control legs and evidence.
