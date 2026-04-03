---
status: resolved
file: internal/core/run/logging.go
line: 155
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc54vHCQ,comment:PRRC_kwDORy7nkc60y0gU
---

# Issue 015: _⚠️ Potential issue_ | _🟠 Major_
## Review Comment

_⚠️ Potential issue_ | _🟠 Major_

**Always finalize `done` on completion errors.**

If `writeRenderedLines` fails in this branch, the function returns before `markDone`, so any caller waiting on `Done()` can block forever in the error path. Finalization should happen before returning, or in a `defer`.

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/core/run/logging.go` around lines 136 - 155, The error path in
sessionUpdateHandler.HandleCompletion returns early if writeRenderedLines fails,
so markDone (which signals Done()) is never called and callers can block; ensure
markDone is always invoked on completion errors by calling h.markDone(err, true)
before returning from the writeRenderedLines error branch (or install a defer at
the top of HandleCompletion that calls h.markDone with the appropriate
error/flag) so that Done() is always finalized even when writeRenderedLines
fails.
```

</details>

<!-- fingerprinting:phantom:medusa:grasshopper:9505bd09-f35d-4087-8d05-bc08c4654a2d -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `valid`
- Rationale: In `HandleCompletion(err)`, if writing the rendered completion error fails, the method returns before `markDone` runs. That leaves the handler's `Done()` channel open forever on that error path.
- Fix plan: Finalize the handler before returning from the write-failure branch and add a regression test that exercises the failing writer path.
- Resolution: `HandleCompletion` now finalizes `Done()` before returning a completion-write failure, and the new failing-writer regression test proves the original completion error is still preserved. `make verify` passed.
