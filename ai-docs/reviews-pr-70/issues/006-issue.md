# Issue 6 - Review Thread Comment

**File:** `internal/core/model/preparation_test.go:58`
**Date:** 2026-04-07 11:44:37 America/Sao_Paulo
**Status:** - [x] ADDRESSED

## Technical Disposition

`VALID` - the close-failure test only checked for a non-nil error. It now uses a local sentinel error and `errors.Is` to assert the exact failure while keeping the production behavior unchanged.

## Body

_⚠️ Potential issue_ | _🟠 Major_

**Use a specific error assertion for the close-failure path.**

The assertion currently checks only `err != nil`. Use a sentinel error and `errors.Is(err, sentinel)` so the test verifies the exact failure behavior.

<details>
<summary>Suggested test assertion improvement</summary>

```diff
-	handle := &stubJournalHandle{err: errors.New("close failed")}
+	closeErr := errors.New("close failed")
+	handle := &stubJournalHandle{err: closeErr}
 	prep := &SolvePreparation{JournalHandle: handle}

 	err := prep.CloseJournal(context.Background())
-	if err == nil {
-		t.Fatal("expected close error")
+	if !errors.Is(err, closeErr) {
+		t.Fatalf("expected closeErr, got: %v", err)
 	}
```
</details>

As per coding guidelines, "`**/*_test.go`: Must have specific error assertions (ErrorContains, ErrorAs)".

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/core/model/preparation_test.go` around lines 55 - 58, The test
currently only checks err != nil for prep.CloseJournal; instead define or reuse
a sentinel error (e.g., ErrCloseJournalFailed) that the CloseJournal
implementation returns on the failure path, then update the test to assert the
exact error using errors.Is(err, ErrCloseJournalFailed) (or testing helper
require.ErrorIs/Assert.ErrorIs) against prep.CloseJournal; ensure the sentinel
is exported/visible to the test or placed in a shared test-only package and that
CloseJournal returns that sentinel in the failure scenario.
```

</details>

<!-- fingerprinting:phantom:poseidon:hawk:5863e1b4-cf18-4de0-81c6-bd40921cf292 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Resolve

Thread ID: `PRRT_kwDORy7nkc55T4L8`

```bash
gh api graphql -f query='mutation($id:ID!){resolveReviewThread(input:{threadId:$id}){thread{isResolved}}}' -F id=PRRT_kwDORy7nkc55T4L8
```

---
*Generated from PR review - CodeRabbit AI*
