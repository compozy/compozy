---
status: resolved
file: internal/api/core/interfaces.go
line: 503
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc58gqW0,comment:PRRC_kwDORy7nkc651WGb
---

# Issue 005: _⚠️ Potential issue_ | _🟠 Major_
## Review Comment

_⚠️ Potential issue_ | _🟠 Major_

**Keep `RunDetailPayload` bounded.**

`RunService` already exposes paginated `Events`, but this payload adds a raw `[]events.Event` timeline and also nests `Snapshot`, which already contains the current `Run`. For large runs, the detail endpoint will ship overlapping data and grow with run length on every page load.

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/api/core/interfaces.go` around lines 497 - 503, RunDetailPayload
currently embeds a full Run via Snapshot and an unbounded []events.Event
timeline which can duplicate data and grow without bound; change
RunDetailPayload to avoid duplicating Run by replacing Snapshot RunSnapshot with
a lighter RunReference (or remove Snapshot) and replace Timeline []events.Event
with a paginated/pointer field (e.g., TimelinePageToken, TimelineCount or
paginated Events slice type already exposed by RunService) so callers must
request pages; update usages of RunDetailPayload, RunSnapshot, and consumers
expecting timeline to use the RunService paginated Events API instead to keep
responses bounded.
```

</details>

<!-- fingerprinting:phantom:medusa:grasshopper:a9f5b26c-6acb-406f-a7a4-c71fefe05a3a -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `invalid`
- Notes:
  - `RunDetailPayload` is not currently exposed by any HTTP handler or browser OpenAPI route; the browser uses `/api/runs/{run_id}` for summary and `/api/runs/{run_id}/snapshot` plus streaming for detail rendering.
  - The boundedness concern is real in the abstract, but refactoring this internal payload now would create wide churn without fixing any shipped transport behavior in this batch.
  - The existing paginated `/api/runs/{run_id}/events` surface is the correct place for future pagination work if this payload becomes public later.

## Resolution

- Closed as analysis-only. No code change was warranted because the cited payload is not part of the current browser HTTP contract and the batch scope did not expose a concrete shipped defect here.
- Verified with `make verify`.
