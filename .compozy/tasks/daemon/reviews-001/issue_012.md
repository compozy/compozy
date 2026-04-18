---
status: resolved
file: internal/api/httpapi/server.go
line: 184
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc579mm6,comment:PRRC_kwDORy7nkc65HKYF
---

# Issue 012: _⚠️ Potential issue_ | _🟠 Major_
## Review Comment

_⚠️ Potential issue_ | _🟠 Major_

**Set `started` before releasing the lock.**

Two concurrent `Start` calls can both pass this guard, bind separate listeners, and then race to overwrite `s.httpServer`/`s.listener`/`s.serveDone`. One server becomes untracked and `Shutdown` only cleans up the last writer.

<details>
<summary>Suggested fix</summary>

```diff
 s.mu.Lock()
 if s.started {
 	s.mu.Unlock()
 	return errors.New("httpapi: server already started")
 }
+s.started = true
 s.mu.Unlock()

 address := net.JoinHostPort(s.host, strconv.Itoa(s.port))
 var listenConfig net.ListenConfig
 ln, err := listenConfig.Listen(ctx, "tcp", address)
 if err != nil {
+	s.mu.Lock()
+	s.started = false
+	s.mu.Unlock()
 	return fmt.Errorf("httpapi: listen on %q: %w", address, err)
 }
```

Apply the same rollback on every error path before the serve goroutine is launched.
</details>

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/api/httpapi/server.go` around lines 179 - 184, The Start method
currently checks s.started while holding s.mu but releases the lock before
setting it, allowing two concurrent Start calls to both proceed; fix by setting
s.started = true while still holding s.mu (i.e., inside the s.mu.Lock() /
Unlock() section) so the second caller immediately sees started; also ensure
that on every subsequent error path (any failure before launching the serve
goroutine that sets s.httpServer/s.listener/s.serveDone) you perform the same
rollback (clear s.started and any partially set fields) while holding s.mu so
the server state remains consistent and Shutdown will clean up correctly;
reference s.mu, s.started, s.httpServer, s.listener, s.serveDone and the serve
goroutine launch when applying the fix.
```

</details>

<!-- fingerprinting:phantom:medusa:grasshopper:4748c55f-38a6-4940-81c9-cabca13fdd92 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `VALID`
- Root cause: `httpapi.Server.Start` checks `s.started` under the lock but does not reserve it until after bind/setup work completes, so concurrent starts can both pass the guard and publish conflicting server state.
- Fix plan: reserve the started state while holding the mutex, roll it back on every pre-serve error path, and add deterministic concurrency coverage using a blocking port-updater hook.
- Resolution: Implemented and verified with `make verify`.
