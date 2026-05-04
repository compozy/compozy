---
provider: coderabbit
pr: "133"
round: 2
round_created_at: 2026-04-30T21:47:34.803875Z
status: resolved
file: internal/daemon/review_watch_git_test.go
line: 119
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc5-3Yfo,comment:PRRC_kwDORy7nkc69AEIA
---

# Issue 008: _⚠️ Potential issue_ | _🟡 Minor_ | _⚡ Quick win_
## Review Comment

_⚠️ Potential issue_ | _🟡 Minor_ | _⚡ Quick win_

**Guard integration-like git runner checks when `git` is unavailable**

This test directly invokes the system `git` binary and can fail in minimal CI environments for infrastructure reasons. Consider skipping when `git` is not on `PATH` to keep failures signal-rich.

 

<details>
<summary>Suggested guard</summary>

```diff
 import (
 	"context"
 	"errors"
+	"os/exec"
 	"reflect"
 	"strings"
 	"testing"
 )
 ...
 func TestReviewWatchGitCommandRunnerAndParsers(t *testing.T) {
 	t.Parallel()

+	if _, err := exec.LookPath("git"); err != nil {
+		t.Skip("git binary not available in PATH")
+	}
+
 	git := newExecReviewWatchGit()
```
</details>

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/daemon/review_watch_git_test.go` around lines 109 - 119, The test
calls the system git binary via newExecReviewWatchGit() and git.run(...) which
can fail in minimal CI; add a guard at the start of the test to detect whether
"git" exists on PATH (e.g. using exec.LookPath or equivalent) and call
t.Skipf(...) if not found, then proceed with the existing assertions; ensure the
check runs before creating newExecReviewWatchGit() and invoking git.run so the
test is skipped rather than failing when git is unavailable.
```

</details>

<!-- fingerprinting:phantom:poseidon:hawk:a02def14-ace0-4527-9531-2aec99eb5414 -->

<!-- d98c2f50 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `UNREVIEWED`
- Notes:
