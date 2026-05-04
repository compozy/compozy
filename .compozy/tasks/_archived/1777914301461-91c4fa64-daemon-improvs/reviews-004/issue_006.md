---
status: resolved
file: internal/store/globaldb/registry_test.go
line: 266
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc58kNgB,comment:PRRC_kwDORy7nkc656GNI
---

# Issue 006: _⚠️ Potential issue_ | _🟡 Minor_
## Review Comment

_⚠️ Potential issue_ | _🟡 Minor_

<details>
<summary>🧩 Analysis chain</summary>

🏁 Script executed:

```shell
#!/bin/bash
set -eu
rg -n --hidden --glob '.github/workflows/*.yml' --glob '.github/workflows/*.yaml' 'windows|GOOS|matrix'
```

Repository: compozy/compozy

Length of output: 41

---

🏁 Script executed:

```shell
# Find the implementation of canonicalizeExistingPathCaseWith
rg -n "func canonicalizeExistingPathCaseWith" -A 20
```

Repository: compozy/compozy

Length of output: 1538

---

🏁 Script executed:

```shell
# Check the test file location and examine the context
head -300 internal/store/globaldb/registry_test.go | tail -100
```

Repository: compozy/compozy

Length of output: 3308

---

🏁 Script executed:

```shell
# Look for CI/CD configuration files to understand testing matrix
find . -type f -name "*.yml" -o -name "*.yaml" -o -name "*.toml" -o -name "go.mod" | grep -E "(\.github|workflow|action|\.gitlab|\.circleci|go\.mod)" | head -20
```

Repository: compozy/compozy

Length of output: 454

---

🏁 Script executed:

```shell
# Check if there are any platform-specific test builds or skips
rg -n "GOOS|runtime.GOOS|build.*windows" internal/store/globaldb/
```

Repository: compozy/compozy

Length of output: 41

---

🏁 Script executed:

```shell
# Check CI workflow for Windows testing
cat .github/workflows/ci.yml
```

Repository: compozy/compozy

Length of output: 1787

---

🏁 Script executed:

```shell
# Verify filepath.IsAbs behavior on different platforms with a quick Python check
python3 << 'EOF'
import os
import pathlib

# On Windows, IsAbs behavior
windows_paths = [
    "\\Users\\test",  # Not absolute - no drive letter
    "C:\\Users\\test",  # Absolute
    "\\",  # Not absolute
    "C:\\",  # Absolute
]

print("filepath.IsAbs behavior simulation (Windows):")
for path in windows_paths:
    # Python's pathlib.PureWindowsPath.is_absolute()
    is_abs = pathlib.PureWindowsPath(path).is_absolute()
    print(f"  {path:20} -> is_absolute: {is_abs}")
EOF
```

Repository: compozy/compozy

Length of output: 282

---

🏁 Script executed:

```shell
# Check the exact test to understand what paths are being constructed
sed -n '238,266p' internal/store/globaldb/registry_test.go
```

Repository: compozy/compozy

Length of output: 1018

---



</details>

**Make this fixture absolute on Windows.**

On Windows, `string(filepath.Separator)` produces `\`, and paths like `\Users\...` are not absolute per `filepath.IsAbs()` (they require a drive letter like `C:\`). Since `canonicalizeExistingPathCaseWith` returns early when the path is not absolute (line 926), this test would fail on Windows because the function would return `input` unchanged instead of processing it. Although Windows is not currently in the CI matrix, the code's volume handling suggests Windows support is intended.

Suggested fix:
```diff
-	root := string(filepath.Separator)
+	root := string(filepath.Separator)
+	if volume := filepath.VolumeName(t.TempDir()); volume != "" {
+		root = volume + string(filepath.Separator)
+	}
```

<!-- suggestion_start -->

<details>
<summary>📝 Committable suggestion</summary>

> ‼️ **IMPORTANT**
> Carefully review the code before committing. Ensure that it accurately replaces the highlighted code, contains no missing lines, and has no issues with indentation. Thoroughly test & benchmark the code to ensure it meets the requirements.

```suggestion
	root := string(filepath.Separator)
	if volume := filepath.VolumeName(t.TempDir()); volume != "" {
		root = volume + string(filepath.Separator)
	}
	usersDir := filepath.Join(root, "Users")
	homeDir := filepath.Join(usersDir, "pedronauck")
	devDir := filepath.Join(homeDir, "Dev")
	compozyDir := filepath.Join(devDir, "compozy")
	want := filepath.Join(compozyDir, "agh")
	input := filepath.Join(homeDir, "dev", "compozy", "agh")

	dirs := map[string][]os.DirEntry{
		root:       {fakeDirEntry{name: "Users"}},
		usersDir:   {fakeDirEntry{name: "pedronauck"}},
		homeDir:    {fakeDirEntry{name: "Dev"}},
		devDir:     {fakeDirEntry{name: "compozy"}},
		compozyDir: {fakeDirEntry{name: "agh"}},
	}

	got, err := canonicalizeExistingPathCaseWith(input, func(path string) ([]os.DirEntry, error) {
		entries, ok := dirs[path]
		if !ok {
			return nil, fs.ErrNotExist
		}
		return entries, nil
	})
	if err != nil {
		t.Fatalf("canonicalizeExistingPathCaseWith() error = %v", err)
	}
	if got != want {
		t.Fatalf("canonicalizeExistingPathCaseWith() = %q, want %q", got, want)
	}
```

</details>

<!-- suggestion_end -->

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/store/globaldb/registry_test.go` around lines 238 - 266, The test
fixture uses root := string(filepath.Separator) which yields "\" on Windows and
produces non-absolute paths; update the fixture so the constructed paths are
absolute on Windows by prefixing a drive/volume (use the same drive as the test
input). Specifically, when building root/usersDir/homeDir etc. for
canonicalizeExistingPathCaseWith, set root to the volume + separator (e.g.,
filepath.VolumeName(input) + string(filepath.Separator)) or otherwise ensure
input is absolute via filepath.IsAbs/input->filepath.Abs before calling
canonicalizeExistingPathCaseWith so the function treats the path as absolute and
the test exercises the case-normalization logic.
```

</details>

<!-- fingerprinting:phantom:medusa:grasshopper:3b83986d-d641-4b98-9c1f-3d955d92a465 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `valid`
- Notes:
  - The fixture currently derives `root` from `string(filepath.Separator)`, which becomes `\` on Windows. Paths like `\Users\...` are not absolute there, so `canonicalizeExistingPathCaseWith` returns early and the test never exercises the case-normalization walk.
  - Root cause: the synthetic absolute path fixture is POSIX-only even though the production code explicitly handles Windows volumes.
  - Fix approach: derive the fixture root from the active temp-dir volume when one exists, keeping the simulated path absolute on Windows while preserving the current POSIX behavior.
  - Resolution: the path-case fixture now uses `testAbsoluteRoot(t)`, which prefixes the current volume when present so the simulated input remains absolute on Windows and still exercises the case-normalization walk.
  - Regression coverage: `TestCanonicalizeExistingPathCaseWithUsesOnDiskNames` now uses the portable root fixture, and the new unreadable-parent test reuses the same helper for platform-safe absolute paths.
  - Verification: `go test ./internal/store/globaldb -run 'TestRegistryValidationBranches|TestCanonicalizeExistingPathCaseWithUsesOnDiskNames|TestCanonicalizeExistingPathCaseWithFallsBackToCleanPathWhenParentsCannotBeRead|TestGetByPathPrefersResolvedCanonicalWorkspaceRow' -count=1` passed. `make verify` also passed with `2548` tests, `2` skipped helper-process tests, and a successful `go build ./cmd/compozy`.
