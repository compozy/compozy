# Issue 7 - Review Thread Comment

**File:** `internal/setup/types.go:307`
**Date:** 2026-04-13 18:25:17 UTC
**Status:** - [ ] UNRESOLVED

## Body

_⚠️ Potential issue_ | _🟡 Minor_

**Either sort these results or drop the “sorted order” contract.**

`MissingReusableAgentNames` and `DriftedReusableAgentNames` currently preserve `r.Agents` order; they never sort. As written, the API comment is false and callers can rely on ordering that is not actually guaranteed.

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/setup/types.go` around lines 283 - 307, The comments for
MissingReusableAgentNames and DriftedReusableAgentNames claim results are sorted
but the functions never sort; update both functions to sort the names slice
before returning (e.g., collect names from ReusableAgentVerifyResult's r.Agents
as done now, then call sort.Strings(names) before return) and add the "sort"
import, or alternatively remove/adjust the "sorted order" wording in the comment
if you prefer to not change behavior; reference the functions
MissingReusableAgentNames, DriftedReusableAgentNames and the
ReusableAgentVerifyResult->r.Agents collection when making the change.
```

</details>

<!-- fingerprinting:phantom:medusa:grasshopper:4cdfae9c-22b0-4501-8a36-0aa965d55bf2 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Disposition: VALID
- Resolution: both reusable-agent helper methods now sort their result slices before returning, and the ordering is covered by a regression test.

## Resolve

Thread ID: `PRRT_kwDORy7nkc56nVMs`

```bash
gh api graphql -f query='mutation($id:ID!){resolveReviewThread(input:{threadId:$id}){thread{isResolved}}}' -F id=PRRT_kwDORy7nkc56nVMs
```

---

_Generated from PR review - CodeRabbit AI_
