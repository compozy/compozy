# Issue 4 - Review Thread Comment

**File:** `internal/core/run/ui_adapter_test.go:271`
**Date:** 2026-04-06 11:28:05 America/Sao_Paulo
**Status:** - [x] RESOLVED

**Disposition:** VALID

**Rationale:** O teste deixou de depender de `runtime.NumGoroutine()`. A verificação agora se apoia apenas nos sinais determinísticos já expostos pelo adapter: `done`, fechamento do sink e `bus.SubscriberCount() == 0`.

## Body

_⚠️ Potential issue_ | _🟡 Minor_

**Goroutine count assertion may be flaky.**

The check `runtime.NumGoroutine() <= baseGoroutines+2` can be unreliable because goroutine counts fluctuate due to GC finalizers, runtime internals, and background activities. This may cause intermittent test failures.

Consider either increasing the tolerance or replacing this assertion with a more deterministic check (e.g., verifying that specific resources are released via a mock or channel signal).

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/core/run/ui_adapter_test.go` around lines 269 - 271, The
goroutine-count assertion using runtime.NumGoroutine() inside waitForCondition
is flaky; update the test to avoid relying on global goroutine counts: either
widen the tolerance (e.g., use baseGoroutines+5) or, preferably, replace the
NumGoroutine check with a deterministic signal from the component under test
(e.g., wait on a shutdown/completed channel or a mock hook exposed by the UI
adapter) so waitForCondition verifies that the adapter has released its
resources rather than comparing runtime.NumGoroutine; reference
waitForCondition, runtime.NumGoroutine and baseGoroutines to locate and change
the assertion.
```

</details>

<!-- fingerprinting:phantom:medusa:ocelot:f9061d8b-6785-40e8-a02a-a194e204112b -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Resolve

Thread ID: `PRRT_kwDORy7nkc55C9ua`

```bash
gh api graphql -f query='mutation($id:ID!){resolveReviewThread(input:{threadId:$id}){thread{isResolved}}}' -F id=PRRT_kwDORy7nkc55C9ua
```

---
*Generated from PR review - CodeRabbit AI*
