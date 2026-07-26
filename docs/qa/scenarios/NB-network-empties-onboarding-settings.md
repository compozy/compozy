---
id: NB-network-empties-onboarding-settings
area: NB
title: Oriented Network empties, onboarding mention, settings no enrollment
persona: Nia
journey: J-network-local-default
expected: Network area empty states answer orientation questions with one settings action; disabled empty names admin operators; onboarding links to Network without mutating settings; Network settings show availability + Live defaults/ceilings and state that they do not opt executions in, with no default_channel enrollment control.
entry_points: web /network and /settings/network; onboarding Workspaces step; public runtime Network/autonomy/config guides; agh skill view agh and bundled skills/agh Network guidance
qa_status: untested
bug_ids: BUG-20260715-network-ready-empty-unoriented
fix_status: fixed
retest_status: pass
fix_commits:
evidence: docs/qa/evidence/2026-07-14-network-changes/ch-network-local-default.md
last_report: docs/qa/reports/2026-07-14-network-changes.md
overlaps: NB-network-live-config-lifecycle
---

Planning flag for discoverability, settings, docs, and bundled-skill parity. Browser execution must cover ready and disabled empty states plus onboarding; a separate structured read must confirm that none of those visits changed settings or participation.

QA impact 2026-07-22: the Network mention moved from the deleted chat step to Workspaces. Status remains untested.
