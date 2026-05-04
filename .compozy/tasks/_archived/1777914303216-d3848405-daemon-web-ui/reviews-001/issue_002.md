---
status: resolved
file: .compozy/tasks/daemon-web-ui/qa/logs/20-run-start-cookiejar.txt
line: 5
severity: minor
author: coderabbitai[bot]
provider_ref: review:4148025019,nitpick_hash:61ac837f9c5d
review_hash: 61ac837f9c5d
source_review_id: "4148025019"
source_review_submitted_at: "2026-04-21T13:30:56Z"
---

# Issue 002: Avoid committing live CSRF token values in QA log artifacts.
## Review Comment

Please redact/sanitize the cookie value (or keep this class of logs untracked) to reduce token exposure patterns and snapshot churn.

## Triage

- Decision: `valid`
- Notes:
  - The committed cookie jar currently stores a live CSRF token value from a manual browser run.
  - Root cause: QA log capture persisted a runtime token verbatim instead of a sanitized placeholder.
  - Intended fix: keep the artifact format but redact the sensitive cookie value to a stable placeholder.

## Resolution

- Replaced the live CSRF cookie value in `.compozy/tasks/daemon-web-ui/qa/logs/20-run-start-cookiejar.txt` with a stable redacted placeholder.
- Verified with `make verify`.
