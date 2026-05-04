---
status: resolved
file: internal/daemon/query_documents.go
line: 180
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc58gqX2,comment:PRRC_kwDORy7nkc651WH4
---

# Issue 014: _⚠️ Potential issue_ | _🔴 Critical_
## Review Comment

_⚠️ Potential issue_ | _🔴 Critical_

**Reject symlinked markdown entries.**

`WalkDir` will surface `*.md` symlinks, and the later `os.ReadFile` follows them. A repo-controlled symlink under `memory/` or `adrs/` can therefore expose arbitrary host files through these read-model endpoints. Please reject symlinks, or resolve the path and verify it still stays under `cleanRoot` before appending it.

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/daemon/query_documents.go` around lines 153 - 180, WalkDir currently
includes symlinked *.md files which can point outside cleanRoot; detect symlinks
(use entry.Type()&fs.ModeSymlink or filepath.EvalSymlinks on path) before
appending to entries and either skip/reject them or resolve them and verify they
remain under cleanRoot: call filepath.EvalSymlinks(path) to get the resolved
path, compute its absolute/clean path and ensure it has cleanRoot as its prefix
(using filepath.Clean + filepath.Abs or similar), then use os.Stat on the
resolved path for size/mtime and only append a markdownDirEntry if the resolved
path stays within cleanRoot; otherwise return an error or skip the entry. Ensure
references to cleanRoot, WalkDir callback, entry.Info()/os.Stat, and
markdownDirEntry are updated accordingly.
```

</details>

<!-- fingerprinting:phantom:medusa:grasshopper:e75280e4-7172-485f-934b-c3510e24ebf0 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `valid`
- Notes:
  - `readMarkdownDir` currently accepts symlinked `*.md` entries, and later reads follow those symlinks, which can escape the intended workflow root.
  - Root cause: the directory walk filters by extension only and never rejects symlinked markdown entries before appending them to the document list.
  - Intended fix: reject symlinked markdown entries during enumeration and add regression coverage around symlink handling.

## Resolution

- Rejected symlinked markdown entries during directory scans and direct reads, and added regression tests covering the symlink-escape cases.
- Verified with `make verify`.
