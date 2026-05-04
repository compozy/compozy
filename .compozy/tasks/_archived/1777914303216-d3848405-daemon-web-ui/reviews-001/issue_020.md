---
status: resolved
file: internal/daemon/watcher_error_test.go
line: 35
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc58gqYY,comment:PRRC_kwDORy7nkc651WIf
---

# Issue 020: _⚠️ Potential issue_ | _🟠 Major_
## Review Comment

_⚠️ Potential issue_ | _🟠 Major_

<details>
<summary>🧩 Analysis chain</summary>

🏁 Script executed:

```shell
#!/bin/bash
# Verify this file uses required subtest pattern and avoids brittle-only error assertions.
set -euo pipefail

target="$(fd -p 'watcher_error_test.go' | head -n1)"
echo "Inspecting: ${target}"

echo "1) Subtest pattern (expect at least one t.Run(\"Should...\") match):"
rg -n 't\.Run\("Should' "${target}" || true

echo "2) Temp dir usage (expect t.TempDir() in tests needing paths):"
rg -n 't\.TempDir\(\)' "${target}" || true

echo "3) Direct err.Error() string matching (should be minimized / complemented):"
rg -n 'err\.Error\(\)|strings\.Contains\(' "${target}" || true
```

Repository: compozy/compozy

Length of output: 432

---



</details>

**Refactor this into `t.Run("Should...")` subtests with stronger error assertions.**

This test packs two scenarios into one flow and uses brittle error-string matching. Split into table-driven subtests (`Should...` names), instantiate a fresh watcher per case, use `t.TempDir()` for `workflowRoot`, and assert the wrapped cause directly in addition to message context.

<details>
<summary>Suggested refactor</summary>

```diff
 func TestWorkflowWatcherHandleBackendErrorRecordsWrappedFailure(t *testing.T) {
-	t.Parallel()
-
-	watcher := &workflowWatcher{
-		workflowRoot: "/tmp/demo-workflow",
-		logger: slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{
-			Level: slog.LevelWarn,
-		})),
-	}
-
-	watcher.handleBackendError(nil)
-	if err := watcher.stopError(); err != nil {
-		t.Fatalf("stopError(after nil) = %v, want nil", err)
-	}
-
-	rootErr := errors.New("backend failed")
-	watcher.handleBackendError(rootErr)
-
-	err := watcher.stopError()
-	if err == nil {
-		t.Fatal("stopError() = nil, want wrapped backend error")
-	}
-	if !strings.Contains(err.Error(), "workflow watcher error") || !strings.Contains(err.Error(), "backend failed") {
-		t.Fatalf("stopError() = %v, want wrapped backend error", err)
-	}
+	t.Parallel()
+
+	cases := []struct {
+		name        string
+		backendErr  error
+		wantNil     bool
+		wantContain []string
+	}{
+		{
+			name:       "Should return nil when backend error is nil",
+			backendErr: nil,
+			wantNil:    true,
+		},
+		{
+			name:        "Should return wrapped backend failure",
+			backendErr:  errors.New("backend failed"),
+			wantContain: []string{"workflow watcher error", "backend failed"},
+		},
+	}
+
+	for _, tc := range cases {
+		tc := tc
+		t.Run(tc.name, func(t *testing.T) {
+			t.Parallel()
+
+			watcher := &workflowWatcher{
+				workflowRoot: t.TempDir(),
+				logger: slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{
+					Level: slog.LevelWarn,
+				})),
+			}
+
+			watcher.handleBackendError(tc.backendErr)
+			err := watcher.stopError()
+
+			if tc.wantNil {
+				if err != nil {
+					t.Fatalf("stopError() = %v, want nil", err)
+				}
+				return
+			}
+
+			if err == nil {
+				t.Fatal("stopError() = nil, want wrapped backend error")
+			}
+			if !errors.Is(err, tc.backendErr) {
+				t.Fatalf("stopError() = %v, want wrapped cause %v", err, tc.backendErr)
+			}
+			for _, want := range tc.wantContain {
+				if !strings.Contains(err.Error(), want) {
+					t.Fatalf("stopError() = %v, missing %q", err, want)
+				}
+			}
+		})
+	}
 }
```
</details>

Per coding guidelines, "`**/*_test.go`: MUST use t.Run("Should...") pattern for ALL test cases", "MUST have specific error assertions (ErrorContains, ErrorAs)", and "Use `t.TempDir()` for filesystem isolation instead of manual temp directory management".

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/daemon/watcher_error_test.go` around lines 11 - 35, Split the single
test into table-driven t.Run subtests named "Should..." and create a fresh
workflowWatcher per subtest (set workflowRoot to t.TempDir() and construct
logger as before); for each case call watcher.handleBackendError(...) and then
assert the stopError result using stronger assertions: use errors.As (or
testing/assertion helper ErrorAs) to verify the wrapped cause is the original
backend error and ErrorContains (or strings.Contains/assert helper) to verify
the outer message contains "workflow watcher error" context; ensure each subtest
is independent and uses t.Parallel() where appropriate.
```

</details>

<!-- fingerprinting:phantom:poseidon:hawk:8e7145e9-26be-4fbe-9fe1-b3f902cc66b1 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `valid`
- Notes:
  - The watcher error test combines nil and failure branches in one flow, uses a hard-coded temp path, and only checks the wrapped error via string matching.
  - Root cause: the test does not isolate scenarios and does not assert the wrapped cause directly.
  - Intended fix: convert it to `Should...` subtests with `t.TempDir()` and direct wrapped-error assertions alongside message-context checks.

## Resolution

- Converted the watcher error coverage to named subtests, replaced the hard-coded temp path with `t.TempDir()`, and asserted wrapped causes directly.
- Verified with `make verify`.
