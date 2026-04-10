---
status: resolved
file: internal/core/provider/coderabbit/nitpicks.go
line: 355
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc56KkMC,comment:PRRC_kwDORy7nkc62sRjV
---

# Issue 005: _⚠️ Potential issue_ | _🟠 Major_
## Review Comment

_⚠️ Potential issue_ | _🟠 Major_

**The nitpick hash is too coarse-grained.**

`ReviewHash` currently ignores the location, so two identical nitpicks in the same file but on different lines collapse into one entry in `latestByHash`. Once one of those gets resolved, the history filter can also suppress the other in later rounds. The hash needs an additional stable location discriminator instead of just `file + title + body`.

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/core/provider/coderabbit/nitpicks.go` around lines 346 - 355, The
current buildNitpickHash function is too coarse because it omits a stable
location discriminator; modify buildNitpickHash to include a stable location
component (e.g., a normalized line/column span or token range) in the canonical
string so identical nitpicks at different locations produce different hashes.
Specifically, add a location parameter (stableLocation or startLine:endLine) to
the buildNitpickHash signature, incorporate canonicalHashValue(location) into
the canonical join alongside "file:", "title:", and "body:" (and still use
firstParagraph(body)), and ensure callers that compute nitpick hashes (places
that populate latestByHash) pass the stable location value; keep using
sha256.Sum256 and nitpickHashLength unchanged.
```

</details>

<!-- fingerprinting:phantom:medusa:grasshopper:3c6a12e0-accd-4d1e-a3db-5a1d78606f67 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `valid`
- Root cause: `buildNitpickHash` only hashes provider, file, title, and the first paragraph of the body, so identical nitpick text in the same file but at different locations collapses into one dedup key.
- Impact: Distinct nitpicks can overwrite each other in `latestByHash`, and once one location is resolved the history filter can suppress the other location in later rounds.
- Fix approach: Add a stable location component to the nitpick hash input, update the caller that computes hashes to pass that location, and add a focused parser regression test in `internal/core/provider/coderabbit/nitpicks_test.go` even though that test file is outside the batch code-file list because the behavior change needs direct coverage.
- Resolution: CodeRabbit nitpick hashes now include the normalized location string, and a focused regression test in `internal/core/provider/coderabbit/nitpicks_test.go` proves identical nitpick text at different locations stays distinct.
- Verification: `go test ./internal/cli ./internal/core ./internal/core/provider/coderabbit` passed, and `env -u COMPOZY_NO_UPDATE_NOTIFIER make verify` passed cleanly.
