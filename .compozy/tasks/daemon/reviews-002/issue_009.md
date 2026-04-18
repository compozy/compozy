---
status: resolved
file: internal/cli/daemon.go
line: 177
severity: nitpick
author: coderabbitai[bot]
provider_ref: review:4134973697,nitpick_hash:70c71bc00086
review_hash: 70c71bc00086
source_review_id: "4134973697"
source_review_submitted_at: "2026-04-18T19:43:56Z"
---

# Issue 009: Replace map[string]any payloads with typed output structs.
## Review Comment

Lines 177 and 248 use `map[string]any` for serialization of known response shapes. Replace with typed structs for compile-time field and type guarantees, aligning with the guideline "Do not use `interface{}`/`any` when a concrete type is known."

## Triage

- Decision: `valid`
- Root cause: the daemon status/stop JSON writers serialize fixed response shapes through `map[string]any`, which weakens compile-time guarantees for known fields and types.
- Fix plan: replace those payload maps with typed structs that preserve the current JSON schema and optional `daemon` field behavior.
- Resolution: `internal/cli/daemon.go` now uses typed JSON output structs for daemon status/stop responses, and `internal/cli/daemon_commands_test.go` verifies the stable JSON schema including omission of `daemon` when absent.
