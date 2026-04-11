---
status: resolved
file: internal/core/model/artifacts.go
line: 79
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc55LZWc,comment:PRRC_kwDORy7nkc61XmRD
---

# Issue 014: _⚠️ Potential issue_ | _🟠 Major_
## Review Comment

_⚠️ Potential issue_ | _🟠 Major_

**Sanitize `safeName` inside `JobArtifacts`.**

`filepath.Join` will happily resolve values like `../x` or `nested/name`, so this helper currently trusts every caller to keep job names safe. That can redirect artifact writes outside the intended flat `jobs` namespace. `NewRunArtifacts` already hardens `runID`; this path builder should apply the same kind of normalization or reject unsafe names before joining.

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/core/model/artifacts.go` around lines 74 - 79, The JobArtifacts
helper trusts the caller's safeName and can be tricked into creating paths
outside the intended jobs directory; update RunArtifacts.JobArtifacts to
sanitize/normalize safeName (same approach used in NewRunArtifacts) before
joining: reject empty or names containing path separators or "..", or else
replace with filepath.Base / strings.ReplaceAll to strip directory components
and disallow traversal, and then use that sanitized name to build PromptPath,
OutLogPath and ErrLogPath so artifact writes remain confined to the flat jobs
namespace.
```

</details>

<!-- fingerprinting:phantom:medusa:grasshopper:a60f1eb0-d795-4bb2-8b4d-2afc11c2fe85 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `valid`
- Notes:
  `RunArtifacts.JobArtifacts` trusts its `safeName` argument and joins it directly under `JobsDir`, so callers can escape the flat jobs namespace with separators or traversal segments. The fix is to sanitize the job artifact name in this helper, using the same normalization rules already applied to run IDs.
  Resolution: implemented and verified with the focused Go test run plus `make verify`.
