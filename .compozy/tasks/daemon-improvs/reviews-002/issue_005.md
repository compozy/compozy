---
status: resolved
file: internal/logger/logger.go
line: 236
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc58ixsp,comment:PRRC_kwDORy7nkc654MFV
---

# Issue 005: _⚠️ Potential issue_ | _🟠 Major_
## Review Comment

_⚠️ Potential issue_ | _🟠 Major_

**Recover the active log handle when rotation fails.**

After Line 224 closes the file, any error from `rotateLogFiles` or `openLogFile` leaves `r.file` pointing at a closed descriptor. Subsequent writes then keep failing with a closed-file error, so one transient rotation failure can permanently brick daemon logging until restart.

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/logger/logger.go` around lines 221 - 236, The rotation code closes
r.file before calling rotateLogFiles/openLogFile which can leave r.file pointing
at a closed descriptor if rotation or reopening fails; change the logic to
preserve the active handle until a new file is successfully opened: keep the
existing r.file (and its descriptor) open, call rotateLogFiles using r.path,
attempt to open a new file via openLogFile into local variables (e.g., newFile,
newSize), and only if openLogFile succeeds assign r.file=newFile and
r.size=newSize and then close the previous file; if rotateLogFiles or
openLogFile fail, leave r.file intact and return the error so logging continues
uninterrupted (referencing r.file, rotateLogFiles, openLogFile, r.path,
r.maxRetainedFiles, r.filePerm).
```

</details>

<!-- fingerprinting:phantom:medusa:grasshopper:e40d238e-ad07-4f04-8bea-f476de16b781 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `valid`
- Root cause: `rotateIfNeededLocked` closes `r.file` before rotation and reopen steps. If either step fails, `r.file` still points at the now-closed descriptor, so later writes can fail permanently instead of continuing on the active handle.
- Plan: preserve the current file handle until rotation and reopen succeed, then swap in the new handle and close the previous one. Add coverage that a failed rotation does not brick subsequent logging once the filesystem problem is removed.
- Resolution: updated `internal/logger/logger.go` so rotation keeps the current handle open while the rename and reopen path runs, swaps the new file in only after `openLogFile` succeeds, and closes the previous descriptor last.
- Regression coverage: added `TestOpenRotatingFileKeepsWritingAfterRotationFailure` in `internal/logger/logger_test.go` to prove a failed retained-file cleanup no longer bricks later writes once the filesystem obstruction is removed.
- Verification: `go test ./internal/logger -run 'Test(OpenRotatingFileRotatesAtConfiguredSize|OpenRotatingFileKeepsWritingAfterRotationFailure|InstallDaemonLoggerForegroundMirrorsToStderrAndFile|InstallDaemonLoggerDetachedWritesOnlyFile)$' -count=1` passed. `make verify` then passed with `2544` tests and `2` skipped helper-process tests.
