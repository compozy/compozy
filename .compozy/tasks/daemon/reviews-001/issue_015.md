---
status: resolved
file: internal/api/udsapi/server.go
line: 73
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc579mm9,comment:PRRC_kwDORy7nkc65HKYI
---

# Issue 015: _⚠️ Potential issue_ | _🟡 Minor_
## Review Comment

_⚠️ Potential issue_ | _🟡 Minor_

**Don't resolve home paths before options are applied.**

`udsapi.New(WithSocketPath(...))` still fails if `ResolveHomePaths()` cannot run, even though the caller already supplied the socket path. That hidden environment dependency makes explicit configuration less reliable than it needs to be.

<details>
<summary>Suggested fix</summary>

```diff
 func New(opts ...Option) (*Server, error) {
-	paths, err := compozyconfig.ResolveHomePaths()
-	if err != nil {
-		return nil, fmt.Errorf("udsapi: resolve home paths: %w", err)
-	}
-
-	server := &Server{socketPath: paths.SocketPath}
+	server := &Server{}
 	for _, opt := range opts {
 		if opt != nil {
 			opt(server)
 		}
 	}
+	if strings.TrimSpace(server.socketPath) == "" {
+		paths, err := compozyconfig.ResolveHomePaths()
+		if err != nil {
+			return nil, fmt.Errorf("udsapi: resolve home paths: %w", err)
+		}
+		server.socketPath = paths.SocketPath
+	}
 	if err := server.finalize(); err != nil {
 		return nil, err
 	}
 	return server, nil
 }
```
</details>

<!-- suggestion_start -->

<details>
<summary>📝 Committable suggestion</summary>

> ‼️ **IMPORTANT**
> Carefully review the code before committing. Ensure that it accurately replaces the highlighted code, contains no missing lines, and has no issues with indentation. Thoroughly test & benchmark the code to ensure it meets the requirements.

```suggestion
func New(opts ...Option) (*Server, error) {
	server := &Server{}
	for _, opt := range opts {
		if opt != nil {
			opt(server)
		}
	}
	if strings.TrimSpace(server.socketPath) == "" {
		paths, err := compozyconfig.ResolveHomePaths()
		if err != nil {
			return nil, fmt.Errorf("udsapi: resolve home paths: %w", err)
		}
		server.socketPath = paths.SocketPath
	}
	if err := server.finalize(); err != nil {
		return nil, err
	}
	return server, nil
}
```

</details>

<!-- suggestion_end -->

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/api/udsapi/server.go` around lines 67 - 73, The constructor New
currently calls compozyconfig.ResolveHomePaths() before applying provided
options, causing New(WithSocketPath(...)) to fail when ResolveHomePaths()
errors; change New to first instantiate Server (zero-value), apply all Option
functions (e.g., WithSocketPath) to that server, and only call
ResolveHomePaths() as a fallback when server.socketPath is still empty; update
references in New, Option handling, and Server.socketPath accordingly so
explicit options override the home-path resolution.
```

</details>

<!-- fingerprinting:phantom:medusa:grasshopper:4748c55f-38a6-4940-81c9-cabca13fdd92 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `VALID`
- Root cause: `udsapi.New` resolves the default home-scoped socket path before applying options, which makes an explicit `WithSocketPath(...)` call depend on ambient home-resolution state.
- Fix plan: apply options first and only resolve the default home socket as a fallback when no explicit socket path was provided. Add regression coverage that verifies explicit socket-path precedence.
- Resolution: Implemented and verified with `make verify`.
