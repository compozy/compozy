# Issue 1 - Review Thread Comment

**File:** `internal/contentblock/engine.go:87`
**Date:** 2026-04-07 11:44:36 America/Sao_Paulo
**Status:** - [x] ADDRESSED

## Technical Disposition

`VALID` - `UnmarshalEnvelopeJSON` could dereference a nil validator and returned validator failures without type context. The fix adds an explicit nil guard and wraps validator errors with the envelope type. Regression coverage was added in `internal/contentblock/engine_test.go`.

## Body

_⚠️ Potential issue_ | _🟠 Major_

<details>
<summary>🧩 Analysis chain</summary>

🏁 Script executed:

```shell
cd /repo && wc -l internal/contentblock/engine.go
```

Repository: compozy/compozy

Length of output: 114

---

🏁 Script executed:

```shell
cd /repo && sed -n '75,100p' internal/contentblock/engine.go
```

Repository: compozy/compozy

Length of output: 114

---

🏁 Script executed:

```shell
pwd && find . -name "engine.go" -type f | head -5
```

Repository: compozy/compozy

Length of output: 111

---

🏁 Script executed:

```shell
wc -l ./internal/contentblock/engine.go
```

Repository: compozy/compozy

Length of output: 96

---

🏁 Script executed:

```shell
sed -n '75,100p' ./internal/contentblock/engine.go
```

Repository: compozy/compozy

Length of output: 777

---

🏁 Script executed:

```shell
sed -n '70,90p' ./internal/contentblock/engine.go
```

Repository: compozy/compozy

Length of output: 674

---

🏁 Script executed:

```shell
grep -n "UnmarshalEnvelopeJSON" ./internal/contentblock/engine.go
```

Repository: compozy/compozy

Length of output: 200

---

🏁 Script executed:

```shell
sed -n '86,87p' ./internal/contentblock/engine.go
```

Repository: compozy/compozy

Length of output: 141

---

🏁 Script executed:

```shell
sed -n '73,95p' ./internal/contentblock/engine.go | cat -n
```

Repository: compozy/compozy

Length of output: 816

---



</details>

**Add nil check for validate parameter and wrap error with context.**

Line 86 will panic if `validate` is nil. Line 87 loses envelope type context by returning the validator error directly without wrapping.

<details>
<summary>Suggested fix</summary>

```diff
 	if envelope.Type == "" {
 		return Envelope[T]{}, fmt.Errorf("decode content block envelope: missing type")
 	}
+	if validate == nil {
+		return Envelope[T]{}, fmt.Errorf("decode content block envelope: missing validator")
+	}
 	if err := validate(envelope.Type, data); err != nil {
-		return Envelope[T]{}, err
+		return Envelope[T]{}, fmt.Errorf("decode %s block: %w", envelope.Type, err)
 	}
```
</details>

Per coding guidelines: "No `panic()` in production paths" and "Prefer explicit error returns with wrapped context using `fmt.Errorf(\"context: %w\", err)`".

<!-- suggestion_start -->

<details>
<summary>📝 Committable suggestion</summary>

> ‼️ **IMPORTANT**
> Carefully review the code before committing. Ensure that it accurately replaces the highlighted code, contains no missing lines, and has no issues with indentation. Thoroughly test & benchmark the code to ensure it meets the requirements.

```suggestion
	if envelope.Type == "" {
		return Envelope[T]{}, fmt.Errorf("decode content block envelope: missing type")
	}
	if validate == nil {
		return Envelope[T]{}, fmt.Errorf("decode content block envelope: missing validator")
	}
	if err := validate(envelope.Type, data); err != nil {
		return Envelope[T]{}, fmt.Errorf("decode %s block: %w", envelope.Type, err)
	}
```

</details>

<!-- suggestion_end -->

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/contentblock/engine.go` around lines 86 - 87, The code calls
validate(envelope.Type, data) without checking if validate is nil and returns
the validator error without context; add a nil-check for the validate function
pointer and return a clear wrapped error if it's missing (e.g., return
Envelope[T]{}, fmt.Errorf("no validator for type %s", envelope.Type)), and when
validate returns an error wrap it with context including envelope.Type using
fmt.Errorf("validate %s: %w", envelope.Type, err); ensure fmt is imported and
update the call site around validate(...) and the Envelope[T] return
accordingly.
```

</details>

<!-- fingerprinting:phantom:poseidon:hawk:8cb42230-60c5-42d8-8c7f-6e72b85fe631 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Resolve

Thread ID: `PRRT_kwDORy7nkc55T4K_`

```bash
gh api graphql -f query='mutation($id:ID!){resolveReviewThread(input:{threadId:$id}){thread{isResolved}}}' -F id=PRRT_kwDORy7nkc55T4K_
```

---
*Generated from PR review - CodeRabbit AI*
