---
status: resolved
file: internal/daemon/run_snapshot_compact.go
line: 131
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc5-QUlj,comment:PRRC_kwDORy7nkc68K-Qi
---

# Issue 032: _⚠️ Potential issue_ | _🟠 Major_
## Review Comment

_⚠️ Potential issue_ | _🟠 Major_

**`SessionUpdateCount > TranscriptMessageCount` will over-mark compact runs as incomplete.**

The compact snapshot path is supposed to collapse many session updates into a much smaller transcript tail. Tool-update churn and chunked messages make this condition true for healthy runs, so you'll emit `runIntegrityReasonTranscriptGap` even when nothing is missing.

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/daemon/run_snapshot_compact.go` around lines 129 - 131, The current
check using stats.SessionUpdateCount > stats.TranscriptMessageCount incorrectly
flags compact snapshot runs; change the condition so
runIntegrityReasonTranscriptGap is only appended for non-compact runs (i.e.,
skip this check when the compact snapshot path is used). Concretely, update the
if that references stats.SessionUpdateCount and stats.TranscriptMessageCount to
also check the compact flag for this run (e.g., !run.IsCompact or
!compactSnapshot) before appending runIntegrityReasonTranscriptGap so healthy
compacting churn doesn’t produce a false transcript-gap reason.
```

</details>

<!-- fingerprinting:phantom:medusa:grasshopper:8d3d9d9d-d8a1-4421-95c0-379c08c617ed -->

<!-- 4e71b3a2 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `valid`
- Notes: Confirmed compact snapshot integrity auditing compared dense session-update counts to compact transcript-tail message counts. Removed that transcript-gap check from the compact snapshot path so healthy compaction does not mark runs incomplete.
