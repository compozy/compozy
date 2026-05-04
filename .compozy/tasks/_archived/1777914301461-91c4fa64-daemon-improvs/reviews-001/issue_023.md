---
status: resolved
file: internal/logger/logger.go
line: 283
severity: nitpick
author: coderabbitai[bot]
provider_ref: review:4148016854,nitpick_hash:96d66402901c
review_hash: 96d66402901c
source_review_id: "4148016854"
source_review_submitted_at: "2026-04-21T13:29:50Z"
---

# Issue 023: Consider using filepath.Clean for path normalization.
## Review Comment

`normalizeFilePath` only trims whitespace but doesn't clean the path. Using `filepath.Clean` would normalize `..`, `.`, and redundant separators, preventing potential path traversal issues and ensuring consistent paths for rotation.

## Triage

- Decision: `valid`
- Root cause: `normalizeFilePath` trims whitespace but leaves redundant separators and `.` / `..` segments unnormalized, so the logger carries a non-canonical path string through validation, open, and rotation flows.
- Fix approach: canonicalize the trimmed path with `filepath.Clean` and add regression coverage that confirms validation/open use the normalized log path.
- Resolution: `normalizeFilePath` now returns `filepath.Clean(trimmedPath)`, and `TestNormalizeFilePathCleansRelativeSegments` verifies the helper output.
- Verification: `go test ./internal/logger` and `make verify`
