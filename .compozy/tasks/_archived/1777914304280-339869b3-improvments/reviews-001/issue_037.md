---
status: resolved
file: internal/daemon/workspace_events.go
line: 41
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc5-QUlv,comment:PRRC_kwDORy7nkc68K-Qx
---

# Issue 037: _⚠️ Potential issue_ | _🟠 Major_
## Review Comment

_⚠️ Potential issue_ | _🟠 Major_

<details>
<summary>🧩 Analysis chain</summary>

🏁 Script executed:

```shell
fd workspace_events.go
```

Repository: compozy/compozy

Length of output: 95

---

🏁 Script executed:

```shell
cat -n internal/daemon/workspace_events.go
```

Repository: compozy/compozy

Length of output: 7457

---

🏁 Script executed:

```shell
rg -n "func detachContext" --type go
```

Repository: compozy/compozy

Length of output: 153

---

🏁 Script executed:

```shell
rg -n "detachContext" --type go -B 2 -A 5 | head -50
```

Repository: compozy/compozy

Length of output: 3233

---

🏁 Script executed:

```shell
sed -n '2594,2610p' internal/daemon/run_manager.go
```

Repository: compozy/compozy

Length of output: 436

---

🏁 Script executed:

```shell
rg -n "apicore.WorkspaceEventStream" --type go -B 2 -A 5
```

Repository: compozy/compozy

Length of output: 715

---

🏁 Script executed:

```shell
rg -n "type WorkspaceEventStream" --type go -B 2 -A 10
```

Repository: compozy/compozy

Length of output: 922

---

🏁 Script executed:

```shell
rg -n "var _ " --type go | grep -i stream
```

Repository: compozy/compozy

Length of output: 161

---

🏁 Script executed:

```shell
cat -n internal/api/core/interfaces.go | sed -n '123,135p'
```

Repository: compozy/compozy

Length of output: 545

---

🏁 Script executed:

```shell
rg -n "publishWorkspaceEvent\|streamWorkspaceEvents" internal/daemon/workspace_events.go -A 3
```

Repository: compozy/compozy

Length of output: 41

---

🏁 Script executed:

```shell
rg -n "OpenWorkspaceStream\|workspaceEventStream" --type go -B 2 -A 2 | head -50
```

Repository: compozy/compozy

Length of output: 41

---

🏁 Script executed:

```shell
rg -n "context.WithoutCancel" --type go -B 3 -A 3
```

Repository: compozy/compozy

Length of output: 4847

---

🏁 Script executed:

```shell
rg -n "WithoutCancel" --type go -B 5 -A 5 | grep -A 10 -B 10 "stream\|Stream\|subscriber\|Subscriber" | head -60
```

Repository: compozy/compozy

Length of output: 3965

---

🏁 Script executed:

```shell
rg -n "Close\(\)" internal/daemon/workspace_events.go -B 3 -A 3
```

Repository: compozy/compozy

Length of output: 199

---

🏁 Script executed:

```shell
rg -n "var _.*workspaceEventStream" --type go
```

Repository: compozy/compozy

Length of output: 41

---

🏁 Script executed:

```shell
rg -n "resolveWorkspaceReference" --type go -A 10 | head -30
```

Repository: compozy/compozy

Length of output: 2545

---

🏁 Script executed:

```shell
rg -n "OpenWorkspaceStream" --type go -B 3 -A 8
```

Repository: compozy/compozy

Length of output: 4503

---

🏁 Script executed:

```shell
rg -n "\.Close\(\)" internal/daemon --type go -B 2 -A 1 | grep -A 3 -B 3 "stream\|Stream" | head -40
```

Repository: compozy/compozy

Length of output: 1510

---

🏁 Script executed:

```shell
rg -n "OpenWorkspaceStream\|streamWorkspaceEvents" internal/daemon/run_manager_test.go -B 2 -A 15 | head -80
```

Repository: compozy/compozy

Length of output: 41

---

🏁 Script executed:

```shell
cat -n internal/daemon/run_manager_test.go | sed -n '590,670p'
```

Repository: compozy/compozy

Length of output: 3095

---

🏁 Script executed:

```shell
cat -n internal/api/core/handlers.go | sed -n '1130,1200p'
```

Repository: compozy/compozy

Length of output: 2796

---

🏁 Script executed:

```shell
rg -n "openWorkspaceSocket\|acceptWorkspaceSocket" internal/api/core/handlers.go -A 20 | head -60
```

Repository: compozy/compozy

Length of output: 41

---

🏁 Script executed:

```shell
cat -n internal/api/core/handlers.go | sed -n '1100,1230p'
```

Repository: compozy/compozy

Length of output: 5207

---

🏁 Script executed:

```shell
cat -n internal/api/core/handlers.go | sed -n '1110,1127p'
```

Repository: compozy/compozy

Length of output: 823

---

🏁 Script executed:

```shell
rg -n "OpenWorkspaceStream" --type go
```

Repository: compozy/compozy

Length of output: 686

---

🏁 Script executed:

```shell
rg -n "OpenWorkspaceStream" --type go -B 5 -A 10 | grep -v "^--$"
```

Repository: compozy/compozy

Length of output: 6010

---



</details>

**Detach only the workspace lookup context, not the stream context.**

The stream context is detached from the caller's request context at line 34, so client disconnects no longer stop the subscription goroutine. Per coding guidelines, "Every goroutine must have explicit ownership and shutdown via `context.Context` cancellation." If the transport fails to call `Close()` on abnormal shutdown, the goroutine leaks until process exit.

<details>
<summary>💡 Proposed fix</summary>

```diff
-	streamCtx, cancel := context.WithCancel(detachContext(ctx))
+	streamCtx, cancel := context.WithCancel(ctx)
```
</details>

Also add compile-time interface verification: `var _ apicore.WorkspaceEventStream = (*workspaceEventStream)(nil)` at the end of the file.

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/daemon/workspace_events.go` around lines 25 - 41, The code detaches
the entire stream context causing the subscription goroutine to outlive client
disconnects; instead only detach the workspace lookup call: call
resolveWorkspaceReference(detachContext(ctx), ...) but create streamCtx, cancel
:= context.WithCancel(ctx) (i.e., do not use detachContext for the stream) so
m.streamWorkspaceEvents is cancelled when the caller's ctx is cancelled; ensure
stream.close still calls cancel(); finally add a compile-time interface
assertion var _ apicore.WorkspaceEventStream = (*workspaceEventStream)(nil) at
the end of the file to verify the implementation.
```

</details>

<!-- fingerprinting:phantom:medusa:grasshopper:4021176c-ee24-47e4-a6f3-a97e632a8084 -->

<!-- 4e71b3a2 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `valid`
- Notes: Confirmed the stream subscription context was detached from the caller, so request cancellation would not stop the goroutine unless `Close` was called. Kept detached context only for workspace lookup and derived the stream lifecycle from the caller context; issue 036's compile-time assertion was added in the same file.
