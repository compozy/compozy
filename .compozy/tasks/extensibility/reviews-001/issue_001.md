---
status: resolved
file: Makefile
line: 86
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc56RVPI,comment:PRRC_kwDORy7nkc621Va-
---

# Issue 001: _⚠️ Potential issue_ | _🟠 Major_
## Review Comment

_⚠️ Potential issue_ | _🟠 Major_

**Gate publishing behind verification and build targets.**

Line 84 allows direct publish without `make verify`, which can release unverified artifacts. Make `publish-extension-sdks` depend on `verify` and `build-extension-sdks`.

<details>
<summary>Suggested patch</summary>

```diff
-publish-extension-sdks:
+publish-extension-sdks: verify build-extension-sdks
 	npm publish --workspace `@compozy/extension-sdk`
 	npm publish --workspace `@compozy/create-extension`
```
</details>

  
Based on learnings: MUST run `make verify` before completing ANY subtask (runs `fmt + lint + test + build`).

<!-- suggestion_start -->

<details>
<summary>📝 Committable suggestion</summary>

> ‼️ **IMPORTANT**
> Carefully review the code before committing. Ensure that it accurately replaces the highlighted code, contains no missing lines, and has no issues with indentation. Thoroughly test & benchmark the code to ensure it meets the requirements.

```suggestion
publish-extension-sdks: verify build-extension-sdks
	npm publish --workspace `@compozy/extension-sdk`
	npm publish --workspace `@compozy/create-extension`
```

</details>

<!-- suggestion_end -->

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@Makefile` around lines 84 - 86, The publish-extension-sdks Makefile target
currently runs npm publish directly; change its rule to depend on the verify and
build-extension-sdks targets so publishing is gated by verification and build
steps — update the publish-extension-sdks target to list verify and
build-extension-sdks as prerequisites and keep the existing npm publish commands
under that target (target name: publish-extension-sdks; dependency names:
verify, build-extension-sdks).
```

</details>

<!-- fingerprinting:phantom:poseidon:hawk:ba2aaa30-7f4d-46c1-9957-425c6870aaec -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `VALID`
- Notes:
  - `publish-extension-sdks` is currently a standalone target with no prerequisites, so it can publish artifacts without first running the repository verification gate or rebuilding the SDK workspaces.
  - Root cause: the target omits both `verify` and `build-extension-sdks`, which makes the release path less strict than the repository policy requires.
  - Implemented fix: added `verify` and `build-extension-sdks` as prerequisites for `publish-extension-sdks`.
