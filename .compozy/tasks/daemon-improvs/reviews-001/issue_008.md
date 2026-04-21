---
status: resolved
file: internal/api/contract/types.go
line: 513
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc58go75,comment:PRRC_kwDORy7nkc651UMU
---

# Issue 008: _⚠️ Potential issue_ | _🟠 Major_
## Review Comment

_⚠️ Potential issue_ | _🟠 Major_

**Reject `has_more=true` pages without a cursor.**

`Decode()` currently accepts `has_more=true` with an empty `next_cursor`, which leaves the client unable to fetch the next page and masks a broken server response. This contract should fail fast.

<details>
<summary>Suggested change</summary>

```diff
 func (r RunEventPageResponse) Decode() (RunEventPage, error) {
 	nextCursor, err := ParseCursor(r.NextCursor)
 	if err != nil {
 		return RunEventPage{}, fmt.Errorf("decode events cursor: %w", err)
 	}
+	if r.HasMore && nextCursor.Sequence == 0 {
+		return RunEventPage{}, fmt.Errorf("decode events cursor: missing next_cursor when has_more=true")
+	}
 
 	page := RunEventPage{
 		Events:  r.Events,
 		HasMore: r.HasMore,
 	}
```
</details>

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/api/contract/types.go` around lines 500 - 513, In
RunEventPageResponse.Decode, add a fast-fail when the server declares HasMore
but doesn't provide a NextCursor: if r.HasMore && r.NextCursor == "" return an
error (e.g. fmt.Errorf("decode events: has_more=true but next_cursor missing")).
Keep the ParseCursor call and its error handling for non-empty cursors, and only
set page.NextCursor when ParseCursor succeeds and nextCursor.Sequence > 0;
reference RunEventPageResponse.Decode, RunEventPage, r.HasMore, r.NextCursor,
and ParseCursor.
```

</details>

<!-- fingerprinting:phantom:medusa:grasshopper:0a63c130-a1da-4180-ae3d-657764834efe -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `valid`
- Root cause: `RunEventPageResponse.Decode` treats `has_more=true` with an empty `next_cursor` as a valid page, leaving callers unable to resume pagination while hiding a broken server response.
- Fix approach: fail fast when `has_more=true` but no usable next cursor is present, and add regression coverage for that contract violation.
- Resolution: `RunEventPageResponse.Decode` now rejects `has_more=true` without a decoded `next_cursor`.
- Regression coverage: the contract unit test now covers the missing-cursor failure case, and `go test ./internal/api/client ./internal/api/contract ./internal/api/core ./internal/api/httpapi` passed.
- Verification: `make verify` passed after the final edit with `2534` tests, `2` skipped daemon helper-process tests, and a successful `go build ./cmd/compozy`.
