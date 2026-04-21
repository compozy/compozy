---
status: resolved
file: web/.storybook/preview.ts
line: 125
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc58gqY2,comment:PRRC_kwDORy7nkc651WJH
---

# Issue 031: _⚠️ Potential issue_ | _🟠 Major_
## Review Comment

_⚠️ Potential issue_ | _🟠 Major_

<details>
<summary>🧩 Analysis chain</summary>

🏁 Script executed:

```shell
#!/bin/bash
# Verify the problematic sibling render in app-mode branch.
rg -n -C3 'routerOptions\?\.kind === "app"|createElement\(Story\)|createElement\(RouterProvider' web/.storybook/preview.ts

# Find stories opting into app router mode (potentially impacted).
rg -n --iglob '*.stories.tsx' 'router:\s*\{\s*kind:\s*"app"'
```

Repository: compozy/compozy

Length of output: 726

---

🏁 Script executed:

```shell
# Find the createStorybookRouter implementation
fd -t f -i 'storybook' | grep -E '\.(ts|tsx|js|jsx)$' | head -20
```

Repository: compozy/compozy

Length of output: 308

---

🏁 Script executed:

```shell
# Search for createStorybookRouter definition
rg -n 'createStorybookRouter' --type ts --type tsx
```

Repository: compozy/compozy

Length of output: 86

---

🏁 Script executed:

```shell
# Check the full context of the preview.ts file around the problematic section
wc -l web/.storybook/preview.ts
```

Repository: compozy/compozy

Length of output: 88

---

🏁 Script executed:

```shell
# Find createStorybookRouter implementation across the codebase
rg -n 'createStorybookRouter' --type ts
```

Repository: compozy/compozy

Length of output: 375

---

🏁 Script executed:

```shell
# Read the full preview.ts file to understand the context
cat -n web/.storybook/preview.ts
```

Repository: compozy/compozy

Length of output: 5484

---

🏁 Script executed:

```shell
# Search for any router configuration in story files
rg -n 'kind.*app' -A 2 -B 2
```

Repository: compozy/compozy

Length of output: 3596

---

🏁 Script executed:

```shell
# Examine the route-story implementation to understand app mode usage
cat -n web/src/storybook/route-story.tsx
```

Repository: compozy/compozy

Length of output: 1934

---

🏁 Script executed:

```shell
# Check the test to understand expected behavior
cat -n web/src/storybook/web-storybook-config.test.tsx | head -100
```

Repository: compozy/compozy

Length of output: 3447

---

🏁 Script executed:

```shell
# Search for tests that verify app mode Fragment behavior specifically
rg -n 'Fragment|routerOptions\?.kind === "app"' web/src/storybook/web-storybook-config.test.tsx -A 5
```

Repository: compozy/compozy

Length of output: 41

---

🏁 Script executed:

```shell
# Check if there are any integration tests for app mode rendering
rg -n 'app.*router|route.*story' --iglob '*.test.tsx' -l | head -5
```

Repository: compozy/compozy

Length of output: 106

---

🏁 Script executed:

```shell
# Verify the stub router still works with just RouterProvider
rg -n 'createStubStorybookRouter\|createRouter' web/.storybook/preview.ts -B 2 -A 5
```

Repository: compozy/compozy

Length of output: 41

---



</details>

**Unnecessary conditional rendering in app mode may cause issues.**

In app mode, `Story` is rendered outside the `RouterProvider` (lines 119-124), meaning it cannot access router context. Additionally, the conditional is redundant since `createStorybookRouter` already handles story wrapping differently for each mode—in stub mode, the Story is wrapped as a route component; in app mode, it's ignored in favor of the real route tree. Unconditionally rendering only `RouterProvider` works for both modes.

<details>
<summary>💡 Suggested fix</summary>

```diff
-      routerOptions?.kind === "app"
-        ? createElement(
-            Fragment,
-            null,
-            createElement(Story),
-            createElement(RouterProvider, { router })
-          )
-        : createElement(RouterProvider, { router })
+      createElement(RouterProvider, { router })
```
</details>

<!-- suggestion_start -->

<details>
<summary>📝 Committable suggestion</summary>

> ‼️ **IMPORTANT**
> Carefully review the code before committing. Ensure that it accurately replaces the highlighted code, contains no missing lines, and has no issues with indentation. Thoroughly test & benchmark the code to ensure it meets the requirements.

```suggestion
      createElement(RouterProvider, { router })
```

</details>

<!-- suggestion_end -->

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@web/.storybook/preview.ts` around lines 118 - 125, The conditional branch
that separately renders Story outside RouterProvider is unnecessary and breaks
router context access; update the render logic so that only RouterProvider with
the router is rendered (remove the routerOptions?.kind === "app" ? ... : ...
branch and the createElement(Fragment, null, createElement(Story), ...) path),
ensuring Story is provided/handled by the router created by
createStorybookRouter so it can access routing context via RouterProvider.
```

</details>

<!-- fingerprinting:phantom:poseidon:hawk:460bc426-8b0d-4b41-9da4-b222d8c078da -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `invalid`
- Notes:
  - The current app-router story architecture intentionally renders `Story` outside `RouterProvider` so setup-only story components like `StorybookWorkspaceSetup` and `StorybookRunStreamSetup` can run their side effects before the real route tree renders.
  - Root cause of the comment: it assumes `Story` itself should be router-context aware in app mode, but for these stories `Story` is used as an out-of-router setup hook while the route tree comes from `createAppStorybookRouter`.
  - Resolution path: keep the existing split render path. Removing `createElement(Story)` would break route stories whose `render` function only performs setup and returns those side-effect helpers.

## Resolution

- Closed as invalid. No code change was made because the existing app-mode Storybook decorator is intentionally structured to run setup helpers outside the router tree, and the current route-story tests pass with that behavior intact.
- Verification:
- `make verify`
