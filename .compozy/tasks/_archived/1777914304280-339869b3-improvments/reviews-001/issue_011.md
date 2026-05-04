---
status: resolved
file: internal/api/httpapi/browser_middleware_test.go
line: 78
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc5-QUlB,comment:PRRC_kwDORy7nkc68K-P1
---

# Issue 011: _⚠️ Potential issue_ | _🟠 Major_
## Review Comment

_⚠️ Potential issue_ | _🟠 Major_

**Strengthen error assertions beyond `wantErr` boolean checks.**

For failure cases, assert expected error content/type so the test fails only for the right reason, not any error.

<details>
<summary>♻️ Suggested refactor</summary>

```diff
 	testCases := []struct {
 		name    string
 		rootDir string
 		wantErr bool
+		errContains string
 	}{
-		{name: "valid directory", rootDir: validRoot, wantErr: false},
-		{name: "empty root", rootDir: " ", wantErr: true},
-		{name: "missing root", rootDir: filepath.Join(t.TempDir(), "missing"), wantErr: true},
-		{name: "file root", rootDir: fileRoot, wantErr: true},
+		{name: "Should accept valid directory", rootDir: validRoot, wantErr: false},
+		{name: "Should reject empty root", rootDir: " ", wantErr: true, errContains: "root"},
+		{name: "Should reject missing root", rootDir: filepath.Join(t.TempDir(), "missing"), wantErr: true, errContains: "no such file"},
+		{name: "Should reject file root", rootDir: fileRoot, wantErr: true, errContains: "directory"},
 	}
@@
 			err := validateWorkspaceRoot(tc.rootDir)
 			if tc.wantErr && err == nil {
 				t.Fatal("validateWorkspaceRoot() error = nil, want error")
 			}
 			if !tc.wantErr && err != nil {
 				t.Fatalf("validateWorkspaceRoot() error = %v, want nil", err)
 			}
+			if tc.wantErr && tc.errContains != "" && !strings.Contains(err.Error(), tc.errContains) {
+				t.Fatalf("validateWorkspaceRoot() error = %q, want substring %q", err.Error(), tc.errContains)
+			}
 		})
 	}
```
</details>

  
As per coding guidelines, `**/*_test.go`: MUST have specific error assertions (ErrorContains, ErrorAs).

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/api/httpapi/browser_middleware_test.go` around lines 73 - 78, The
current test only checks tc.wantErr boolean for validateWorkspaceRoot() which is
too weak; update the failure branch to assert the specific error content or type
using testing helpers (e.g., ErrorContains or ErrorAs) against err and the
expected substring/type from the test case (add an expectedErr string or
expectedErrType to tc if needed), and keep the success branch asserting err ==
nil; reference validateWorkspaceRoot(), tc.wantErr, and err when implementing
the stronger assertions so failures only pass for the intended error.
```

</details>

<!-- fingerprinting:phantom:poseidon:hawk:0aa58313-6e22-4f13-a85f-3db5cd1d7a6e -->

<!-- d98c2f50 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `VALID`
- Notes: The test only checked whether any error occurred. Added expected error substrings for empty, missing, and file roots so failures prove the intended validation path.
