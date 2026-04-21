---
status: resolved
file: internal/api/httpapi/openapi_contract_test.go
line: 195
severity: minor
author: coderabbitai[bot]
provider_ref: review:4148025019,nitpick_hash:77b46b643101
review_hash: 77b46b643101
source_review_id: "4148025019"
source_review_submitted_at: "2026-04-21T13:30:56Z"
---

# Issue 010: Filter path-item keys to real HTTP methods.
## Review Comment

This helper currently turns every key under a path item into a route key. If the spec starts using path-level `parameters`, `summary`, or `servers`, the test will invent fake routes and fail for the wrong reason. Restrict this loop to the OpenAPI operation methods only.

## Triage

- Decision: `valid`
- Notes:
  - `openAPIContractRouteKeys` currently treats every path-item key as a route method, so future path-level metadata would create false positives.
  - Root cause: the helper does not filter path-item entries to actual OpenAPI HTTP operation keys.
  - Intended fix: constrain the helper to the real HTTP method set only.

## Resolution

- Restricted the OpenAPI contract helper to real HTTP operation keys so path-level metadata no longer creates fake route expectations.
- Verified with `make verify`.
