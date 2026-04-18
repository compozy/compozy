---
status: resolved
file: sdk/create-extension/test/create-extension.test.ts
line: 13
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc56T0il,comment:PRRC_kwDORy7nkc624f76
---

# Issue 026: _⚠️ Potential issue_ | _🟠 Major_
## Review Comment

_⚠️ Potential issue_ | _🟠 Major_

<details>
<summary>🧩 Analysis chain</summary>

🌐 Web query:

`When npm runs a workspace script like `npm test --workspace `@compozy/create-extension``, what is `process.cwd()` inside the test process? Is `path.resolve("sdk/extension-sdk-ts")` resolved relative to the package directory or the monorepo root?`

💡 Result:

When running npm test --workspace `@compozy/create-extension`, process.cwd inside the test process is the directory of the workspace package (e.g., monorepo/packages/create-extension/), not the monorepo root. path.resolve("sdk/extension-sdk-ts") is resolved relative to the package directory (the workspace root).

Citations:

- 1: https://docs.npmjs.com/cli/v9/commands/npm-run-script/?v=true
- 2: https://docs.npmjs.com/cli/v7/using-npm/workspaces/
- 3: https://docs.npmjs.com/cli/v9/using-npm/workspaces/?v=true
- 4: https://docs.npmjs.com/cli/v8/using-npm/scripts/
- 5: https://docs.npmjs.com/cli/v7/commands/npm-run-script/

---

🏁 Script executed:

```shell
# First, find the test file mentioned in the review
find . -type f -name "create-extension.test.ts" 2>/dev/null
```

Repository: compozy/compozy

Length of output: 111

---

🏁 Script executed:

```shell
# Read the test file to verify the content at lines 12-13 and 115-128
cat -n ./sdk/create-extension/test/create-extension.test.ts
```

Repository: compozy/compozy

Length of output: 4960

---



</details>

**Anchor repo-local paths to the test file, not `process.cwd()`.**

These constants only work when Vitest starts at the monorepo root. Under npm workspace execution, the cwd is the package directory instead, making `resolve("sdk/extension-sdk-ts")` and `resolve(".")` point to non-existent paths. The same issue affects `buildLocalPackages()` at line 127.

Use `import.meta.url` to anchor paths. However, note that `resolve()` is also called on line 30 and must either be preserved in imports or that path must also be anchored to the repository root.

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@sdk/create-extension/test/create-extension.test.ts` around lines 12 - 13,
localSDKSpec and localGoSDKReplace are using process.cwd()-based resolve() which
breaks when Vitest runs from a package directory; change their initialization to
compute repo-root-relative paths using import.meta.url (via fileURLToPath +
path.dirname) and build those file: URIs from that anchored repo root so they
always point to sdk/extension-sdk-ts and the repo root respectively, and update
buildLocalPackages() to use the same import.meta.url-anchored root; keep the
existing resolve() usage referenced on line 30 intact for imports or replace
that call with the same anchored path if you also change import resolution.
```

</details>

<!-- fingerprinting:phantom:medusa:grasshopper:c30debaf-c506-49cf-8095-ed74f878d9da -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `valid`
- Notes:
  - Confirmed by inspection. `localSDKSpec`, `localGoSDKReplace`, and `buildLocalPackages()` are anchored to `process.cwd()`, so workspace-scoped npm execution can resolve them relative to `sdk/create-extension/` instead of the repo root.
  - Root cause: the tests assume Vitest always starts at the monorepo root.
  - Intended fix: derive the repository root from `import.meta.url` and use that anchored path for the local SDK spec, Go replace target, and workspace build command.
  - Resolution: the create-extension tests now derive the repo root from `import.meta.url` and use anchored file paths/URIs consistently.
