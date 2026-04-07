# Issue 2 - Review Thread Comment

**File:** `internal/core/agent/registry_test.go:603`
**Date:** 2026-04-07 15:21:57 UTC
**Status:** - [x] RESOLVED

## Technical Disposition

`VALID` - the nil-runtime-config assertions were matching strings. The fix introduced package sentinel `ErrRuntimeConfigNil`, returned it from `ValidateRuntimeConfig` and `EnsureAvailable`, and updated the relevant tests in `internal/core/agent/registry_test.go` to use `errors.Is`.

## Body

_⚠️ Potential issue_ | _🟠 Major_

<details>
<summary>🧩 Analysis chain</summary>

🏁 Script executed:

```shell
#!/bin/bash
# Verify whether sentinel errors exist and whether tests rely on string matching.

set -euo pipefail

echo "== Potential sentinel/runtime-config error definitions =="
rg -n 'runtime config is nil|Err[A-Za-z0-9_]*Runtime|errors\.New\('

echo
echo "== String-based checks in agent registry tests =="
rg -n 'strings\.Contains\(err\.Error\(\),\s*"runtime config is nil"\)|ValidateRuntimeConfig\(nil\)|EnsureAvailable\(\s*context\.Background\(\),\s*nil' internal/core/agent/registry_test.go

echo
echo "== errors.Is/errors.As usage around related tests =="
rg -n 'errors\.(Is|As)\(' internal/core/agent/registry_test.go
```

Repository: compozy/compozy

Length of output: 16702

---



</details>

**Define a sentinel error and use `errors.Is()` for nil-config assertions.**

The tests at lines 590 and 602 use `strings.Contains(err.Error(), "runtime config is nil")` to match errors. Per coding guidelines, use `errors.Is()`/`errors.As()` instead. Define a sentinel error constant (e.g., `var ErrRuntimeConfigNil = errors.New("runtime config is nil")`) in the package, then update both test assertions and the two error returns in `registry_validate.go` and `registry_launch.go` to use it.

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/core/agent/registry_test.go` around lines 590 - 603, Add a
package-level sentinel error (e.g., var ErrRuntimeConfigNil =
errors.New("runtime config is nil")) and replace the string-based error checks
and string-returned errors with that sentinel: change the error returns in the
functions that currently return the "runtime config is nil" message (refer to
the error returns in registry_validate.go and registry_launch.go) to return
ErrRuntimeConfigNil, and update the tests that call ValidateRuntimeConfig and
EnsureAvailable to use errors.Is(err, ErrRuntimeConfigNil) (or errors.As where
appropriate) instead of strings.Contains(err.Error(), ...); ensure imports
include "errors" where added.
```

</details>

<!-- fingerprinting:phantom:poseidon:hawk:1c95559b-1220-4d4f-8066-bd9bc9b6b6b3 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Resolve

Thread ID: `PRRT_kwDORy7nkc55UjZO`

```bash
gh api graphql -f query='mutation($id:ID!){resolveReviewThread(input:{threadId:$id}){thread{isResolved}}}' -F id=PRRT_kwDORy7nkc55UjZO
```

---
*Generated from PR review - CodeRabbit AI*
