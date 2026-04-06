# Issue 6 - Review Thread Comment

**File:** `pkg/compozy/events/bus_test.go:170`
**Date:** 2026-04-06 11:28:05 America/Sao_Paulo
**Status:** - [x] RESOLVED

**Disposition:** VALID

**Rationale:** O teste de `Bus` agora verifica diretamente o estado observável do componente depois do `Close()`: contador de subscribers zerado e todos os canais fechados, drenando qualquer buffer remanescente em vez de depender de contagem global de goroutines.

## Body

_⚠️ Potential issue_ | _🟡 Minor_

<details>
<summary>🧩 Analysis chain</summary>

🏁 Script executed:

```shell
# First, locate the bus-related files
find . -type f -name "*bus*" | head -20
```

Repository: compozy/compozy

Length of output: 284

---

🏁 Script executed:

```shell
# Get the full test file to understand the context
cd pkg/compozy/events && wc -l bus_test.go
```

Repository: compozy/compozy

Length of output: 74

---

🏁 Script executed:

```shell
# View the specific test function and surrounding context
cd pkg/compozy/events && sed -n '110,175p' bus_test.go
```

Repository: compozy/compozy

Length of output: 1407

---

🏁 Script executed:

```shell
# Examine the bus.go implementation to understand goroutine ownership
cd pkg/compozy/events && cat -n bus.go | head -100
```

Repository: compozy/compozy

Length of output: 2701

---

🏁 Script executed:

```shell
# Check the Close method implementation
cd pkg/compozy/events && rg "func.*Close" -A 20 bus.go
```

Repository: compozy/compozy

Length of output: 575

---

🏁 Script executed:

```shell
# Verify if there are any other goroutine cleanup patterns in test file
rg "runtime\." pkg/compozy/events/bus_test.go
```

Repository: compozy/compozy

Length of output: 153

---

🏁 Script executed:

```shell
# Check if there are alternative leak detection patterns elsewhere in codebase
rg "runtime\.NumGoroutine|t\.TempDir\|testing leak" --type go | head -20
```

Repository: compozy/compozy

Length of output: 508

---

🏁 Script executed:

```shell
# Examine the full Close implementation to confirm it's synchronous
sed -n '98,130p' pkg/compozy/events/bus.go
```

Repository: compozy/compozy

Length of output: 806

---



</details>

**Replace `runtime.NumGoroutine()` leak detection with direct bus-state verification.**

Since `Bus` is synchronous and spawns no background goroutines, the leak check should verify that the subscriptions map is empty and channels are properly closed after `Close()`, rather than relying on a process-wide goroutine count. The latter is fragile across different test runners and CI environments.

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@pkg/compozy/events/bus_test.go` around lines 128 - 170, Replace the global
goroutine-leak check using runtime.NumGoroutine() with direct verification of
the Bus internal state after Close(): call New[int] and Subscribe() as before,
perform the concurrent Publish and unsubscribes, then after bus.Close(ctx)
assert that the Bus's subscriptions map is empty and that any subscriber
channels are closed; specifically inspect the Bus instance returned by New[int]
(e.g., the subscriptions/subscribers map or slice field used by the
implementation) to ensure len(bus.subscriptions) == 0 and iterate any remaining
entries to confirm receiving from those channels yields the closed condition
(channel reads return zero value and false), failing the test if the map is
non-empty or any channel remains open.
```

</details>

<!-- fingerprinting:phantom:medusa:grasshopper:ac49757c-9624-4c4c-9e22-24ca46b4b75e -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Resolve

Thread ID: `PRRT_kwDORy7nkc55C9um`

```bash
gh api graphql -f query='mutation($id:ID!){resolveReviewThread(input:{threadId:$id}){thread{isResolved}}}' -F id=PRRT_kwDORy7nkc55C9um
```

---
*Generated from PR review - CodeRabbit AI*
