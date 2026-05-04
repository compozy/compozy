---
status: resolved
file: packages/ui/src/components/alert.tsx
line: 44
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc59RNfj,comment:PRRC_kwDORy7nkc662oQM
---

# Issue 003: _⚠️ Potential issue_ | _🟠 Major_
## Review Comment

_⚠️ Potential issue_ | _🟠 Major_

**Don't make every non-error alert a live region.**

Defaulting to `role="status"` here means even static info/warning banners get announced by screen readers on mount. That should usually be opt-in. A safer default is `alert` for true errors and no implicit live-region role otherwise.


<details>
<summary>Suggested change</summary>

```diff
-  const resolvedRole = role ?? (variant === "error" ? "alert" : "status");
+  const resolvedRole = role ?? (variant === "error" ? "alert" : undefined);
```
</details>

<!-- suggestion_start -->

<details>
<summary>📝 Committable suggestion</summary>

> ‼️ **IMPORTANT**
> Carefully review the code before committing. Ensure that it accurately replaces the highlighted code, contains no missing lines, and has no issues with indentation. Thoroughly test & benchmark the code to ensure it meets the requirements.

```suggestion
  const resolvedRole = role ?? (variant === "error" ? "alert" : undefined);
  return (
    <div className={cn(alertVariants({ variant }), className)} role={resolvedRole} {...props}>
```

</details>

<!-- suggestion_end -->

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@packages/ui/src/components/alert.tsx` around lines 42 - 44, The component
currently sets resolvedRole = role ?? (variant === "error" ? "alert" :
"status"), which makes non-error banners live regions; change this so only error
variant gets a live region: compute resolvedRole = role ?? (variant === "error"
? "alert" : undefined) (or omit the role attribute when undefined) so that
provided role still wins, "error" results in role="alert", and other variants do
not default to role="status".
```

</details>

<!-- fingerprinting:phantom:medusa:grasshopper:fd160fe4-d2d3-4a0b-ad04-a1afdb520e03 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `valid`
- Notes:
  - `Alert` currently defaults non-error variants to `role="status"`, which turns static info/success/warning banners into live regions and causes unnecessary screen-reader announcements on mount.
  - The root cause is the default role resolution, not consumer usage.
  - Fix: keep the implicit live region only for `variant="error"` and add a primitive regression test covering the default role behavior.
