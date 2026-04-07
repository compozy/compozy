# Issue 2 - Review Thread Comment

**File:** `internal/contentblock/engine.go:111`
**Date:** 2026-04-07 11:44:37 America/Sao_Paulo
**Status:** - [x] ADDRESSED

## Technical Disposition

`VALID` - `DecodeBlock` invoked `blockType` unconditionally, so a nil callback would panic. The fix adds an explicit nil guard and a regression test in `internal/contentblock/engine_test.go`.

## Body

_⚠️ Potential issue_ | _🟠 Major_

<details>
<summary>🧩 Analysis chain</summary>

🏁 Script executed:

```shell
fd -t f "engine.go" | grep contentblock
```

Repository: compozy/compozy

Length of output: 90

---

🏁 Script executed:

```shell
cat -n internal/contentblock/engine.go | sed -n '95,120p'
```

Repository: compozy/compozy

Length of output: 810

---

🏁 Script executed:

```shell
ast-grep --pattern $'func DecodeBlock[$$$]($$$) ($$$) {
  $$$
}'
```

Repository: compozy/compozy

Length of output: 1340

---

🏁 Script executed:

```shell
rg "DecodeBlock" internal/contentblock/ -A 3 -B 1
```

Repository: compozy/compozy

Length of output: 426

---

🏁 Script executed:

```shell
rg "DecodeBlock" --type go -A 5
```

Repository: compozy/compozy

Length of output: 1230

---

🏁 Script executed:

```shell
rg "DecodeBlock" internal/core/model/content.go -B 15 -A 8
```

Repository: compozy/compozy

Length of output: 751

---

🏁 Script executed:

```shell
rg "DecodeBlock" pkg/compozy/events/kinds/content_block.go -B 15 -A 8
```

Repository: compozy/compozy

Length of output: 744

---



</details>

**Add a nil guard for `blockType` callback to prevent runtime panic.**

Line 109 dereferences `blockType` unconditionally. In contrast, the `normalize` parameter at line 114 includes a nil check. For consistency and to align with the coding guideline against panic in production paths, `blockType` should also be guarded:

<details>
<summary>Suggested fix</summary>

```diff
 func DecodeBlock[T any, B ~string](
 	data []byte,
 	expected B,
 	blockType func(T) B,
 	normalize func(*T, B),
 ) (T, error) {
 	var block T
+	if blockType == nil {
+		var zero T
+		return zero, fmt.Errorf("decode %s block: missing type extractor", expected)
+	}
 	if err := json.Unmarshal(data, &block); err != nil {
 		var zero T
 		return zero, fmt.Errorf("decode %s block: %w", expected, err)
 	}
```

</details>

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/contentblock/engine.go` around lines 100 - 111, The code calls the
blockType callback without checking for nil (unlike normalize); add a nil guard
before invoking blockType in the decode function: verify blockType != nil and
return a descriptive error (e.g., "decode <expected> block: missing blockType
callback") if it's nil, then proceed to call blockType(block) and compare to
expected; reference the blockType and normalize parameters and the existing
error-return pattern used after json.Unmarshal to keep behavior consistent.
```

</details>

<!-- fingerprinting:phantom:poseidon:hawk:8cb42230-60c5-42d8-8c7f-6e72b85fe631 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Resolve

Thread ID: `PRRT_kwDORy7nkc55T4LU`

```bash
gh api graphql -f query='mutation($id:ID!){resolveReviewThread(input:{threadId:$id}){thread{isResolved}}}' -F id=PRRT_kwDORy7nkc55T4LU
```

---
*Generated from PR review - CodeRabbit AI*
