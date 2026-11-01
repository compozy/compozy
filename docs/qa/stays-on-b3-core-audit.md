# Proactive core coverage audit

Worktree: stays-on-b3. Initial implementation commit: 2086e2f7f.
Required cases are mapped below to their canonical owning suites.
Evidence combines real SQLite lifecycle checks, provider subprocess startup, and boundary policy suites.

| ID | Owning test | Audit status |
| --- | --- | --- |
| UT-032 | TestInstanceDueFiltering | Active/due included; paused/future excluded |
| UT-033 | TestInstanceCheckGates | On-demand gate and zero dispatch asserted |
| UT-034 | TestInstanceCheckGates | Empty gate and zero dispatch asserted |
| UT-035 | TestHeartbeatPreferences | Floor assertion located |
| UT-036 | TestInstanceCheckBackoff | Third quiet doubles cadence, sixth reaches cap; retries do not advance |
| UT-037 | TestInstanceProposalDedupe | Duplicates suppressed within 24h and delivered at 24h |
| UT-038 | TestParseTriageReply | Proposal cap asserted |
| UT-039 | TestInstanceCheckGates; TestHeartbeatWakeCancellation | Timer/manual veto and legacy-source cancellation behavior asserted |
| UT-040 | TestInstanceCheckGates | Failure streak and zero quiet streak asserted |
| UT-041 | TestInstanceCheckFatigue; TestInstanceReportPublication | Tenth failure pauses; one durable fatigue report and notification |
| UT-087 | TestInstanceDigestSuppression | Repeated digest makes zero calls until max interval |
| UT-088 | TestInstanceWakeTransactions | Failed finish preserves digest and cursor |
| UT-089 | TestInstanceWakeTransactions | Concurrent admission admits exactly one |
| UT-090 | TestInstanceCheckCapacity | Two admitted, three remain due with unchanged slots |
| UT-091 | TestInstanceCheckGates | Repeated tick creates one event |
| UT-092 | TestCollectDigest | Unknown collector diagnostic without error |
| UT-093 | TestTriageReplyOutcome; TestTriageDispatcher | Free text produces invalid-output failure |
| UT-094 | TestInstanceProposalDedupe | Reworded proposal with same item keys suppressed |
| UT-095 | TestInstanceProfileDedupe | Same agent and scope in two profiles both deliver |
| UT-096 | TestInstanceCollectorSecrets | Real task run contains secret; facts and digest expose neither token nor field |
| UT-107 | TestInstanceCollectorCompletion | Report append leaves digest fingerprint unchanged |
| UT-108 | TestInstanceCollectorCompletion | Eleven failures consumed in two successful checks; failed check preserves first page |
| UT-109 | TestInstanceDigestSuppression | A to empty to A makes no dispatch |
| UT-110 | TestCollectDigest | Missing context invokes all five collectors |
| UT-111 | TestParseTriageReply | Fence and digest-key filtering asserted |
| UT-112 | TestCollectDigest | Predecessor-owned task included |
| UT-113 | TestInstanceFinishOutcomes | Added nine persisted result/reason cases with released-slot readback |
| UT-114 | TestInstanceChecksToggle | Enable schedules one cadence ahead; retry preserves schedule |
| UT-115 | TestInstanceCheckGates | Repeated gate preserves next wake |
| IT-011 | TestDueInstanceTriagePolicy; TestTriagePolicy; TestTriageStartup; TestTriageToolPolicy; TestTriageSessionPermissions | Due scheduler to dispatcher to session restricts startup and persists unsupported failure; ACP and tool boundaries reject writes without approvals |
| IT-012 | TestTriageDispatcher | Quiet hidden session deleted; focused race execution passed |
| IT-013 | TestInstanceReportPublication | Proposal notification and no model prompt asserted; race execution passed |
| IT-014 | TestInstanceMissedDueTimes | Unbound main session does not prevent a single overdue dispatch |
| IT-029 | TestInstanceCheckRecovery | Recovery writes one failure before ticks; repeated recovery does not replay |
| IT-030 | TestInstanceWakeTransactions | Pause/resume retains slot and suppresses report |
| IT-039 | TestInstanceManualCheckRace; TestAgentInstanceCheck | Race admits one; early manual preserves cadence; API gate returns 200 |
| IT-040 | TestInstanceWakeMigration | Real migration, Recover/Tick, zero dispatch and no new event asserted |

Validation so far: heartbeat race suite passed; ACP/tools TestTriage race suites passed;
TestInstanceFinishOutcomes race suite passed; test-shape checker passed.
The full globaldb run from the first turn failed after 603.145s with a large goroutine dump; it is not passing evidence. Use the focused TestInstance
run for this slice and preserve any eventual full-run failure evidence.

Additional validation: `go build ./...` passed. `CGO_ENABLED=1 go test -race
./internal/store/globaldb -run '^TestInstance' -count=1 -timeout=90s` passed
(35.410s), including the real SQLite upgrade/no-wake scenario.

Turn 3: focused session/API triage, report publication, and check suites passed with race. Backoff, proposal dedupe, and digest suppression assertions reviewed against UT-036, UT-037, UT-087, UT-094, UT-109.

Turn 4: TestInstanceCollectorSecrets passed with race (3.374s). IT-011 was subsequently covered by TestDueInstanceTriagePolicy (turn 5).

Turn 5: TestDueInstanceTriagePolicy passed with race. Provider startup I/O uses the existing fake driver; actual unsupported-provider negotiation is exercised by TestTriageStartup with its helper subprocess. No live provider account was used.

Final focused command passed across all seven packages:

```sh
CGO_ENABLED=1 go test -race ./internal/heartbeat ./internal/store/globaldb ./internal/session ./internal/acp ./internal/tools ./internal/hooks ./internal/api/core -run 'Test(Instance|DueInstanceTriagePolicy|Triage|CollectDigest|ParseTriageReply|HeartbeatPreferences|HeartbeatWakeCancellation|AgentInstanceCheck)' -count=1 -timeout=120s
```

This includes TestInstanceWakeMigration (upgrade, recovery and tick produce no wake)
and TestTriagePolicy/TestTriageToolPolicy (writes and MCP denied without approval).
Global gates and E2E were intentionally excluded by the implementation-worker brief.

Final `go build ./...` passed after the triage integration addition.
