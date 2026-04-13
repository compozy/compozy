---
status: resolved
file: sdk/extension-sdk-ts/src/testing/test_harness.ts
line: 144
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc56V4Zn,comment:PRRC_kwDORy7nkc627G24
---

# Issue 010: _⚠️ Potential issue_ | _🟠 Major_
## Review Comment

_⚠️ Potential issue_ | _🟠 Major_

**Reject outstanding calls when the harness loop terminates.**

`call()` is public now, but a closed/broken transport still leaves the matching promise unresolved because `hostLoop()` returns without draining `this.pending`. That turns extension crashes into hung tests instead of fast failures.  


<details>
<summary>Possible fix</summary>

```diff
+  private rejectPending(error: unknown): void {
+    for (const [id, pending] of this.pending) {
+      this.pending.delete(id);
+      pending.reject(error);
+    }
+  }
+
   /** Issues one arbitrary request against the running extension. */
   async call<T>(method: string, params: unknown): Promise<T> {
     const id = String(++this.requestID);
     const response = await new Promise<Message>((resolve, reject) => {
       this.pending.set(id, { resolve, reject });
@@
   private async hostLoop(): Promise<void> {
     while (true) {
       let message: Message;
       try {
         message = await this.hostTransport.readMessage();
-      } catch {
+      } catch (error) {
+        this.rejectPending(error);
         return;
       }
```
</details>

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@sdk/extension-sdk-ts/src/testing/test_harness.ts` around lines 129 - 144, The
public call() can hang because pending requests in this.pending are never
rejected when the harness transport/hostLoop() stops; modify hostLoop (or the
transport shutdown path) to, upon termination or on transport error, iterate
this.pending and call each stored reject(...) with a clear Error (e.g. "harness
terminated" or include the underlying error), then clear the map so callers of
call() get fast failures; ensure you still remove entries in call() when
writeMessage fails (hostTransport.writeMessage) and keep
requestID/RPCError.fromShape usage unchanged.
```

</details>

<!-- fingerprinting:phantom:medusa:grasshopper:d73c2a5b-b595-4db2-9fe8-f643a58586e0 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `valid`
- Notes:
  - Root cause: `hostLoop()` returns on transport EOF/error without rejecting the outstanding entries in `this.pending`.
  - Impact: callers of the public `call()` API can hang forever when an extension crashes or closes the transport before replying.
  - Fix approach: reject and clear pending calls on harness termination and add an SDK regression test that simulates a terminated extension during a pending request.
  - Implemented: added pending-call rejection on harness termination in `test_harness.ts` and covered the behavior in `sdk/extension-sdk-ts/test/host_api.test.ts`.
  - Verification: targeted Vitest coverage passed and the final `make verify` run passed cleanly.
