---
id: APP-desktop-runtime-bundle-coherence
area: APP
title: Opening an updated app replaces its stale app-owned runtime
persona: Bruno
journey: J-desktop-attach-daily
expected: When the app bundle is newer than a healthy running runtime that the desktop app owns, opening the app stops only that owned process, atomically installs the bundled runtime, publishes matching provenance, starts it, and reaches the existing product state without deleting the Compozy home. A newer running runtime is retained and attached only if compatible; a newer verified installed runtime is never replaced by an older bundle. If replacement or provenance publication fails, the prior binary remains recoverable and no half-published identity is trusted.
entry_points: packaged CompozyOS app after an N to N+1 app update; compozy status in the same isolated home
qa_status: pass
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-runtime-ui-regressions-20260827-155435-128437-lab/qa-artifacts/qa/runtime-ui-proof.md
last_report: docs/qa/reports/2026-09-09-issue-559-safe-update-recovery.md
overlaps: APP-attach-running-daemon; APP-start-installed-daemon; APP-runtime-update-app-owned
---

QA impact 2026-08-27: the desktop bootstrap used to attach to any healthy compatible runtime before
comparing the installed app-owned binary with the runtime bundled by the new app. An app update
could therefore leave the old daemon running indefinitely and surface unrelated session failures
until the operator deleted `~/.compozy`. This scenario owns the narrower launch-time coherence
contract; the broader consent-driven update flow remains owned by `APP-runtime-update-app-owned`.

Issue 559 re-walk: `docs/qa/reports/2026-09-09-issue-559-safe-update-recovery.md`.

QA impact 2026-10-16 (#742): after a one-click update the reopened app kept rendering the previous
release's Web bundle. The runtime served `index.html` without `Cache-Control`, and Chromium reused
the cached shell heuristically, so the new runtime's cards never loaded. The runtime now marks the
shell and unversioned public files `no-cache`, and the product window loads the shell with
`pragma: no-cache` so a shell cached by an older runtime is also skipped. Re-walk an N → N+1 update
and confirm the reopened app requests the new `assets/index-*.js`.

2026-11-03 cache retest: use a throwaway desktop home with an HTTP cache above 256 MiB.
Launch twice and confirm the oversized HTTP cache is cleared asynchronously on the first launch,
then kept under the Chromium 256 MiB limit (allowing eviction slack). Seed and re-read IndexedDB,
localStorage, cookies and persisted window/layout state across both launches. Cleanup uses only
`Session.clearCache()`. Code Cache (64 MB in the incident) and GPU caches (under 12 MB combined)
remain Chromium-managed and are outside this HTTP-only budget. Daemon response headers are unchanged.

## Windows desktop acceptance (2026-11-04)

Run on Windows x64 with an isolated `COMPOZY_HOME` and install directory containing spaces and
non-ASCII characters. Install the signed NSIS baseline without administrator elevation. Confirm
offline bundled-runtime provisioning, product readiness and the existing `app status`, `app open`,
`app retry` and diagnostic export commands over the authenticated local named pipe. A second
launch must retain one shell and deliver the requested deep link.

Stage a verified next-version installer through the existing update journal. Observe actual NSIS
installation, relaunch on the new app version, matching owned-runtime version and archived
`updated` outcome. The original configuration and workspace data must remain readable. Uninstall
the app and verify those files still exist. The native `Desktop (Windows install and update)` CI
lane owns this acceptance; pending CI is not a passing Windows receipt.
