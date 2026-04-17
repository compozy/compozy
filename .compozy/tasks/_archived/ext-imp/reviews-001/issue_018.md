---
status: resolved
file: internal/core/provider/overlay.go
line: 286
severity: nitpick
author: coderabbitai[bot]
provider_ref: review:4093952670,nitpick_hash:6f8e8f9d036f
review_hash: 6f8e8f9d036f
source_review_id: "4093952670"
source_review_submitted_at: "2026-04-11T16:18:34Z"
---

# Issue 018: Consider logging ignored Close() errors for debugging.
## Review Comment

Per coding guidelines, errors should not be ignored without justification. While ignoring cleanup errors during teardown is common, logging them at debug level would aid troubleshooting.

As per coding guidelines: "Do not ignore errors with `_`—every error must be handled or have a written justification."

## Triage

- Decision: `valid`
- Notes:
  - Confirmed by inspection. `closeOverlayBridges` ignores `Bridge.Close()` errors during overlay teardown.
  - Root cause: teardown is best-effort, but failures are discarded without any observability, which makes stuck bridge/process cleanup harder to troubleshoot.
  - Intended fix: keep teardown non-fatal while logging close failures with bridge/provider context.
  - Resolution: overlay bridge teardown now logs close failures with provider context instead of silently discarding them.
