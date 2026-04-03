---
status: resolved
file: internal/core/run/execution_acp_test.go
line: 70
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc54vHCC,comment:PRRC_kwDORy7nkc60y0gB
---

# Issue 012: _⚠️ Potential issue_ | _🟡 Minor_
## Review Comment

_⚠️ Potential issue_ | _🟡 Minor_

**Using `t.Errorf` inside goroutines may not fail the test reliably.**

When `t.Errorf` is called from a goroutine, the test may complete before the error is reported, or it may cause a race. Consider capturing errors via channels or using `t.Cleanup` to verify state.


<details>
<summary>🔧 Suggested approach</summary>

```diff
 	secondClient := newFakeACPClient(func(_ context.Context, _ agent.SessionRequest) (agent.Session, error) {
 		session := newFakeACPSession("sess-2")
+		var blockErr error
 		go func() {
 			textBlock, err := model.NewContentBlock(model.TextBlock{Text: "retry succeeded"})
 			if err != nil {
-				t.Errorf("new content block: %v", err)
+				blockErr = err
 				return
 			}
 			session.publish(model.SessionUpdate{
 				Blocks: []model.ContentBlock{textBlock},
 				Status: model.StatusRunning,
 			})
 			session.finish(nil)
 		}()
+		t.Cleanup(func() {
+			if blockErr != nil {
+				t.Errorf("new content block: %v", blockErr)
+			}
+		})
 		return session, nil
 	})
```
</details>


Also applies to: 294-298

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/core/run/execution_acp_test.go` around lines 66 - 70, The test is
calling t.Errorf inside a goroutine (around
model.NewContentBlock/model.TextBlock usage), which can miss failures; change
the goroutine to send any error to a buffered channel (e.g., errCh := make(chan
error, 1) or use an errgroup/WaitGroup) instead of calling t.Errorf directly,
then in the main test goroutine receive from that channel and call
t.Fatalf/t.Errorf based on the received error; apply the same pattern to the
other occurrence referenced (lines around 294-298) so all goroutine errors are
reported deterministically from the main test goroutine.
```

</details>

<!-- fingerprinting:phantom:medusa:ocelot:e9a5cb25-70ed-4597-839e-cc9e1e67363a -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `valid`
- Rationale: The two flagged tests call `t.Errorf` from goroutines that outlive the immediate call site. That is nondeterministic test reporting and should be routed back to the main test goroutine.
- Fix plan: Replace the goroutine-local `t.Errorf` calls with buffered error channels and assert the result from the parent goroutine.
- Resolution: Replaced the goroutine-local test reporting with buffered error channels and parent-goroutine assertions in both ACP execution tests. `make verify` passed.
