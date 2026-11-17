# Daemon CPU hot paths — review remediation validation

This pass covers the first review round on PR #1802 for issue #1794. It uses the
same isolated workload and public transports as the [initial run](2026-11-17-daemon-cpu-hot-paths.md).
The workload uses scripted ACP runtimes, loopback MCP servers and equivalent Web
detail traffic through HTTP. Browser UX and live-provider accounts are outside
this run.

## Session matrix

| Persona | Journey | Surface | Status | Evidence |
| --- | --- | --- | --- | --- |
| Dora, agent owner | Edit active `deny_tools` | Authored agent and hosted MCP | Pass | Tool withdrawn in 0.629 s. A separate agent read succeeded. |
| Théo, workspace operator | Approve a project MCP server in an open session | Settings HTTP and hosted MCP | Pass | Server hidden before consent and visible after it. A separate Settings read returned `allowed`. |
| Théo, delegating agent | Spawn a child restricted to one parent server | Native spawn and child hosted MCP | Pass | Alpha visible, beta and gamma hidden, including after a separate session detail read. |
| Operator | Compare two 20 s CPU captures and an allocation delta | CLI, HTTP and hosted MCP | Pass | 14 live runtimes, 216 detail reads, zero request errors. |
| Operator | Check hot activation, recovery and publication | Existing daemon integration suites | Pass | Four integration scenarios passed in 31.912 s. |

## Session debriefs

Dora observed the tool disappear from the running bridge after editing the
authored definition. The agent remained readable through HTTP. Restoring the
definition restored the tool. Théo observed that the pending project server
was absent before approval and callable after it, without a session restart.
A child spawned through the native tool advertised alpha alone. The isolated
CLI status read also succeeded.

Both labs passed the strict evidence audit with no blockers or warnings.
Targeted teardown returned `clean: true` with no surviving owned processes in
either lab. The binary SHA-256 was
`27693827c79429ac401458dd44e50b260c9839f0592f3a23e50826a67038a96e`.

## Controlled comparison

The baseline at `f7498334f33d94695c762071c6fd3ca02f29a120` used the same machine,
fourteen-session workload, two local servers with 64 tools and 20 schema fields
each, one native catalog read per second, and HTTP detail reads at 5 Hz.
The new capture window was 40.239 s. Each MCP server received 17 discovery/list
requests across the run, including setup and warm-up, compared with 212 in the
baseline. CPU samples are absolute seconds and share of the corresponding
capture. Cumulative shares overlap. A zero means no samples in that capture.

| Metric | Base CPU 1 | Base CPU 2 | Review CPU 1 | Review CPU 2 | Target | Result |
| --- | --- | --- | --- | --- | --- | --- |
| Total CPU per 20 s | 10.88 s | 10.50 s | 2.22 s | 1.85 s | ≤4 s | Met |
| `ResolveAgentConfig` cumulative | 2.91 s (26.75%) | 3.17 s (30.19%) | 0.05 s (2.25%) | 0.01 s (0.54%) | ≤2% | Miss in capture 1 |
| `syscall.rawsyscalln` flat | 4.59 s (42.19%) | 4.70 s (44.76%) | 0.69 s (31.08%) | 0.49 s (26.49%) | ≤8% | Miss |
| `EffectiveMCPAuthority` cumulative | 4.30 s (39.52%) | 4.50 s (42.86%) | 0.20 s (9.01%) | 0.08 s (4.32%) | ≤5% | Miss in capture 1 |
| `ProjectionVisibility` cumulative | 2.10 s (19.30%) | 2.43 s (23.14%) | 0.25 s (11.26%) | 0.06 s (3.24%) | ≤2% | Miss |
| Hosted stream cumulative | 2.22 s (20.40%) | 2.55 s (24.29%) | 0.28 s (12.61%) | 0.10 s (5.41%) | ≤5% | Miss |
| `ListSessionMCP` cumulative | 2.34 s (21.51%) | 2.51 s (23.90%) | 0 sampled | 0 sampled | ≤3% | Met |
| `Manager.Status` cumulative | 1.50 s (13.79%) | 1.51 s (14.38%) | 0.05 s (2.25%) | 0.01 s (0.54%) | ≤3% | Met |
| GC background worker and assist | 0.57 s (5.24%) | 0.42 s (4.00%) | 0.09 s (4.05%) | 0.05 s (2.70%) | ≤8% | Met |
| Sampled allocation rate per run | 424.47 MB/s | Same run | 44.36 MB/s | Same run | ≤60 MB/s | Met |

## Review and verification

- Policy reads apply current validated definitions while full resource
  publication waits. The observer retains published scope dependencies so a
  cached session summary can still be invalidated after its own policy reads
  stop resolving the workspace. The configured skills watcher retains its
  own cadence.
- Each summary refresh gives each session its own read context. Policy and
  catalog revisions fence the cache. Independent idle eviction releases
  detail snapshots that were never published.
- MCP listing caches and projection generations use one expiration policy.
  Credential rotation changes both keys. Provider fingerprints use only that
  provider's availability, and the workspace scanner observes a definition
  created in an existing empty agent directory.
- Independent Claude Opus 5.5 High review approved the corrections from
  static inspection. The policy overlay uses the normalized spec returned
  by the publishing codec. The watcher test uses virtual time and joins its
  goroutines. The obsolete zero-TTL projection helper was removed.
- Focused `go test -p 2` passed for workspace, tools, MCP and daemon. The
  watcher subtest passed 12 consecutive runs. Repository-pinned
  golangci-lint v2.15.0 was clean for all four packages. No new test-shape
  findings appeared. The CLI/daemon build passed.
- The integration selection ran
  `TestDaemonIntegrationMCPDeadEntityRecoversWithoutRestart`,
  `TestDaemonMCPHotActivation`, `TestDaemonMCPHotActivationInheritance`, and
  `TestAgentSkillPublicationAndBootRebuild` using `-tags=integration`.

The initial CI head exposed a race in the old watcher test's counter, which
also failed the aggregate Verify check. The independent SQLite acceptance
job completed its 1 GB upgrade and met the throughput floors. It failed
paired load after transcript writes reached 2.055 s in a trial with a
1 s limit, alongside 26 operation errors in the head runs. The load harness
exercises the stores directly. This round leaves the SQLite workload and
acceptance policy intact and relies on CI for a fresh result.

## Final status

Targeted runtime QA passed. CPU and allocation met their absolute targets.
The table preserves each percentage miss. Issue #1797 tracks the four
residual hotspots already identified for separate work. Delivery remains
subject to CI and review on the pushed head.
