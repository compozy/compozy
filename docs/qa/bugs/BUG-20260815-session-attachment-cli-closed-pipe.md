# BUG-20260815-session-attachment-cli-closed-pipe: CLI hides upload rejection

- **Status:** fixed
- **Impact (user-side):** Trust-Damage
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Théo
- **Journey Step:** J-session-attachments, reject an oversized upload
- **Scenarios:** ET-session-attachment-oversize
- **Found:** 2026-08-15 · **Report:** docs/qa/reports/2026-08-15-session-attachments-pr-412-final.md

## Summary

An oversized CLI upload returned a generic closed pipe error instead of the daemon's actionable file-limit rejection.

## Reproduction

- **Charter:** CH-session-attachments · **Tour:** Network Tour
- **Environment:** macOS arm64, isolated daemon over UDS, en-US

1. Upload a file above session.attachments.max_file_bytes through compozy session attachments add.
2. Read the structured CLI result.

**Expected:** The CLI surfaces the daemon's configured-limit error.
**Actual:** Multipart producer cleanup won the error race and replaced the HTTP 413 response with closed pipe.

## Fix

- **Root cause:** The streaming multipart request had no exact content length, and cleanup returned its expected pipe teardown before the response decoder could preserve the daemon error.
- **Fix:** Publish the exact multipart length with Expect: 100-continue, decode non-success responses first, and suppress only the expected cleanup pipe error.
- **Regression suite:** internal/cli/client_test.go, TestUnixSocketClientSessionAttachmentUpload

## Verification

- **Retested:** 2026-08-15 in session-attachments-pr-412-final-20260815-195219-431614.
- **Result:** Passed. The real CLI returned the configured 10 MiB limit message; the direct Web build rendered the matching upload-limit error.
- **Evidence:** qa-artifacts/qa/cli-oversize.json; docs/qa/evidence/2026-08-15-session-attachments-pr-412-final/16-packaged-oversize-413.png.
