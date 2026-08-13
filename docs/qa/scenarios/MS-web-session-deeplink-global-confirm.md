---
id: MS-web-session-deeplink-global-confirm
area: MS
title: Deep link to a Global session turns Global scope on instead of selecting the home row
persona: Bruno
journey: J-operate-desktop-shell
expected: With a project workspace active, opening a link to a session owned by the operator-home registration shows the switch confirmation in its Global variant — title "Turn on Global scope?", confirm "Turn on Global scope", and the description names Global (`~`), never the home folder name. Confirming turns Global scope on and re-enters the link; the remembered project selection is untouched (persist key keeps the project id) and the toggle can still switch back. Declining stays on the not-found rendering. The home workspace id is never written into `selectedWorkspaceId`.
entry_points: web /agents/$name/sessions/$id deep link; /session/$id permalink
qa_status: blocked-verify
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: web/src/routes/_app/__tests__/-session-workspace-switch-action.test.ts
last_report:
overlaps: MS-web-menubar-global-scope-toggle; RT-missing-workspace-pruned
---

story: As a builder following a shared link to a Global session, I confirm a scope change — not a fake "workspace switch" to a folder I never registered.

Introduced 2026-08-12 by the deep-link hardening pass: pre-fix, confirming wrote the home row id into `selectedWorkspaceId`, permanently poisoning the remembered project and locking the toggle.

src: web/src/routes/_app/-session-workspace-switch.tsx; web/src/routes/_app/-session-workspace-switch-action.ts; web/src/systems/session/components/session-workspace-switch-dialog.tsx

2026-08-12 walk: blocked-verify. Unit coverage exercises the confirm action's store effects and the dialog variant renders in component scope; an isolated QA lab with a live daemon was not started, so a persona walk through the public deep-link entry points could not meet the qa-execution evidence standard.
