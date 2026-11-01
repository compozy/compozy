---
id: RT-acp-adapter-cold-start
area: RT
title: Recover cold npm adapter installation before ACP initialization
persona: Runtime Operator
journey: J-12-open-session-fast
expected: Cold npx preparation completes before initialize starts; concurrent preparations serialize and interrupted installs recover without deleting npm caches. Session failures and doctor retain recovery guidance.
entry_points: compozy session; compozy doctor; Loop run-agent
qa_status: partial
bug_ids: GitHub-1104
fix_status: implemented
retest_status: focused integration passed; Loop public walk remains CI-owned
fix_commits:
evidence: TestACPIntegrationNPXColdCache; TestLocalLauncherNPXPreparation; TestDoctorNPXStartupFailure
last_report:
overlaps: RT-session-native-stop; RT-session-fallback-chain
---

Use an isolated home and npm cache; never clear the operator's cache or restart
the operator's daemon. Start an unpinned npm ACP adapter from an empty cache,
interrupt preparation, then start it again. Confirm a real ACP initialize reply.
Start three sessions concurrently through a custom npx command and verify no
competing preparation, successful session creation, and no crash/quarantine for
the slow first install. Repeat through a Loop run-agent node in CI.

Hold initialize after process launch and confirm the ordinary 30-second deadline
still applies. Inspect the session failure and `compozy doctor --only provider`:
both must identify npx initialization and recommend retry/local command diagnosis.
A successful retry must clear the session failure diagnostic.

Owning checks: `TestLocalLauncherNPXPreparation`, `TestACPControlRequestDeadlines`,
`TestNPXStartupDiagnostic`, `TestDoctorNPXStartupFailure`, and opt-in real npm
`TestACPIntegrationNPXColdCache` (`COMPOZY_TEST_NPX_COLD=1`). Full local E2E and
`make gate` are excluded by the issue execution contract; CI owns full gates.

Local evidence (2026-11-01): real npm 11.19.1 / Node 24.22.1, isolated cache,
interrupted after 3 seconds, retry preparation completed in 73.4 seconds and real
Codex ACP initialize in 10.0 seconds. Three concurrent custom-command starts and
cancellation/retry passed under race detection. Doctor HTTP response included the
cached unresolved initialize failure. Windows cross-build passed; native Windows
execution and the Loop public-surface journey remain CI validation scope.
