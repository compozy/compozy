---
status: resolved
file: internal/api/httpapi/openapi_contract_test.go
line: 274
severity: nitpick
author: coderabbitai[bot]
provider_ref: review:4149120998,nitpick_hash:384a25e1b484
review_hash: 384a25e1b484
source_review_id: "4149120998"
source_review_submitted_at: "2026-04-21T15:56:28Z"
---

# Issue 004: Unnecessary wrapper function.
## Review Comment

`stringsSplitN` is a trivial pass-through to `strings.SplitN`. Consider calling `strings.SplitN` directly in `getOperation`.

## Triage

- Decision: `valid`
- Notes:
  - `stringsSplitN` is a one-line pass-through used only by `getOperation`, so it adds indirection without encapsulating any behavior.
  - Implemented: removed the wrapper and call `strings.SplitN` directly in `getOperation`.
  - Verification: `go test ./internal/api/httpapi -run 'Test(DevProxyRoutesServeFrontendRequests|DevProxyRoutesStripDaemonCredentialsBeforeForwarding|DevProxyRoutesBypassAPIAndUnsupportedMethods|DevProxyReturnsBadGatewayWhenUpstreamIsUnavailable|NewWithDevProxyTargetPrefersProxyOverEmbeddedStaticFS|BrowserOpenAPIContractMatchesRegisteredBrowserRoutes)$' -count=1`
  - Repo gate note: the attempted full `make verify` run stopped earlier in `frontend:bootstrap` because of the unrelated pre-existing `package.json`/`bun.lock` mismatch.
