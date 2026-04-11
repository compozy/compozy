---
status: resolved
file: internal/core/migration/migrate.go
line: 194
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc55LZWX,comment:PRRC_kwDORy7nkc61XmQ9
---

# Issue 012: _⚠️ Potential issue_ | _🟠 Major_
## Review Comment

_⚠️ Potential issue_ | _🟠 Major_

**Don't downgrade every inspect failure to “invalid artifact”.**

`appendTaskMigration`, `appendReviewMigration`, and `recordRoundMeta` swallow wrapped read/parse errors and only keep the path. That makes permission/I/O failures indistinguishable from malformed content and drops the original cause from the returned error.

As per coding guidelines, "Prefer explicit error returns with wrapped context using `fmt.Errorf(\"context: %w\", err)`".

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/core/migration/migrate.go` around lines 166 - 194, The three
functions appendTaskMigration, appendReviewMigration, and recordRoundMeta
currently swallow all errors from
inspectTaskArtifact/inspectReviewArtifact/inspectRoundMeta by calling
s.recordInvalid(path) and returning nil; instead, preserve and return the
original errors wrapped with context (e.g., fmt.Errorf("inspectTaskArtifact %s:
%w", path, err)) so I/O/permission/parse failures remain visible to callers.
Concretely: in appendTaskMigration and appendReviewMigration, on error do not
just call s.recordInvalid(path) and return nil — return a wrapped error; if you
have a specific sentinel error for “invalid artifact” (e.g.,
ErrInvalidArtifact), you may still call s.recordInvalid(path) for that case but
still return the wrapped sentinel error. Apply the same change to
recordRoundMeta (wrap and return inspectRoundMeta errors instead of silently
recording invalid).
```

</details>

<!-- fingerprinting:phantom:medusa:grasshopper:6c29911c-ba13-4d74-ad6e-790b2357b234 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `valid`
- Notes:
  The migration scan currently records invalid paths but drops the specific read/parse failure that made them invalid, so callers only get a generic invalid-count error. That loses important diagnostics for permissions, I/O, or malformed front matter. The fix is to keep the invalid-path accounting while preserving wrapped causes in the final returned error.
  Resolution: implemented and verified with the focused Go test run plus `make verify`.
