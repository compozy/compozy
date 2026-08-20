---
id: NB-network-empties-onboarding-settings
area: NB
title: Oriented Network empties, onboarding mention, settings no enrollment
persona: Nia
journey: J-network-local-default
expected: Network area empty states answer orientation questions with one settings action; disabled empty names admin operators; onboarding mentions Network in the workspace-step HelpTip without a link and without mutating settings; Network settings show availability + Live defaults/ceilings and state that they do not opt executions in, with no default_channel enrollment control.
entry_points: web /network and /settings/network; onboarding Workspaces step; public runtime Network/autonomy/config guides; compozy skill view compozy and bundled skills/compozy Network guidance
qa_status: untested
bug_ids: BUG-20260715-network-ready-empty-unoriented
fix_status: fixed
retest_status: pass
fix_commits: 8eeb8a38
evidence: docs/qa/evidence/2026-07-14-network-changes/ch-network-local-default.md;/Users/pedronauck/dev/qa-labs/compozy-qa-misc-network-goal-release-site-20260730-060405-932516-lab/qa-artifacts/qa
last_report: docs/qa/reports/2026-07-28-untested-full.md
overlaps: NB-network-live-config-lifecycle
---

Planning flag for discoverability, settings, docs, and bundled-skill parity. Browser execution must cover ready and disabled empty states plus onboarding; a separate structured read must confirm that none of those visits changed settings or participation.

QA impact 2026-07-22: the Network mention moved from the deleted chat step to Workspaces. Status remains untested.

QA impact 2026-07-26: the official skill entry point hard-cut to Compozy. Status remains untested.

2026-08-20 qa-impact: onboarding Network copy moved from a Workspaces paragraph into the step heading HelpTip. Reset for a copy walk.
