---
status: resolved
file: internal/core/agent/registry_launch.go
line: 324
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc55LZV7,comment:PRRC_kwDORy7nkc61XmQe
---

# Issue 006: _⚠️ Potential issue_ | _🟠 Major_
## Review Comment

_⚠️ Potential issue_ | _🟠 Major_

<details>
<summary>🧩 Analysis chain</summary>

🏁 Script executed:

```shell
# Read the file to confirm the code at lines 306-324
head -n 330 internal/core/agent/registry_launch.go | tail -n +300
```

Repository: compozy/compozy

Length of output: 607

---

🏁 Script executed:

```shell
# Check Go version requirement
cat go.mod | head -n 5
```

Repository: compozy/compozy

Length of output: 114

---

🏁 Script executed:

```shell
# Search for AvailabilityError definition and usage
rg "type AvailabilityError" -A 15
```

Repository: compozy/compozy

Length of output: 976

---

🏁 Script executed:

```shell
# Search for where joinAvailabilityErrors is called
rg "joinAvailabilityErrors" -B 3 -A 3
```

Repository: compozy/compozy

Length of output: 903

---

🏁 Script executed:

```shell
# Check if errors.Join is used elsewhere in the codebase
rg "errors\.Join" --type go
```

Repository: compozy/compozy

Length of output: 597

---



</details>

**Replace error flattening with error chaining to preserve `AvailabilityError` details.**

The current implementation converts all errors to plain text via `strings.Join()`, which destroys the type information of `AvailabilityError` instances. This prevents callers from using `errors.As()` to recover install hints, docs URLs, and other structured fields once fallback launchers are involved, violating the guideline to use `errors.As()` for error matching rather than comparing error strings.

<details>
<summary>Suggested fix</summary>

```diff
-	parts := make([]string, 0, len(errs))
-	for _, err := range errs {
-		parts = append(parts, err.Error())
-	}
-	return errors.New(strings.Join(parts, " | "))
+	return errors.Join(errs...)
```
</details>

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/core/agent/registry_launch.go` around lines 306 - 324,
joinAvailabilityErrors currently flattens multiple errors into a single string
losing typed AvailabilityError details; update the function
(joinAvailabilityErrors) to preserve error types by returning a chained/joined
error instead of strings.Join — e.g., when len(errs) > 1 return
errors.Join(errs...) (or wrap with fmt.Errorf("%w", err) per project style) so
callers can use errors.As to recover AvailabilityError fields; keep the existing
zero-case that returns an AvailabilityError constructed from Spec and the
single-error case returning errs[0].
```

</details>

<!-- fingerprinting:phantom:medusa:grasshopper:6c29911c-ba13-4d74-ad6e-790b2357b234 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `valid`
- Notes:
  `joinAvailabilityErrors` flattens multiple typed errors into one string error, which discards `AvailabilityError` structure and breaks `errors.As` for fallback-launcher failures. The fix is to preserve typed causes by joining the original errors instead of concatenating their messages.
  Resolution: implemented and verified with the focused Go test run plus `make verify`.
