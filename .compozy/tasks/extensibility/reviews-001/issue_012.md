---
status: resolved
file: internal/core/agent/client.go
line: 514
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc56RVO7,comment:PRRC_kwDORy7nkc621Vas
---

# Issue 012: _🛠️ Refactor suggestion_ | _🟠 Major_
## Review Comment

_🛠️ Refactor suggestion_ | _🟠 Major_

<details>
<summary>🧩 Analysis chain</summary>

🌐 Web query:

`What does context.WithoutCancel do in Go, and when is it preferable to context.Background for long-lived work derived from a request context?`

💡 Result:

context.WithoutCancel(parent Context) returns a derived context that inherits the parent's values but ignores the parent's cancellation signal. The returned context has no Deadline, no Err, a nil Done channel, and context.Cause returns nil. It was added in Go 1.21. It is preferable to context.Background for long-lived work derived from a request context when you need to retain request-scoped values (e.g., for logging, tracing, metrics, auth) from the parent while detaching from its cancellation (e.g., HTTP request timeout). context.Background discards all parent values, while WithoutCancel preserves them without propagating cancellation. Example use case: In an HTTP handler, spawn a goroutine for non-critical async work like sending an email or cleanup: go func { ctx := context.WithoutCancel(r.Context) // Use ctx for work that inherits request values but continues after request ends sendEmail(ctx, ...) } This allows quick response to client (e.g., 202 Accepted) while completing background work with full context info.

Citations:

- 1: https://boldlygo.tech/archive/2025-07-14-context.withoutcancel/
- 2: https://github.com/golang/go/issues/40221
- 3: https://pkg.go.dev/context@go1.21.4
- 4: https://stackoverflow.com/questions/54128834/context-without-cancel-propagation
- 5: https://pkg.go.dev/context
- 6: https://blog.devtrovert.com/p/go-ep6-keep-contexts-going-with-contextwithoutca
- 7: https://go.dev/src/context/context.go?m=text

---

🏁 Script executed:

```shell
# Get the function containing lines 509-514 and its signature
head -n 520 internal/core/agent/client.go | tail -n 30
```

Repository: compozy/compozy

Length of output: 860

---

🏁 Script executed:

```shell
# Get more context - lines 480-530 to understand the function
sed -n '480,530p' internal/core/agent/client.go
```

Repository: compozy/compozy

Length of output: 1534

---

🏁 Script executed:

```shell
# Check the function signature to see available parameters
sed -n '480,515p' internal/core/agent/client.go | grep -A 50 "func"
```

Repository: compozy/compozy

Length of output: 933

---



</details>

**Replace `context.Background()` with a derived context that preserves request-scoped values.**

The agent process is client-owned (stored in `c.process`) and should outlive individual requests. Launching with `context.Background()` discards request-scoped values (logging, tracing, auth context) and violates the coding guideline: "Pass `context.Context` as the first argument to all functions crossing runtime boundaries; avoid `context.Background()` outside `main` and focused tests."

Use `context.WithoutCancel(ctx)` instead to inherit request context values while detaching from the request's cancellation and deadline:

<details>
<summary>Before and after</summary>

```diff
-	process, err := subprocess.Launch(context.Background(), subprocess.LaunchConfig{
+	process, err := subprocess.Launch(context.WithoutCancel(ctx), subprocess.LaunchConfig{
```
</details>

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/core/agent/client.go` around lines 509 - 514, The subprocess is
launched with context.Background(), dropping request-scoped values; change the
call in the subprocess.Launch invocation to use a derived context that preserves
values but is detached from cancellation by using context.WithoutCancel(ctx) (or
equivalent) so the launched process (stored in c.process) inherits
tracing/logging/auth from the request context without being cancelled when the
request ends; update the Launch call to pass the new ctx returned by
context.WithoutCancel and remove context.Background() usage.
```

</details>

<!-- fingerprinting:phantom:medusa:grasshopper:ce4005c2-f225-49f9-bac5-6ac8c129da42 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `VALID`
- Notes:
  - `ensureStarted` launches the long-lived ACP subprocess with `context.Background()`, which drops request-scoped values such as logging/tracing metadata from the caller context.
  - Root cause: the launch path detaches from cancellation by discarding the incoming context entirely instead of preserving its values.
  - Implemented fix: derived the launch context with `context.WithoutCancel(ctx)` via a small helper so the subprocess is detached from request cancellation while still inheriting request-scoped values.
