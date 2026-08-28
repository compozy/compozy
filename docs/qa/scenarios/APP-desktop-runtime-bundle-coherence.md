---
id: APP-desktop-runtime-bundle-coherence
area: APP
title: Opening an updated app replaces its stale app-owned runtime
persona: Bruno
journey: J-desktop-attach-daily
expected: When the app bundle is newer than a healthy running runtime that the desktop app owns, opening the app stops only that owned process, atomically installs the bundled runtime, publishes matching provenance, starts it, and reaches the existing product state without deleting the Compozy home. If replacement or provenance publication fails, the prior binary remains recoverable and no half-published identity is trusted.
entry_points: packaged CompozyOS app after an N to N+1 app update; compozy status in the same isolated home
qa_status: pass
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-runtime-ui-regressions-20260827-155435-128437-lab/qa-artifacts/qa/runtime-ui-proof.md
last_report: docs/qa/reports/2026-08-27-runtime-ui-regressions.md
overlaps: APP-attach-running-daemon; APP-start-installed-daemon; APP-runtime-update-app-owned
---

QA impact 2026-08-27: the desktop bootstrap used to attach to any healthy compatible runtime before
comparing the installed app-owned binary with the runtime bundled by the new app. An app update
could therefore leave the old daemon running indefinitely and surface unrelated session failures
until the operator deleted `~/.compozy`. This scenario owns the narrower launch-time coherence
contract; the broader consent-driven update flow remains owned by `APP-runtime-update-app-owned`.
