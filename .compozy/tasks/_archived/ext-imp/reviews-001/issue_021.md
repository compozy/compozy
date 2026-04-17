---
status: resolved
file: internal/core/run/exec/exec_test.go
line: 616
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc56T0ic,comment:PRRC_kwDORy7nkc624f7w
---

# Issue 021: _⚠️ Potential issue_ | _🟡 Minor_
## Review Comment

_⚠️ Potential issue_ | _🟡 Minor_

**Guard nil receiver before dereferencing in `DispatchMutableHook`.**

`m.observerPayloads` is accessed before confirming `m != nil`, which can panic if the method is ever invoked on a nil receiver.

<details>
<summary>🛠️ Minimal fix</summary>

```diff
 func (m *execHookManager) DispatchMutableHook(ctx context.Context, hook string, payload any) (any, error) {
+	if m == nil {
+		return payload, nil
+	}
 	if hook == "run.post_shutdown" {
 		if m.observerPayloads == nil {
 			m.observerPayloads = make(map[string][]any)
 		}
 		m.observerPayloads[hook] = append(m.observerPayloads[hook], payload)
 	}
-	if m != nil && m.dispatchMutable != nil {
+	if m.dispatchMutable != nil {
 		return m.dispatchMutable(ctx, hook, payload)
 	}
 	return payload, nil
 }
```
</details>

<!-- suggestion_start -->

<details>
<summary>📝 Committable suggestion</summary>

> ‼️ **IMPORTANT**
> Carefully review the code before committing. Ensure that it accurately replaces the highlighted code, contains no missing lines, and has no issues with indentation. Thoroughly test & benchmark the code to ensure it meets the requirements.

```suggestion
func (m *execHookManager) DispatchMutableHook(ctx context.Context, hook string, payload any) (any, error) {
	if m == nil {
		return payload, nil
	}
	if hook == "run.post_shutdown" {
		if m.observerPayloads == nil {
			m.observerPayloads = make(map[string][]any)
		}
		m.observerPayloads[hook] = append(m.observerPayloads[hook], payload)
	}
	if m.dispatchMutable != nil {
		return m.dispatchMutable(ctx, hook, payload)
	}
	return payload, nil
}
```

</details>

<!-- suggestion_end -->

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/core/run/exec/exec_test.go` around lines 607 - 616, Guard the nil
receiver at the top of DispatchMutableHook before accessing m.observerPayloads:
add an early check "if m == nil { return nil, nil }" (or the appropriate zero
return) so you never dereference m when nil, then proceed to the existing logic
that initializes m.observerPayloads and calls m.dispatchMutable; this ensures
DispatchMutableHook, m.observerPayloads and m.dispatchMutable are only accessed
when m is non-nil.
```

</details>

<!-- fingerprinting:phantom:poseidon:hawk:03d7857f-d529-43ca-be71-43278e85f981 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `valid`
- Notes:
  - Confirmed by inspection. `execHookManager.DispatchMutableHook` dereferences `m.observerPayloads` before checking whether the receiver is nil.
  - Root cause: the nil guard happens too late in the test helper implementation.
  - Intended fix: return early on a nil receiver, then keep the existing observer/mutable dispatch behavior.
  - Resolution: the helper now returns early on a nil receiver before touching observer state.
