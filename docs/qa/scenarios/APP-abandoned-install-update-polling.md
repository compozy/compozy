---
id: APP-abandoned-install-update-polling
area: APP
title: Abandoned desktop install fails against the retired channel
persona: Dora
journey: J-desktop-update-moment
expected: An old abandoned app records its scheduled update-check failure while no retired feed object or compatibility redirect is restored.
entry_points: previous desktop app logs; retired distribution origin; current GitHub release
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: APP-app-auto-update; REL-beta-channel-contract
---

Added for the Electron hard cut. The walk must prove the failure stays local to the abandoned app
and that recovery is the documented manual download, not restoration of the retired channel.
