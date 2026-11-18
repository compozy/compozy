# Daemon CPU hot paths — review remediation validation

This report covers review remediation on PR #1802 for issue #1794. It uses the
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

## Second review round

- The registry replaces its policy, index and listing memos when a session
  summary inherits a status context from an older revision. The summary and
  external scope caches use the revision captured at the start. Each cache
  checks the live revision before saving. The regression changes `deny_tools`
  after loading status authority through a real workspace resolver. The same
  context and the next fresh read both show one usable server instead of two.
- Hosted projections track the secret-redaction fingerprint, including
  authoritative generations and shared stream listings. The regression
  registers a secret after discovery and verifies that the projection redacts
  it while reusing the upstream tool listing.
- Authored agent validation now receives the caller context. The skills
  polling default also owns the MCP summary expiry. The provider fingerprint
  test changes a sibling's availability, and the workspace test consumes the
  policy edit before verifying sidecar creation independently.
- Focused `go test -p 2` and golangci-lint v2.15.0 passed for config, daemon,
  MCP, tools and workspace. The four daemon integration scenarios above
  passed in 33.001 s. The CLI/daemon build passed and the test-shape checker
  reported no new diagnostics.

The current binary repeated all three transport journeys. A `deny_tools` edit
took effect in 0.656 s. Project approval exposed tools without restarting the
session. A child restricted to alpha kept beta and gamma hidden. Independent
HTTP reads confirmed each result. The profile used 14 live runtimes, 214 detail
reads and the same loopback MCP workload. Each upstream server received 17
listing requests including setup and warm-up. Both labs passed the strict audit
and targeted teardown. The binary SHA-256 was
`c705d66040f065d910ee349fd1a71fac39a2ff241cbca7843917b0e9fa5afff9`.

| Metric | Round 2 CPU 1 | Round 2 CPU 2 | Target | Result |
| --- | --- | --- | --- | --- |
| Total CPU per 20 s | 1.73 s | 2.11 s | ≤4 s | Met |
| `ResolveAgentConfig` cumulative | 0.05 s (2.89%) | 0.02 s (0.95%) | ≤2% | Miss in capture 1 |
| `syscall.rawsyscalln` flat | 0.49 s (28.32%) | 0.63 s (29.86%) | ≤8% | Miss |
| `EffectiveMCPAuthority` cumulative | 0.14 s (8.09%) | 0.09 s (4.27%) | ≤5% | Miss in capture 1 |
| `ProjectionVisibility` cumulative | 0.31 s (17.92%) | 0.17 s (8.06%) | ≤2% | Miss |
| Hosted stream cumulative | 0.35 s (20.23%) | 0.18 s (8.53%) | ≤5% | Miss |
| `ListSessionMCP` cumulative | 0 sampled | 0 sampled | ≤3% | Met |
| `Manager.Status` cumulative | 0.04 s (2.31%) | 0.01 s (0.47%) | ≤3% | Met |
| GC background worker and assist | 0.05 s (2.89%) | 0.06 s (2.84%) | ≤8% | Met |
| Sampled allocation rate per run | 46.57 MB/s | Same run | ≤60 MB/s | Met |

GitHub's second SQLite acceptance run preserved 830,020 rows and met all
throughput floors. It recorded zero head operation failures. One incomplete
baseline pair applied the 1 s absolute limit and the head write reached
1.028 s. The warning criteria independently rejected one telemetry hold.
The workload calls the stores directly and does not exercise the daemon
projection changes. The cause of the failed threshold remains unverified.
[SQLite acceptance log](https://github.com/compozy/compozy/actions/runs/44175674353/job/134366239707).

## Third review round

- A shared projection uses the listing window that started its flight. A peer
  in a new window starts a separate flight, and a slower older flight cannot
  overwrite the newer snapshot. The regression returned the old description
  on the previous code and the current description after the repair.
- The MCP provider's authoritative generation includes the redaction
  fingerprint. Only MCP provider listings depend on it within a shared window.
  Registering an unrelated secret now preserves single-flight reuse for stable
  providers. Both hosted redaction regressions still refresh metadata without
  another upstream listing.
- The hosted projection and credential suites passed 12 repetitions with eight
  parallel tests. Focused tests and golangci-lint v2.15.0 passed for daemon, MCP
  and tools. The four daemon integration scenarios above passed in 31.892 s.
  The CLI/daemon build and test-shape check passed.

All three public-transport journeys passed. The `deny_tools` edit took
0.662 s. Project approval showed the tools without a restart. The restricted
child continued to see alpha alone. The profile used 14 live runtimes, 215
detail reads and 16 upstream `tools/list` calls per server, including setup
and warm-up. Both labs passed the strict audit and targeted teardown. The
binary SHA-256 was `cdacffb738519cff791182e7b3322510e0a354cdccc5d75f38b56efd88950555`.

| Metric | Round 3 CPU 1 | Round 3 CPU 2 | Target | Result |
| --- | --- | --- | --- | --- |
| Total CPU per 20 s | 1.67 s | 2.18 s | ≤4 s | Met |
| `ResolveAgentConfig` cumulative | 0.01 s (0.60%) | 0.01 s (0.46%) | ≤2% | Met |
| `syscall.rawsyscalln` flat | 0.49 s (29.34%) | 0.67 s (30.73%) | ≤8% | Miss |
| `EffectiveMCPAuthority` cumulative | 0.18 s (10.78%) | 0.15 s (6.88%) | ≤5% | Miss |
| `ProjectionVisibility` cumulative | 0.14 s (8.38%) | 0.09 s (4.13%) | ≤2% | Miss |
| Hosted stream cumulative | 0.23 s (13.77%) | 0.14 s (6.42%) | ≤5% | Miss |
| `ListSessionMCP` cumulative | 0 sampled | 0 sampled | ≤3% | Met |
| `Manager.Status` cumulative | 0.04 s (2.40%) | 0.02 s (0.92%) | ≤3% | Met |
| GC background worker and assist | 0.04 s (2.40%) | 0.13 s (5.96%) | ≤8% | Met |
| Sampled allocation rate per run | 45.87 MB/s | Same run | ≤60 MB/s | Met |

## Fourth review round

- A secret registered again after its last release gets a new fingerprint. An
  unrelated temporary registration leaves the fingerprint unchanged after that
  registration ends. Exact binary redaction uses an immutable snapshot.
- When the active revision changes during a hosted projection build, bind and
  stream reads filter MCP metadata against a fresh snapshot and leave the result
  uncached for that revision. The regression covers eager bind and deferred
  bootstrap, followed by a stream read. Safe tools remain visible in all three
  paths.
- The provider validation cache has an internal capacity option with a default of
  128. The MCP listing floor option is package-private. The watcher test verifies
  that the live policy applies while the published catalog still has the old
  definition.
- Focused tests and golangci-lint v2.15.0 passed for daemon, MCP, redaction and
  tools. The hosted projection and credential suites passed 12 repetitions
  with eight parallel tests. The four daemon integrations above passed in
  37.073 s. The CLI and daemon build and whitespace check passed.

All three transport journeys passed. The authored policy withdrew the tool in
0.543 s, project approval exposed tools without a restart, and the restricted
child saw alpha alone. The standard profile used 14 live runtimes and 218
detail reads over 40.639 s. Each server received 17 `tools/list` requests
including setup and warm-up. The lab binary SHA-256 was
`d4046e8d2a715b05e4bbc1bcf7aee24599439387f0f7d123c401c673f7ad1e60`.

| Metric | Round 4 CPU 1 | Round 4 CPU 2 | Target | Result |
| --- | --- | --- | --- | --- |
| Total CPU per 20 s | 1.89 s | 3.06 s | ≤4 s | Met |
| `ResolveAgentConfig` cumulative | 0.08 s (4.23%) | 0.02 s (0.65%) | ≤2% | Miss in capture 1 |
| `syscall.rawsyscalln` flat | 0.74 s (39.15%) | 0.87 s (28.43%) | ≤8% | Miss |
| `EffectiveMCPAuthority` cumulative | 0.13 s (6.88%) | 0.22 s (7.19%) | ≤5% | Miss |
| `ProjectionVisibility` cumulative | 0.20 s (10.58%) | 0.13 s (4.25%) | ≤2% | Miss |
| Hosted stream cumulative | 0.26 s (13.76%) | 0.14 s (4.58%) | ≤5% | Miss in capture 1 |
| `ListSessionMCP` cumulative | 0 sampled | 0 sampled | ≤3% | Met |
| `Manager.Status` cumulative | 0.09 s (4.76%) | 0.05 s (1.63%) | ≤3% | Miss in capture 1 |
| GC background worker and assist | 0.05 s (2.65%) | 0.10 s (3.27%) | ≤8% | Met |
| Sampled allocation rate per run | 45.49 MB/s | Same run | ≤60 MB/s | Met |

The credentialed comparison repeated the workload on the baseline and Round 4
with two loopback MCP servers requiring Vault-backed headers. Each run also
made 45 explicit server checks during capture. The baseline completed 191
detail reads and made 236/235 upstream `tools/list` requests. Round 4 completed
217 detail reads and made 41/40 listings. Both runs authenticated every request
and had zero errors. All four labs passed strict audit and targeted teardown.

| Metric | Credentialed base CPU 1 | Credentialed base CPU 2 | Credentialed Round 4 CPU 1 | Credentialed Round 4 CPU 2 |
| --- | --- | --- | --- | --- |
| Total CPU per 20 s | 11.55 s | 11.86 s | 2.31 s | 2.59 s |
| Hosted stream cumulative | 2.37 s (20.52%) | 2.72 s (22.93%) | 0.15 s (6.49%) | 0.17 s (6.56%) |
| Sampled allocation rate per run | 435.72 MB/s | Same run | 56.33 MB/s | Same run |

The normal registry stops the authoritative generation check when a provider
reports an unknown generation. Neither credentialed capture sampled
`projectionSourceKey` or `captureAuthoredAgent`. The first Round 4 credentialed
capture sampled 0.01 s (0.43%) in `renderHTTPServer`. This workload exercises
Vault-backed request rendering through the real daemon and loopback transports.

Focused microbenchmarks measured the paths directly. Median key construction
took 1.04 µs for HTTP and 2.66 µs for stdio, or 17.94 µs and 18.08 µs with one
Vault reference, respectively. The benchmark used a temporary SQLite vault and
the configured file key provider. Server definitions came from a fixture, and
the measurement excluded transport setup and network calls. Resolving one
authored policy against five definitions and a stale catalog took 7.46 µs
and 57 allocations. These are medians of three 200 ms Go benchmark repetitions.
The generation check keeps live credential lookup so rotation changes the
identity immediately. Authored policy validation keeps the publisher's
canonical spec and identity for precedence decisions.

## Final status

Targeted runtime QA passed. CPU and allocation met their absolute targets.
The tables retain every percentage miss. Issue #1797 tracks the four residual
hotspots identified for separate work. The first standard Round 4 capture also
missed the `ResolveAgentConfig` and `Manager.Status` shares. Both stayed below
0.1 s in that capture and met their share targets in the second capture and
both credentialed captures.
