---
provider: coderabbit
pr: "133"
round: 1
round_created_at: 2026-04-30T20:37:59.817595Z
status: resolved
file: internal/daemon/review_exec_transport_service_test.go
line: 195
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc5-22ER,comment:PRRC_kwDORy7nkc68_V6p
---

# Issue 012: _🛠️ Refactor suggestion_ | _🟠 Major_ | _⚡ Quick win_
## Review Comment

_🛠️ Refactor suggestion_ | _🟠 Major_ | _⚡ Quick win_

**Restructure these new daemon transport tests into `t.Run("Should ...")` subtests.**

Please align the added cases with the mandated test pattern; these scenarios are good candidates for table-driven subtests and parallel execution where safe.

 

As per coding guidelines, "Use table-driven tests with subtests (`t.Run`) as the default pattern" and "MUST use t.Run("Should...") pattern for ALL test cases".

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/daemon/review_exec_transport_service_test.go` around lines 126 -
195, These two test functions
(TestTransportReviewServiceFetchSyncsNoMetaRoundAfterLegacyReview and
TestTransportReviewServiceStartWatchUsesDaemonRunManager) must be restructured
into t.Run("Should ...") subtests (or table-driven subtests) to follow the
mandated pattern: wrap each scenario in t.Run with a descriptive "Should ..."
name, move their setup and assertions into the subtest closure, ensure proper
variable capture (use local copies if looping), and call t.Parallel() inside
subtests where safe; keep the existing calls to newTransportReviewService,
service.Fetch/GetRound/ListIssues and service.StartWatch unchanged but invoked
from within the t.Run closures.
```

</details>

<!-- fingerprinting:phantom:poseidon:hawk:9d503b4c-1a51-4ef5-a14d-2e16d6ffd95a -->

<!-- d98c2f50 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `valid`
- Root cause: The two new daemon transport review tests are standalone top-level bodies instead of the required `t.Run("Should...")` subtest structure.
- Fix plan: Wrap each scenario in a descriptive `Should ...` subtest and preserve the current setup and assertions.
