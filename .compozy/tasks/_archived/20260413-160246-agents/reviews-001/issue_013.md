---
status: resolved
file: internal/core/run/internal/acpshared/command_io.go
line: 136
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc56P5t3,comment:PRRC_kwDORy7nkc62zc8n
---

# Issue 013: _⚠️ Potential issue_ | _🟠 Major_
## Review Comment

_⚠️ Potential issue_ | _🟠 Major_

**Don't make reusable-agent setup telemetry a prerequisite for starting the ACP session.**

Lines 125-135 now fail the whole job if lifecycle event submission breaks. That turns a journal/payload problem into an execution outage even though the agent session could still run. This path should degrade to a warning or a best-effort emit instead of aborting setup.

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/core/run/internal/acpshared/command_io.go` around lines 125 - 136,
The current call to emitReusableAgentSetupLifecycle (with req.Context,
req.RunJournal, req.Config.RunArtifacts.RunID, req.Job) aborts setup on error by
closing outFile/errFile, closing client, calling releaseClient(), and returning
the error; change this to best-effort: invoke emitReusableAgentSetupLifecycle
and if it returns an error, log a warning with the error details but do NOT
close outFile/errFile, do NOT close or release the client, and do NOT return;
allow the ACP session to continue (optionally spawn a non-blocking retry/report
task), so failures in telemetry emission degrade to warnings instead of failing
session startup.
```

</details>

<!-- fingerprinting:phantom:medusa:grasshopper:5b83f2d8-737c-414c-9d4a-933187b6f725 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `valid`
- Root cause: `SetupSessionExecution()` treated reusable-agent setup lifecycle emission as mandatory and tore down the ACP client/session logs when the telemetry submit path failed.
- Fix: Made reusable-agent setup lifecycle emission best-effort, logging a warning and continuing session startup; added regression coverage with a failing runtime-event submitter.
- Evidence: `go test ./internal/core/run/internal/acpshared`
