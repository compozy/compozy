# Daemon CPU hot paths — targeted runtime validation

Scope: issue #1794, six daemon CPU phases. An isolated daemon, real HTTP/UDS and hosted MCP
stdio transports, scripted provider-free ACP runtimes and loopback MCP servers exercise the
changed paths. This run does not claim browser UX, live-provider or real OAuth account coverage.

## Session matrix

| Persona | Journey / probe | Surface | Status | Evidence |
| --- | --- | --- | --- | --- |
| Dora, agent owner | Edit `deny_tools` in an active agent; observe withdrawal within one second | Authored agent + hosted MCP | Pass | Final manual run: `deny-edit.json`, 0.621321 s; independent API read |
| Théo, workspace operator | Approve a project MCP definition in an open session | Settings HTTP + hosted MCP | Pass | Final manual run: `project-approval.json`; separate Settings GET |
| Théo, delegating agent | Spawn a child restricted to one of the parent's servers | Native spawn + child hosted MCP | Pass | Final manual run: `child-ceiling.json`; separate session detail GET |
| Operator | Fourteen bound runtimes, two paired CPU captures and allocation delta | CLI + HTTP + hosted streams | Pass | Final profile run: 14 live runtimes, 217 detail reads, no request errors |
| Operator | Hot activation, dead-entity recovery and publication with a missing workspace | Existing transport integration suites | Pass | Final focused integration selection, 33.030 s |

## Charter

Enter through public session creation and prompt APIs. Observe the hosted catalog through the actual
injected stdio bridge. Edit the authored agent, approve an external project definition through Settings,
and invoke the public spawn tool. Confirm each saved change through a separate read and retain parent
and child process identities until teardown. Use the configuration/permission tour; the adjacent canary
is next-message MCP activation and recovery without session recreation. Each runtime probe has a
bounded deadline. Provider fixtures hold turns open without paid provider calls.

## Session debriefs

### Dora — authored policy edit

The final public-transport walk withdrew `compozy__tool_search` in 0.621321 seconds after
an authored `deny_tools` edit. A separate agent API read confirmed the agent remained
readable, and another hosted listing confirmed the withdrawal. Restoring the definition
restored the tool. Earlier development walks exposed stale published agent records;
publication now follows the bounded observer independently of the configured skills cadence.

### Théo — project admission and delegation

A newly declared project server was absent while waiting for consent. The Settings approval
write returned `allowed`, a separate Settings read confirmed it, and the existing hosted
bridge advertised its tools without a restart. The actual native spawn tool created a child
restricted to alpha: alpha remained visible while beta and gamma remained hidden, confirmed
again after an independent session-detail read. An isolated CLI status read also succeeded.

The final manual lab's strict evidence audit passed and canonical teardown reported
`clean: true`, with no survivors. Evidence labels: `daemon-cpu-manual-final`,
`deny-edit.json`, `project-approval.json`, `child-ceiling.json`, `qa-audit-report.json`,
`teardown.json`. Raw runtime identifiers and profiles remain in private scratch evidence.

### Operator — controlled performance comparison

Both binaries ran the same provider-free workload: fourteen live ACP sessions, five authored
agents, two loopback MCP servers with 64 tools and twenty schema fields each, hosted projection
subscribers, one native catalog read per second and alternating session-detail reads at 5 Hz.
Each run captured two 20-second CPU profiles and an allocation delta. The base was
`f7498334f33d94695c762071c6fd3ca02f29a120`. The final binary used the implementation sources
reviewed and verified here; the 62 changed Go inputs were hash-checked after building.
The final binary SHA-256 was
`09a290b8454611dd9083045da96260f6ae68bca553336e669e91388c793160b3`.

Final capture window: 40.1794 seconds, fourteen live runtimes and 217 detail reads.
Each MCP server received sixteen discovery/list requests across the entire final lab run,
versus 212 across the baseline run; counters include setup and warm-up. No paid provider
requests or operator-state access occurred. The final profile lab passed strict auditing and
canonical teardown with `clean: true` and no survivors. All twelve labs owned by this task
have clean teardown receipts, including the earlier failed attempts.

### Measured criteria

Cells show CPU seconds and share of each capture's own total. Cumulative shares overlap;
they must not be added. Zero means no sampled CPU, not proof that the function never ran.
Allocation is a sampled profile delta divided by the measured interval. Targets are unchanged
and judged on the isolated harness; the brief's live S1/S2 figures are context only.

| Metric | Base CPU 1 | Base CPU 2 | Final CPU 1 | Final CPU 2 | Target | Result |
| --- | --- | --- | --- | --- | --- | --- |
| Total CPU per 20 s | 10.88 s (100%) | 10.50 s (100%) | 1.23 s (100%) | 2.66 s (100%) | ≤4 s | Met |
| `ResolveAgentConfig` cumulative | 2.91 s (26.75%) | 3.17 s (30.19%) | 0.02 s (1.63%) | 0.08 s (3.01%) | ≤2% | Miss in capture 2 |
| `syscall.rawsyscalln` flat | 4.59 s (42.19%) | 4.70 s (44.76%) | 0.33 s (26.83%) | 0.78 s (29.32%) | ≤8% | Miss |
| `EffectiveMCPAuthority` cumulative | 4.30 s (39.52%) | 4.50 s (42.86%) | 0.09 s (7.32%) | 0.20 s (7.52%) | ≤5% | Miss |
| `ProjectionVisibility` cumulative | 2.10 s (19.30%) | 2.43 s (23.14%) | 0.13 s (10.57%) | 0.25 s (9.40%) | ≤2% | Miss |
| Hosted stream cumulative | 2.22 s (20.40%) | 2.55 s (24.29%) | 0.13 s (10.57%) | 0.28 s (10.53%) | ≤5% | Miss |
| `ListSessionMCP` cumulative | 2.34 s (21.51%) | 2.51 s (23.90%) | 0 sampled | 0 sampled | ≤3% | Met |
| `Manager.Status` cumulative | 1.50 s (13.79%) | 1.51 s (14.38%) | 0.02 s (1.63%) | 0.06 s (2.26%) | ≤3% | Met |
| GC background worker + allocation assist | 0.57 s (5.24%) | 0.42 s (4.00%) | 0.04 s (3.25%) | 0.09 s (3.38%) | ≤8% | Met |
| Allocation rate, per run | 424.47 MB/s | Same paired run | 46.19 MB/s | Same paired run | ≤60 MB/s | Met |

The paired CPU reductions are 88.69% and 74.67%; allocation fell 89.12%. Both captures
remain visible rather than selecting the better result. Earlier optimized captures at the
observer stage were 2.12/2.49 seconds and 45.10 MB/s; their four missed share targets were
already tracked separately in #1797. The final pair additionally misses the agent-config
share in its second capture. This report makes no claim that every original target passed.

The new stat observer sampled 0/0.01 CPU seconds (0/0.38%). This measures the final lab's
workspace/profile population, not arbitrary numbers of registered workspaces. Residual syscall
stacks include detail/SQLite reads, process scans and persistence; their exact remaining work
belongs to the separately scoped #1797. That follow-up remains unqueued. Its recorded decision
keeps this implementation within the six phases.

### Focused benchmark samples

One run each, Apple M4 Max, Darwin arm64, `GOMAXPROCS=2`, `-benchmem -benchtime=1s`:

| Benchmark | Time | Bytes/op | Allocs/op |
| --- | --- | --- | --- |
| Agent-config hit | 41,163 ns/op | 15,265 | 141 |
| Hosted unchanged, unversioned | 43,826 ns/op | 24,883 | 196 |
| Hosted unchanged, versioned | 37,633 ns/op | 15,008 | 169 |
| Hosted fanout, 16 streams, shared | 3,047,424 ns/op | 6,131,863 | 14,934 |
| Hosted fanout, 16 streams, unshared | 6,158,185 ns/op | 12,877,621 | 28,454 |
| Native projection, 128 allowed | 182,749 ns/op | 426,273 | 1,216 |
| Native projection, 8 allowed | 70,553 ns/op | 230,752 | 363 |

Fanout reported one provider listing per epoch in every case. These are focused benchmark
samples, not a before/after benchmark study; the runtime profiles own the paired comparison.

## Runtime errors observed

Earlier experiments failed on harness readiness, an ACP fixture selector and a missing
required spawn TTL; these were setup failures and were repaired before the final walk.
A policy-publication delay was a production regression in the implementation under review;
the owned observer, retained publication scopes and retry path corrected it. Focused
regressions cover slow refresh, first reads, idle expiry, unreadable dependencies and missing
workspace folders. An integration failure on macOS root aliases exposed an approval-path
identity bug; the final implementation preserves persisted lexical filenames, symlinks and
literal percent escapes. The final transport walk reported no runtime errors. The loopback HTTP fixture printed a
connection-reset traceback after the profile result while connections were closing; the
capture had no request errors, all fourteen runtimes were ready, both strict audits passed
with zero warnings/blockers, and teardown confirmed no survivors.

## Checks and coverage

Focused selections passed in workspace, config, fileutil, redaction, tools, MCP auth, MCP,
session, daemon and both API transports. All nine affected lint packages passed using the
repository-pinned golangci-lint v2.15.0. Final daemon integration selection passed in 33.030 s:
`TestDaemonIntegrationMCPDeadEntityRecoversWithoutRestart`, `TestDaemonMCPHotActivation`,
`TestDaemonMCPHotActivationInheritance`, `TestAgentSkillPublicationAndBootRebuild`.
The final build passed. The test-convention checker introduced zero findings across eleven
changed test files; pre-existing heuristic findings in their canonical suites are unchanged.
Production Go files remain at most 500 lines (largest changed file: 497).

Independent Claude Opus 5.5 High review approved correctness and contracts after five rounds.
It ran no tests, lint, builds, profiles or QA and did not certify performance.

Focused Go tests and lint run locally. CI owns repository-wide verification, race and E2E gates.
The live daemon and operator state are excluded from this run.

## Remaining tradeoffs and scope decision

Known builtin read calls avoid global projection wakes. External and mutation calls retain
conservative invalidation. Read-local memo locks still span I/O; static review found no
recursive path. HTTP listing cache hits can defer outage observation for the three-second
floor or a longer server-provided TTL; fresh discovery and actual calls still reach the server,
and auth, consent, resource and secret identities are checked before cache reuse.

Decided: ≤1 s bound approved by Pedro (2026-11-17).
The scope decision recorded in #1797 retains all six phases here and tracks residual CPU
separately; this run neither changes the target thresholds nor executes that follow-up.

## Final status

Targeted runtime QA is complete: three manual transport checks and the adjacent integration
selection passed, both strict audits passed, and all owned labs have clean teardown evidence.
The original high-CPU symptom has measured improvement; five percentage criteria remain
unmet in the final pair and are disclosed above. No browser, live-provider, full OAuth-account,
repository-wide, race or E2E coverage is claimed locally. Existing broader scenario verdicts
remain historical; their new impact notes point to this bounded evidence.

PR delivery is governed separately by the current head's CI and review evidence. This report
certifies the targeted runtime QA described above.

The [review remediation run](2026-11-17-daemon-cpu-hot-paths-review.md) records the
repeated targeted QA and the updated CPU and allocation comparison.
