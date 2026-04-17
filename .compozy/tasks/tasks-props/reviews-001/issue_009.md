---
status: resolved
file: internal/core/run/executor/runner.go
line: 208
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc57ypzK,comment:PRRC_kwDORy7nkc644Msx
---

# Issue 009: _⚠️ Potential issue_ | _🟠 Major_
## Review Comment

_⚠️ Potential issue_ | _🟠 Major_

**Whitespace-only runtime mutations still bypass the guard.**

A hook can change `"codex"` to `"codex "` and still pass `jobRuntimeChanged`, because the comparison trims both sides. `applyHookModelJob` then copies the raw value back into `r.job`, so the runtime *has* changed after planning and downstream ACP resolution will see the mutated string.

<details>
<summary>Suggested fix</summary>

```diff
-import (
-	"context"
-	"errors"
-	"fmt"
-	"os"
-	"strings"
-	"time"
-)
+import (
+	"context"
+	"errors"
+	"fmt"
+	"os"
+	"time"
+)
...
 func jobRuntimeChanged(before model.Job, after model.Job) bool {
-	return strings.TrimSpace(before.IDE) != strings.TrimSpace(after.IDE) ||
-		strings.TrimSpace(before.Model) != strings.TrimSpace(after.Model) ||
-		strings.TrimSpace(before.ReasoningEffort) != strings.TrimSpace(after.ReasoningEffort)
+	return before.IDE != after.IDE ||
+		before.Model != after.Model ||
+		before.ReasoningEffort != after.ReasoningEffort
 }
```
</details>



Also applies to: 272-275

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/core/run/executor/runner.go` around lines 205 - 208,
jobRuntimeChanged currently trims runtimes so a hook can make whitespace-only
changes (e.g., "codex" -> "codex ") and bypass the guard before
applyHookModelJob writes the raw value; update jobRuntimeChanged to treat
whitespace-only edits as a runtime mutation by comparing both the trimmed values
and the raw values (or explicitly checking if rawBefore != rawAfter even when
trimmedBefore == trimmedAfter) and return true for any difference, then keep
applyHookModelJob as-is; make the same change for the other occurrence
referenced (lines 272-275) so whitespace-only changes cannot slip through.
```

</details>

<!-- fingerprinting:phantom:medusa:grasshopper:7bb75d9d-dbd3-41c6-89da-03415801a6e9 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `valid`
- Notes:
  - Confirmed by inspection. `jobRuntimeChanged` compares trimmed runtime fields, but `applyHookModelJob` writes the raw mutated strings back to the job.
  - Root cause: whitespace-only mutations collapse to equality in the guard, allowing hooks to change the planned runtime payload after the planning phase.
  - Intended fix: treat any raw runtime string difference as a mutation so whitespace-only edits are rejected alongside substantive runtime changes.
  - Resolution: the runtime guard now compares raw runtime strings, and a minimal executor regression test was added in `internal/core/run/executor/runtime_guard_test.go` for whitespace-only hook mutations.
