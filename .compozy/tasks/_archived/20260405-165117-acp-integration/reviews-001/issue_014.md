---
status: resolved
file: internal/core/run/logging.go
line: 103
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc54vHCM,comment:PRRC_kwDORy7nkc60y0gQ
---

# Issue 014: _⚠️ Potential issue_ | _🟠 Major_
## Review Comment

_⚠️ Potential issue_ | _🟠 Major_

**Don't let UI publishing block the ACP read path.**

These sends are synchronous, but the UI channel is only buffered to `max(total*4, 4)` in `newUIController`. A verbose session can fill it and stall `HandleUpdate`, which pushes back on the execution path and can surface as avoidable hangs or timeouts.

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/core/run/logging.go` around lines 89 - 103, The UI channel sends in
HandleUpdate are synchronous and can block; change the direct h.uiCh <-
jobUpdateMsg{...} and h.uiCh <- usageUpdateMsg{...} to non-blocking sends so a
full UI buffer won’t stall the ACP read path. Replace each direct send with a
select { case h.uiCh <- msg: default: } pattern for the jobUpdateMsg (after
sessionView.Apply(update)) and for the usageUpdateMsg (inside the hasUsage
block), leaving the aggregateUsage lock/use logic unchanged.
```

</details>

<!-- fingerprinting:phantom:medusa:grasshopper:9505bd09-f35d-4087-8d05-bc08c4654a2d -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `valid`
- Rationale: `sessionUpdateHandler.HandleUpdate` sends snapshot and usage messages directly into the UI channel. Because `HandleUpdate` runs on the ACP session read path, a full UI buffer can currently stall transcript processing.
- Fix plan: Make the UI message sends non-blocking while keeping aggregate usage accounting unchanged.
- Resolution: Switched the UI snapshot and usage sends to non-blocking delivery and added a regression test proving a full UI channel no longer stalls update handling while usage still aggregates. `make verify` passed.
