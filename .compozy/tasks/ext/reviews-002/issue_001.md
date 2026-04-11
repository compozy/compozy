---
status: resolved
file: internal/cli/makefile_publish_test.go
line: 14
severity: nitpick
author: coderabbitai[bot]
provider_ref: review:4093340073,nitpick_hash:686cd0fbe2f0
review_hash: 686cd0fbe2f0
source_review_id: "4093340073"
source_review_submitted_at: "2026-04-11T03:34:14Z"
---

# Issue 001: Avoid exact full-line dependency matching for Make target prerequisites.
## Review Comment

`strings.Contains(makefile, "publish-extension-sdks: verify build-extension-sdks")` is brittle to harmless prerequisite reordering/spacing. Consider parsing the target line and asserting required deps by membership (`verify`, `build-extension-sdks`) instead of exact sequence.

## Triage

- Decision: `valid`
- Notes:
  - The current test asserts the full target definition as one literal string, so harmless prerequisite reordering or spacing changes would fail the test even if the dependency contract is still correct.
  - The fix is to parse the `publish-extension-sdks` target line, assert membership for `verify` and `build-extension-sdks`, and keep the publish command coverage separate.
  - Resolved by adding `mustMakeTargetPrereqs()` and switching the prerequisite checks to membership assertions.
