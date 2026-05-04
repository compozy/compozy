---
status: resolved
file: internal/api/httpapi/openapi_contract_test.go
line: 222
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc58gqXR,comment:PRRC_kwDORy7nkc651WG_
---

# Issue 011: _⚠️ Potential issue_ | _🟠 Major_
## Review Comment

_⚠️ Potential issue_ | _🟠 Major_

**This helper hides undocumented registered routes.**

Because you only append keys already present in `expectedSet`, any extra browser route registered in Gin is silently ignored. That weakens the contract test: the router can drift ahead of the spec without failing. Compare the full normalized registered browser route set against the spec-derived set instead.

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/api/httpapi/openapi_contract_test.go` around lines 208 - 222,
registeredBrowserRouteKeys currently filters out any registered Gin routes not
present in the expected spec, hiding extra/undocumented routes; change it to
collect and return the full normalized set of registered route keys instead of
only those in expectedSet by iterating over routes, building keys with
route.Method + " " + ginPathParamPattern.ReplaceAllString(route.Path, "{$1}")
for every route, storing them in a set/slice, sorting the result, and returning
it so the test can compare the complete registered set against the spec-derived
expected set.
```

</details>

<!-- fingerprinting:phantom:medusa:grasshopper:e75280e4-7172-485f-934b-c3510e24ebf0 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `valid`
- Notes:
  - The current registered-route helper only returns routes already present in the expected list, which can hide undocumented HTTP routes and let the contract silently drift.
  - Root cause: the test normalizes the registered set through `expectedSet` instead of comparing the full registered browser-route surface.
  - Intended fix: return the complete normalized registered set and align the browser contract/spec artifacts with the actual HTTP surface where needed.

## Resolution

- Changed the registered-route helper to compare against the full normalized browser-route set and documented the intentional exclusions needed to keep the contract explicit.
- Verified with `make verify`.
