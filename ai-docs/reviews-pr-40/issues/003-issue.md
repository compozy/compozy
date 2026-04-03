# Issue 3 - Review Thread Comment

**File:** `internal/cli/root.go:342`
**Date:** 2026-04-03 18:11:21 America/Sao_Paulo
**Status:** - [x] RESOLVED

## Body

_⚠️ Potential issue_ | _🟡 Minor_

**Wrap the new workspace setup failures with step-specific context.**

All of these branches return the raw error, so the CLI loses whether the failure came from applying workspace defaults or resolving the workspace root for a specific command.

<details>
<summary>🛠️ Suggested fix</summary>

```diff
 	if err := s.applyWorkspaceDefaults(cmd); err != nil {
-		return err
+		return fmt.Errorf("apply workspace defaults for %s: %w", cmd.Name(), err)
 	}
```

```diff
 	if err := s.loadWorkspaceRoot(); err != nil {
-		return err
+		return fmt.Errorf("load workspace root for %s: %w", cmd.Name(), err)
 	}
```

Apply the same wrapping to each `loadWorkspaceRoot()` branch.
</details>
As per coding guidelines, "Use explicit error returns with wrapped context using `fmt.Errorf(\"context: %w\", err)`."


Also applies to: 361-363, 394-396, 434-436, 464-466

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/cli/root.go` around lines 340 - 342, The new workspace setup error
handling returns raw errors and loses step-specific context; update all places
calling s.applyWorkspaceDefaults(cmd) and s.loadWorkspaceRoot(...) (including
the branches around those calls and the other occurrences referenced) to wrap
returned errors with fmt.Errorf("apply workspace defaults: %w", err) or
fmt.Errorf("resolve workspace root for <command>: %w", err) as appropriate so
each failure includes step-specific context; ensure you import fmt if needed and
use the exact function names applyWorkspaceDefaults and loadWorkspaceRoot to
locate the spots to change.
```

</details>

<!-- fingerprinting:phantom:medusa:grasshopper:5772648d-4177-42ad-99f4-1954cf786608 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Disposition: `VALID`
- Restated change: wrap workspace-setup failures with command/step-specific context instead of returning raw errors.
- Evidence: `commandState.run`, `commandState.fetchReviews`, and the migrate/sync/archive command runners currently return raw errors from `applyWorkspaceDefaults` / `loadWorkspaceRoot`, which loses the failing step in CLI output.

## Resolve

Thread ID: `PRRT_kwDORy7nkc54yeIi`

```bash
gh api graphql -f query='mutation($id:ID!){resolveReviewThread(input:{threadId:$id}){thread{isResolved}}}' -F id=PRRT_kwDORy7nkc54yeIi
```

---
*Generated from PR review - CodeRabbit AI*
