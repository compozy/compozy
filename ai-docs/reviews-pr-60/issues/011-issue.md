# Issue 11 - Review Thread Comment

**File:** `internal/core/run/execution_acp_integration_test.go:1182`
**Date:** 2026-04-05 19:22:18 America/Sao_Paulo
**Status:** - [x] RESOLVED

**Disposition:** VALID

**Rationale:** `captureExecuteStreams()` currently waits until `fn()` returns before draining either pipe. That can block the code under test once the OS pipe buffer fills, so the readers need to start before `fn()` runs.

## Body

_⚠️ Potential issue_ | _🟠 Major_

**Drain the capture pipes while `fn()` is running.**

Both pipes are read only after `fn()` returns. Once stdout or stderr exceed the pipe buffer, the code under test blocks on write and the test hangs. Start background readers before invoking `fn()`, then wait for them after closing the writers.

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/core/run/execution_acp_integration_test.go` around lines 1137 -
1182, The captureExecuteStreams helper blocks if fn() writes more than the OS
pipe buffer because stdout/stderr are only read after fn returns; fix by
starting background goroutines that continuously read from stdoutRead and
stderrRead into byte buffers (or io.ReadAll) before calling fn(), use channels
to return the read results (bytes and any read error), then call fn(), close
stdoutWrite/stderrWrite, and wait for the reader goroutines to finish and send
their results; reference the symbols captureExecuteStreams, stdoutRead,
stdoutWrite, stderrRead, stderrWrite and runErr to locate where to spawn the
readers, pass their outputs back, and return the collected stdout/stderr strings
along with runErr.
```

</details>

<!-- fingerprinting:phantom:medusa:grasshopper:5606214d-8719-43d5-98a2-064a6cd78ca1 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Resolve

Thread ID: `PRRT_kwDORy7nkc547tHq`

```bash
gh api graphql -f query='mutation($id:ID!){resolveReviewThread(input:{threadId:$id}){thread{isResolved}}}' -F id=PRRT_kwDORy7nkc547tHq
```

---
*Generated from PR review - CodeRabbit AI*
