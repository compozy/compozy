---
status: resolved
file: pkg/compozy/runs/run.go
line: 824
severity: minor
author: coderabbitai[bot]
provider_ref: review:4148025019,nitpick_hash:f6d5318ce1d7
review_hash: f6d5318ce1d7
source_review_id: "4148025019"
source_review_submitted_at: "2026-04-21T13:30:56Z"
---

# Issue 029: Do not silently ignore malformed next_cursor in snapshot frames.
## Review Comment

`dispatchSnapshot` currently drops parse errors and continues, which can mask protocol/data issues and leave cursor state stale.

## Triage

- Decision: `valid`
- Notes:
  - The review is valid. `dispatchSnapshot` currently ignores `parseRemoteCursor` failures and continues, unlike the other cursor-decoding paths in this package and the internal daemon client.
  - Root cause: malformed `next_cursor` values are treated as if the cursor were simply absent, which can hide protocol/data corruption and leave caller cursor state stale.
  - Fix approach: return a decode error when `next_cursor` is malformed and add transport-stream regression coverage for that branch.

## Resolution

- Updated `pkg/compozy/runs/run.go` so `dispatchSnapshot` now returns `decode snapshot cursor` errors instead of silently dropping malformed cursors.
- Added transport regression coverage in `pkg/compozy/runs/transport_test.go` because this package’s stream parsing tests already live there.
- Verification:
- `go test ./internal/store/globaldb ./pkg/compozy/runs -count=1`
- `make verify`
