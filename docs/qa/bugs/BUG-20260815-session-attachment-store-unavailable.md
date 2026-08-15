# BUG-20260815-session-attachment-store-unavailable: Session attachment uploads return unavailable

- **Status:** fixed
- **Impact (user-side):** Blocks-Completion
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Théo
- **Journey Step:** J-session-attachments, upload
- **Scenarios:** ET-session-attachment-picker; ET-session-attachment-paste-reload; ET-session-attachment-multiple-drop
- **Found:** 2026-08-15 · **Report:** docs/qa/reports/2026-08-15-session-attachments-pr-412.md

## Summary

Every supported attachment upload returned HTTP `503`, so picker, paste, and drop previews could not become sendable session attachments.

## Reproduction

- **Charter:** CH-session-attachments · **Tour:** Feature Tour
- **Environment:** macOS arm64, isolated daemon and web app, en-US

1. Start the daemon with session attachments enabled.
2. Open a session and choose a supported Markdown or image file.
3. Wait for the upload to complete.

**Expected:** The upload returns `201` and the composer shows a ready preview.
**Actual:** The upload returned `503` with `session attachment store is not configured`.

## Fix

- **Root cause:** Daemon boot constructed the session manager and server dependencies before it initialized attachment storage, permanently injecting a nil store into those owners.
- **Fix commit:** `2603eed`
- **Regression suite:** `internal/daemon/daemon_test.go`, `TestBootWithNetworkDisabledKeepsDaemonOperational`

## Verification

- **Retested:** 2026-08-15 in `compozy-session-attachments-pr-412-20260815-103704-955265-lab`.
- **Result:** Passed. Picker, paste, and drop uploads returned `201`; prompt dispatch, cold reload, scoped byte reads, and cleanup all completed through public surfaces.
- **Evidence:** `docs/qa/evidence/2026-08-15-session-attachments-pr-412/02-multiple-files-ready.png`; `docs/qa/evidence/2026-08-15-session-attachments-pr-412/07-pasted-image-reloaded.png`.
