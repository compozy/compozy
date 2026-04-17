---
status: resolved
file: internal/core/extension/review_provider_bridge_integration_test.go
line: 331
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc56T0iU,comment:PRRC_kwDORy7nkc624f7n
---

# Issue 014: _⚠️ Potential issue_ | _🟠 Major_
## Review Comment

_⚠️ Potential issue_ | _🟠 Major_

<details>
<summary>🧩 Analysis chain</summary>

🏁 Script executed:

```shell
find . -name "review_provider_bridge_integration_test.go" -type f
```

Repository: compozy/compozy

Length of output: 127

---

🏁 Script executed:

```shell
head -50 internal/core/extension/review_provider_bridge_integration_test.go
```

Repository: compozy/compozy

Length of output: 1312

---

🏁 Script executed:

```shell
sed -n '309,331p' internal/core/extension/review_provider_bridge_integration_test.go
```

Repository: compozy/compozy

Length of output: 1030

---

🏁 Script executed:

```shell
sed -n '1,30p' internal/core/extension/review_provider_bridge_integration_test.go
```

Repository: compozy/compozy

Length of output: 699

---

🏁 Script executed:

```shell
grep -n "//go:build\|// +build" internal/core/extension/review_provider_bridge_integration_test.go
```

Repository: compozy/compozy

Length of output: 41

---

🏁 Script executed:

```shell
wc -l internal/core/extension/review_provider_bridge_integration_test.go
```

Repository: compozy/compozy

Length of output: 129

---

🏁 Script executed:

```shell
sed -n '26,100p' internal/core/extension/review_provider_bridge_integration_test.go
```

Repository: compozy/compozy

Length of output: 2898

---

🏁 Script executed:

```shell
rg "func Test.*\(" internal/core/extension/review_provider_bridge_integration_test.go
```

Repository: compozy/compozy

Length of output: 581

---

🏁 Script executed:

```shell
grep -n "tsReviewProviderBuildOnce\|sdkReviewExtensionBuildOnce" internal/core/extension/review_provider_bridge_integration_test.go
```

Repository: compozy/compozy

Length of output: 229

---

🏁 Script executed:

```shell
sed -n '120,180p' internal/core/extension/review_provider_bridge_integration_test.go
```

Repository: compozy/compozy

Length of output: 2396

---

🏁 Script executed:

```shell
sed -n '180,240p' internal/core/extension/review_provider_bridge_integration_test.go
```

Repository: compozy/compozy

Length of output: 1563

---

🏁 Script executed:

```shell
sed -n '240,310p' internal/core/extension/review_provider_bridge_integration_test.go
```

Repository: compozy/compozy

Length of output: 1874

---

🏁 Script executed:

```shell
sed -n '280,352p' internal/core/extension/review_provider_bridge_integration_test.go
```

Repository: compozy/compozy

Length of output: 2429

---

🏁 Script executed:

```shell
find . -name "*.yml" -o -name "*.yaml" | head -20
```

Repository: compozy/compozy

Length of output: 538

---

🏁 Script executed:

```shell
rg "TS2688" . 2>/dev/null || echo "No TS2688 error found in codebase"
```

Repository: compozy/compozy

Length of output: 92

---

🏁 Script executed:

```shell
grep -r "integration.*build\|build.*tag" . --include="*.md" 2>/dev/null | head -10
```

Repository: compozy/compozy

Length of output: 1847

---

🏁 Script executed:

```shell
sed -n '1,20p' .github/workflows/ci.yml | head -30
```

Repository: compozy/compozy

Length of output: 322

---

🏁 Script executed:

```shell
rg "review_provider_bridge_integration_test|integration" .github/workflows/ci.yml | head -20
```

Repository: compozy/compozy

Length of output: 41

---

🏁 Script executed:

```shell
sed -n '105,130p' internal/core/extension/review_provider_bridge_integration_test.go
```

Repository: compozy/compozy

Length of output: 883

---



</details>

**Move TypeScript build setup outside the standard test suite or use prebuilt fixtures.**

The `buildTypeScriptReviewProviderEntrypoint` function runs `npm run build`, `npm install`, and `npm run build` unconditionally during test execution without build tags. This makes the Go test suite depend on Node, npm, and external package resolution, breaking hermeticity. Since tests run without `//go:build integration` gates, this affects all developers and CI.

Use either:
- A build tag (`//go:build integration`) to gate the TypeScript tests as opt-in, or
- Check in prebuilt TypeScript extension fixtures instead of compiling at test time.

Additionally, the six test functions (3 Go SDK variants + 3 TypeScript variants) follow an identical pattern and would benefit from a table-driven refactor with `t.Run` subtests to reduce duplication and improve maintainability.

<details>
<summary>🧰 Tools</summary>

<details>
<summary>🪛 GitHub Actions: CI</summary>

[error] 314-314: TestReviewProviderBridgeRunsTypeScriptExtensionOverRealStdIO failed: npm run build --workspace `@compozy/extension-sdk` failed with exit status 2. TypeScript build (tsc -p tsconfig.json) error TS2688: Cannot find type definition file for 'node'.

</details>

</details>

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/core/extension/review_provider_bridge_integration_test.go` around
lines 309 - 331, The test currently invokes Node/npm unconditionally in
buildTypeScriptReviewProviderEntrypoint, making the Go test suite non-hermetic;
either add a build tag like //go:build integration and // +build integration to
gate this TypeScript build (move buildTypeScriptReviewProviderEntrypoint and the
TypeScript-specific tests behind that tag), or replace the runtime build by
checking in prebuilt TypeScript extension fixtures and update
buildTypeScriptReviewProviderEntrypoint to copy those fixtures instead of
running npm; also consolidate the six near-identical test functions into a
single table-driven test using t.Run with cases for each SDK/variant to remove
duplication (reference buildTypeScriptReviewProviderEntrypoint and the six test
functions when applying these changes).
```

</details>

<!-- fingerprinting:phantom:medusa:grasshopper:2b0ec709-3f41-4b6f-b5c8-2840068e728b -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `valid`
- Notes:
  - Root cause: this file builds the TypeScript review-provider fixture differently from the existing TypeScript manager integration flow. It builds the workspace package directly and misses the staged local SDK install that also provisions the TypeScript/node type dependencies, which is why CI can fail with `TS2688`.
  - Fix plan: keep the test in the default suite, but switch it to the same staged local-SDK materialization pattern already used by the TypeScript lifecycle integration test. That fixes the broken fixture setup without hiding the coverage behind an opt-in tag.
  - Resolved: the TypeScript review-provider fixture now stages a persistent local SDK build before installing the template, removing the broken workspace-build path while keeping the test in the default suite; verified with `make verify`.
