---
status: resolved
file: internal/core/extension/host_helpers.go
line: 555
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc56RVPH,comment:PRRC_kwDORy7nkc621Va8
---

# Issue 027: _⚠️ Potential issue_ | _🔴 Critical_
## Review Comment

_⚠️ Potential issue_ | _🔴 Critical_

<details>
<summary>🧩 Analysis chain</summary>

🌐 Web query:

`In Go, do filepath.Clean and filepath.Rel prevent symlink traversal outside a root directory, and what is the recommended way to enforce root-confined file access?`

💡 Result:

No, filepath.Clean and filepath.Rel do not prevent symlink traversal outside a root directory. filepath.Clean performs purely lexical processing to simplify paths (removing redundant separators, ., and handling .. where possible), but it does not resolve or check symlinks on the filesystem. filepath.Rel computes a relative path between two paths lexically and also does not follow symlinks or provide confinement guarantees. These functions help with basic path normalization and can detect some lexical traversal attempts (like excessive ..), but they are vulnerable to symlink attacks and TOCTOU races. For example, EvalSymlinks (which does follow symlinks) combined with checks is still racy. The recommended way to enforce root-confined file access is to use the os.Root API introduced in Go 1.24 (available since today's date 2026-04-11). Create a root with os.OpenRoot(rootDir), then use its methods like Open, Create, etc., which prevent escapes via .. or symlinks outside the root using platform-specific mechanisms like openat(2) with RESOLVE_BENEATH on Unix. Alternatively, use the convenience function os.OpenInRoot(dir, name). Example: root, err := os.OpenRoot("/path/to/root") if err != nil { /* handle */ } defer root.Close f, err := root.Open("untrusted/path") // Safe: cannot escape root Prior to Go 1.24, use third-party libraries like github.com/google/safeopen or manual checks with filepath.IsLocal (Go 1.20+) plus EvalSymlinks, but these are less secure due to races. os.Root also provides an fs.FS implementation via Root.FS for io/fs compatibility.

Citations:

- 1: https://go.dev/blog/osroot
- 2: https://pkg.go.dev/path/filepath/
- 3: https://golang.org/blog/osroot
- 4: https://go.dev/issue/67002
- 5: https://go.dev/src/os/root.go
- 6: https://pkg.go.dev/os@go1.25.5

---

🏁 Script executed:

```shell
#!/bin/bash
# Find go.mod and check Go version requirement
find . -name "go.mod" -type f | head -5
```

Repository: compozy/compozy

Length of output: 67

---

🏁 Script executed:

```shell
#!/bin/bash
# Get the file and examine both line ranges
fd "host_helpers.go" --type f
```

Repository: compozy/compozy

Length of output: 98

---

🏁 Script executed:

```shell
#!/bin/bash
# Read the relevant sections of the file
if [ -f "internal/core/extension/host_helpers.go" ]; then
  echo "=== Lines around 528-555 ==="
  sed -n '520,565p' "internal/core/extension/host_helpers.go"
  echo ""
  echo "=== Lines around 665-671 ==="
  sed -n '660,680p' "internal/core/extension/host_helpers.go"
fi
```

Repository: compozy/compozy

Length of output: 1737

---

🏁 Script executed:

```shell
#!/bin/bash
# Search for pathWithinRoot function implementation
rg "func pathWithinRoot" --context 10
```

Repository: compozy/compozy

Length of output: 2188

---

🏁 Script executed:

```shell
#!/bin/bash
# Search for NewPathOutOfScopeError implementation
rg "func NewPathOutOfScopeError" --context 5
```

Repository: compozy/compozy

Length of output: 866

---

🏁 Script executed:

```shell
#!/bin/bash
# Check Go version in go.mod
head -5 ./go.mod
```

Repository: compozy/compozy

Length of output: 114

---

🏁 Script executed:

```shell
#!/bin/bash
# Search for any symlink resolution in the codebase
rg "EvalSymlinks|readlink|realpath|Symlink" --type go | head -20
```

Repository: compozy/compozy

Length of output: 1772

---

🏁 Script executed:

```shell
#!/bin/bash
# Check how resolveScopedPath is actually used to understand the full context
rg "resolveScopedPath" --context 3
```

Repository: compozy/compozy

Length of output: 2063

---



</details>

**Resolve symlinks before path containment checks to prevent traversal outside allowed roots.**

The current implementation uses only `filepath.Clean` and `filepath.Rel`—purely lexical operations that do not resolve symlinks. A symlink in the workspace (e.g., `workspace/link → /tmp/evil/file`) will pass the boundary check but allow actual file operations to escape the intended roots. Symlink traversal is a confirmed security vulnerability.

Since Go 1.26.1 is in use, migrate to the `os.Root` API (available since Go 1.24) for safe root-confined file access with platform-specific protections, or at minimum resolve symlinks with `filepath.EvalSymlinks` on the path before the containment check in `resolveScopedPath`. This also applies to the `pathWithinRoot` helper function (lines 667-671).

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/core/extension/host_helpers.go` around lines 528 - 555, The boundary
check in resolveScopedPath currently uses filepath.Clean and pathWithinRoot (and
pathWithinRoot itself) which are purely lexical and can be bypassed via
symlinks; update resolveScopedPath to resolve symlinks before containment checks
by calling filepath.EvalSymlinks (or migrate to the os.Root API if you prefer
platform-safe root confinement), apply EvalSymlinks to both the candidate path
and the roots (o.workspaceRoot and model.CompozyDir(o.workspaceRoot)), then use
pathWithinRoot against the evaluated real paths; also update pathWithinRoot to
operate on evaluated (symlink-resolved) paths so NewPathOutOfScopeError is only
raised when the real filesystem location is outside the allowed roots.
```

</details>

<!-- fingerprinting:phantom:medusa:grasshopper:ce4005c2-f225-49f9-bac5-6ac8c129da42 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `valid`
- Notes: The current scope check is purely lexical (`filepath.Clean` + `filepath.Rel`) and therefore does not stop symlink traversal outside the workspace root. Because the actual read/write operations later use plain `os.ReadFile` / temp-file writes by absolute path, this is a real confinement bug. The fix needs to move the actual artifact file operations onto root-confined filesystem APIs rather than only tightening the string check. That requires one minimal companion change outside the listed code-file set: `internal/core/extension/host_reads.go` must use the same root-confined access path as the helper/write side, otherwise read operations would remain vulnerable even after the helper change.
