---
status: resolved
file: internal/cli/extension/install.go
line: 80
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc56RVOv,comment:PRRC_kwDORy7nkc621Vaf
---

# Issue 006: _⚠️ Potential issue_ | _🟠 Major_
## Review Comment

_⚠️ Potential issue_ | _🟠 Major_

**Clean up `installPath` when the copy fails midway.**

If `deps.copyDir` returns after creating some files, the command exits with a partially-installed extension on disk. The next install then hits “already exists”, but the tree is incomplete and no state was recorded.



<details>
<summary>Suggested fix</summary>

```diff
-	if err := deps.copyDir(sourcePath, installPath); err != nil {
-		return fmt.Errorf("copy extension into user scope: %w", err)
-	}
+	if err := deps.copyDir(sourcePath, installPath); err != nil {
+		cleanupErr := deps.removeAll(installPath)
+		if cleanupErr != nil {
+			err = errors.Join(err, fmt.Errorf("cleanup failed at %q: %w", installPath, cleanupErr))
+		}
+		return fmt.Errorf("copy extension into user scope: %w", err)
+	}
```
</details>

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/cli/extension/install.go` around lines 79 - 80, When
deps.copyDir(sourcePath, installPath) fails you must clean up the
partially-created install tree to avoid corrupt installs: before calling
deps.copyDir, check whether installPath already existed (os.Stat) and record
that; if copy returns an error and installPath did not exist beforehand, remove
the installPath directory (os.RemoveAll) to rollback the partial install, then
return the original fmt.Errorf("copy extension into user scope: %w", err);
implement this logic around the existing deps.copyDir call so rollback only
removes what this operation created.
```

</details>

<!-- fingerprinting:phantom:medusa:grasshopper:388c7721-ae35-4ddb-a0e7-78fbd9aa7a58 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `VALID`
- Notes:
  - If `deps.copyDir(sourcePath, installPath)` fails after creating part of the destination tree, the function returns immediately and leaves a partial install behind.
  - Root cause: the install flow has no rollback around the copy step even though `ensureInstallTargetAvailable` already established that this command is creating the destination tree.
  - Implemented fix: recorded whether `installPath` existed before the copy attempt and removed the newly created tree on copy failure when the command created it.
