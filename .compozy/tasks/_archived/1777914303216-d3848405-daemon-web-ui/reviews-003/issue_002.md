---
status: resolved
file: internal/api/httpapi/openapi_contract_test.go
line: 136
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc58j9Bi,comment:PRRC_kwDORy7nkc655wbi
---

# Issue 002: _⚠️ Potential issue_ | _🟡 Minor_
## Review Comment

_⚠️ Potential issue_ | _🟡 Minor_

**Split these route/body/status checks into named subtests.**

These loops stop on the first failing case and make the failures less isolated than they need to be. Please wrap each case in `t.Run("Should ...")`, and use `t.Parallel()` inside the independent subtests.

<details>
<summary>Example pattern</summary>

```diff
- for _, routeKey := range workspaceScopedRoutes {
- 	operation := getOperation(t, spec, routeKey)
- 	if !hasParameterRef(operation, "#/components/parameters/ActiveWorkspaceHeader") {
- 		t.Fatalf("%s is missing ActiveWorkspaceHeader", routeKey)
- 	}
- }
+ for _, routeKey := range workspaceScopedRoutes {
+ 	routeKey := routeKey
+ 	t.Run("Should require ActiveWorkspaceHeader for "+routeKey, func(t *testing.T) {
+ 		t.Parallel()
+ 		operation := getOperation(t, spec, routeKey)
+ 		if !hasParameterRef(operation, "#/components/parameters/ActiveWorkspaceHeader") {
+ 			t.Fatalf("%s is missing ActiveWorkspaceHeader", routeKey)
+ 		}
+ 	})
+ }
```
</details>


As per coding guidelines, `**/*_test.go`: `MUST use t.Run("Should...") pattern for ALL test cases` and `Use t.Parallel() for independent subtests`.


Also applies to: 145-150, 177-196

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/api/httpapi/openapi_contract_test.go` around lines 125 - 136, The
loop over workspaceScopedRoutes in openapi_contract_test.go causes failures to
abort the whole loop; change each iteration into an isolated subtest by wrapping
the body for each routeKey in t.Run("Should ...", func(t *testing.T){
t.Parallel(); ... }) so each assertion (calls to getOperation, hasParameterRef
checks for "#/components/parameters/ActiveWorkspaceHeader" and negative check
for "WorkspaceQuery", and hasResponse("412")) runs as its own parallel subtest;
apply the same t.Run/t.Parallel conversion pattern to the other loops mentioned
(the blocks around lines 145-150 and 177-196) so all cases are independent and
report failures individually.
```

</details>

<!-- fingerprinting:phantom:medusa:grasshopper:d20456a7-cc7a-49e0-a8b3-b8b4348e2552 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `valid`
- Notes:
- The loops at `openapi_contract_test.go:125-136`, `145-150`, and `184-196` still execute multiple route assertions inside a single parent test body.
- Root cause: failures inside those loops abort the remaining cases and do not follow the repo's required `t.Run("Should...")` subtest style for independent cases.
- Fix plan: convert each loop iteration into a named parallel subtest while keeping the shared OpenAPI document loaded once in the parent test.
- Implemented: converted the workspace-context, request-body, `TransportError` required-field, and browser-security response loops into named `Should...` subtests with `t.Parallel()`.
- Verification:
- `go test ./internal/api/httpapi -run 'TestBrowserOpenAPIContractKeepsWorkspaceContextAndProblemSemantics$' -count=1`
- `make verify`
