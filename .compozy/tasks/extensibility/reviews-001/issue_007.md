---
status: resolved
file: internal/cli/extension/install.go
line: 323
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc56RVOy,comment:PRRC_kwDORy7nkc621Vaj
---

# Issue 007: _⚠️ Potential issue_ | _🟠 Major_
## Review Comment

_⚠️ Potential issue_ | _🟠 Major_

**Reject or sanitize symlinks instead of recreating them verbatim.**

Lines 300-323 preserve absolute links and `../` escapes exactly as they appeared in the source tree. That makes the installed extension depend on paths outside its install directory and can unintentionally expose arbitrary host files when the extension runs later.


Based on learnings: Keep the system single-binary and local-first.

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/cli/extension/install.go` around lines 299 - 323, copySymlink
currently recreates symlinks verbatim which allows absolute links or "../"
escapes to point outside the install dir; change copySymlink to refuse or
sanitize such links: after reading linkTarget in copySymlink, detect unsafe
targets (filepath.IsAbs(linkTarget) or path elements with ".." that would escape
the extension root when resolved) and either return an error rejecting the
symlink or rewrite it to a safe relative link (or fall back to copying the
resolved file contents) so the installed extension cannot reference files
outside its install directory; ensure any sanitization uses filepath.Clean/IsAbs
and resolves the final target relative to the source tree before deciding.
```

</details>

<!-- fingerprinting:phantom:medusa:grasshopper:388c7721-ae35-4ddb-a0e7-78fbd9aa7a58 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `VALID`
- Notes:
  - `copySymlink` currently recreates the source link target verbatim, including absolute paths and relative `..` escapes that can point outside the installed extension tree.
  - Root cause: symlink copying does not validate whether the resolved link target stays within the source extension root before recreating it in the destination tree.
  - Implemented fix: resolved the link target relative to the source tree, rejected targets that escape the extension root, and recreated safe internal links as destination-relative symlinks.
  - Additional scope needed: minimal test-only harness updates outside the listed code files may be required because the current symlink-copy regression test assumes unsafe absolute-link preservation.
