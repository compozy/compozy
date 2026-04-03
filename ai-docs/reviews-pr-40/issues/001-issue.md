# Issue 1 - Review Thread Comment

**File:** `internal/cli/form.go:23`
**Date:** 2026-04-03 18:11:21 America/Sao_Paulo
**Status:** - [x] RESOLVED

## Body

_⚠️ Potential issue_ | _🟠 Major_

**Prefilled optional values can no longer be cleared.**

Now that the form is seeded from `state`, deleting a preloaded `round`, `model`, `timeout`, `reviews-dir`, or `add-dir` in the UI does not unset it. `applyStringInput`, `applyIntInput`, and `applyStringSliceInput` all ignore empty submissions, so the old workspace default stays in `state`. The form submit path needs to treat empty as an explicit clear once defaults have been materialized.



Also applies to: 61-98

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/cli/form.go` at line 23, The form now seeds inputs from state via
newFormInputsFromState, but applyStringInput, applyIntInput, and
applyStringSliceInput currently ignore empty submissions so users cannot clear
prefilled values; update the form submit path to treat an empty UI submission as
an explicit clear by: in the submit handler (the code that calls
applyStringInput/applyIntInput/applyStringSliceInput) detect when the raw UI
value is empty and call the apply functions with a sentinel/flag or call a new
Clear* path so the target state field is set to its zero/nil value (e.g., unset
round/model strings, zero timeout, empty slice for reviews-dir/add-dir) instead
of leaving the previously materialized default; reference
newFormInputsFromState, applyStringInput, applyIntInput, applyStringSliceInput
when making the change.
```

</details>

<!-- fingerprinting:phantom:medusa:grasshopper:5772648d-4177-42ad-99f4-1954cf786608 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Disposition: `VALID`
- Restated change: interactive form submission must be able to clear optional prefilled values instead of silently retaining workspace defaults/state defaults.
- Evidence: `newFormInputsFromState` seeds optional values, while `applyStringInput`, `applyIntInput`, and `applyStringSliceInput` currently return early on `value == ""`, so deleting a prefilled value cannot propagate a zero value back into `commandState`.
- Implementation note: fix together with Issue 2 so ambiguous `add-dir` prefill does not get cleared accidentally.

## Resolve

Thread ID: `PRRT_kwDORy7nkc54yeIZ`

```bash
gh api graphql -f query='mutation($id:ID!){resolveReviewThread(input:{threadId:$id}){thread{isResolved}}}' -F id=PRRT_kwDORy7nkc54yeIZ
```

---
*Generated from PR review - CodeRabbit AI*
