# Issue 3 - Review Thread Comment

**File:** `internal/core/run/execution.go:172`
**Date:** 2026-04-05 16:33:47 America/Sao_Paulo
**Status:** - [x] RESOLVED

**Disposition:** VALID

**Rationale:** These shutdown fallback writes currently bypass the UI path because they only check `humanOutputEnabled()`. The UI already receives structured shutdown updates, so the raw `stderr` lines should stay disabled while `uiCh` is active, and the completion wording should not imply a graceful-only path after force escalation.

## Body

_⚠️ Potential issue_ | _🟡 Minor_

**Suppress controller shutdown fallback writes while the UI is active.**

These branches only gate on `humanOutputEnabled()`, so interactive runs still write directly to `stderr` during drain/force and before UI shutdown completes. `publishShutdownStatus()` already sends that state through the UI, and the message on Lines 169-170 is also misleading once `beginForce()` has run.

<details>
<summary>Suggested guard</summary>

```diff
- if c.execCtx.cfg.humanOutputEnabled() {
+ if c.execCtx.cfg.humanOutputEnabled() && c.execCtx.uiCh == nil {
```

Apply the same guard to the drain/force/completion fallback writes, and make the completion text neutral so it stays correct after forced shutdown.
</details>


Also applies to: 208-215, 239-241

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/core/run/execution.go` around lines 166 - 172, The shutdown fallback
writes currently only check humanOutputEnabled() and therefore still print to
stderr during interactive UI runs; update the drain/force/completion fallback
branches in execution.go to first check c.execCtx.cfg.humanOutputEnabled()
before writing to os.Stderr (apply same guard used around the existing UI path),
remove or neutralize wording that implies a graceful-only shutdown (change the
completion message in the final branch to a neutral statement like "Controller
shutdown complete" so it remains accurate after beginForce()), and ensure that
publishShutdownStatus() remains the primary mechanism for UI state updates;
apply these same guarded changes to the other fallback write sites corresponding
to the ranges around the existing branches (the ones referenced near lines
208-215 and 239-241).
```

</details>

<!-- fingerprinting:phantom:medusa:grasshopper:c7674e05-2b49-4826-9def-00f247e76749 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Resolve

Thread ID: `PRRT_kwDORy7nkc547MWJ`

```bash
gh api graphql -f query='mutation($id:ID!){resolveReviewThread(input:{threadId:$id}){thread{isResolved}}}' -F id=PRRT_kwDORy7nkc547MWJ
```

---
*Generated from PR review - CodeRabbit AI*
