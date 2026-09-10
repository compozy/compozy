# Beta.24 daemon CPU investigation

## Incident and measured baseline

The installed CompozyOS application and daemon both reported `0.3.0-beta.24` on
2026-09-10. With two user sessions open, Activity Monitor showed the daemon at
355.2% CPU. A live process inventory independently measured 384.5% CPU for daemon
PID 29459 and 37.5% for its Electron renderer. The host is an Apple M4 Max with 16
logical CPUs and 128 GiB RAM. Percentages use macOS process accounting: 100% means
one logical CPU, not the entire machine.

A subsequent six-sample `top` capture (discarding its initial zero interval)
reported daemon CPU values of 256.0%, 370.6%, 319.3%, 328.9%, and 358.8%. Other applications
were active, so these measurements describe the incident, not an isolated capacity
benchmark. The daemon's physical footprint was approximately 163–167 MiB during
most of this window (170 MiB in the last sample); memory exhaustion was not the
observed bottleneck.

The investigation sampled the running processes without stopping the user's
sessions. Raw local evidence remains under `.cache/cpu-investigation/`:

- `daemon-before.sample.txt`: ten-second macOS stack sample, 10 ms interval.
- `daemon-before.top.txt`: process CPU, memory, and context switches.
- `renderer-before.sample.txt`: five-second renderer sample.
- `registry-baseline-bench.txt` and `registry-baseline.cpu.pprof`: existing hosted
  projection microbenchmark and CPU profile.

These files may contain local paths and are not included in the public PR.

Read-only log inspection found 20,319 HTTP requests across approximately 79
minutes (4.27 requests/second), led by session catalog, identity, session detail,
prompt queue, and attention-summary reads. There were 149 intermittent model
catalog refresh warnings, not a tight error retry loop. Workspace resolution
timings are debug-level events and were absent at the installed logging level;
no claim about live cache-hit frequency is derived from those logs.

The live status API reported three active runtimes and four total sessions,
despite two visible sessions in the user's report. There were three ACP child
processes, three extension processes, and one workspace. The unscoped status
reported 23 global skills: 22 filesystem definitions and one bundled definition.
A targeted filesystem inventory additionally found 89 workspace `SKILL.md`
definitions among 872 regular files and approximately 1,378 traversable entries.
Five first-level workspace symlinks pointed outside the trusted roots and were
rejected. There was one active profile and no configured additional directories.
The workspace-scoped API returned 116 projected skills after layer merging;
projected catalog counts are not filesystem traversal counts. Controlled
two-client measurements below are separate from this live inventory. The released
binary records source revision `3327649151d9`; this branch starts at `53aa246b4`,
which also contains the existing post-release memory-catalog correction and an
E2E readiness-test correction. Those preexisting main-branch changes are outside
this PR's diff.

## Root cause

Each hosted MCP projection stream polls at the existing 100 ms interval. The
registry's full projection cache requires every provider to supply an
authoritative generation. The native provider intentionally does not do so,
because native availability can change independently of the daemon epoch. Each
poll consequently rebuilds the projection and reevaluates live policy and
availability.

The live daemon sample repeatedly follows this path:

```text
streamHostedMCPProjection
  HostedService.projectionForGeneration
    RuntimeRegistry.sessionProjection
      daemonExtensionToolProvider.canonicalWorkspaceScope
        workspace.Resolver.Resolve
          scanWorkspace / scanSkillSource / filepath.WalkDir
```

Extension descriptor discovery and individual handle resolution ask only for the
canonical registered workspace identity, but each invokes the full workspace
resolver. Even a full-resolver cache hit walks and parses its filesystem
dependencies before comparing snapshots. Thus the number of extension tools
multiplies skill-directory scanning during every projection refresh. The sample
also shows repeated descriptor schema validation during registry reconstruction.

Among collapsed leaf stacks, filesystem work includes 931 `open`, 509 `lstat`,
254 directory enumeration, and 81 `stat` observations. Sleeping threads are also
sampled, so these counts are evidence of the execution path, not CPU percentages.

## Corrections and compatibility

The workspace resolver now owns an identity-only registration operation. It
shares registration lookup, root reconciliation, and durable identity validation
with full resolution, but does not load configuration, agents, or skill trees.
Extension tool listing, handle resolution, and trusted call-root binding use this
operation. Root reconciliation and identity validation remain synchronous.

Registry indexing also reuses the native provider's privately owned descriptors,
which its constructor has already validated and cloned. Dynamic providers still
list and validate their current descriptors on every projection. Public descriptor
results and inputs to custom policy evaluators remain detached copies, preserving
mutation isolation, including the nested backend capability list. Conflict detection and ordering still run for each index.

Skill discovery and parsed names are reused only after synchronously checking
every traversed non-ignored directory, definition, filesystem identity, mode,
size, modification time, and relevant resolved link target. Directory checks
detect additions inside previously empty nested folders. Rejected first-level
links are checked independently, so the five rejected links in the incident's
workspace do not disable reuse of the entire valid tree. Incomplete, unreadable,
and truncated discovery results remain conservative cache misses.

The workspace's outer cache additionally compares the merged skill projection,
so replacing a definition while retaining its size and timestamps cannot hide a
new skill name after discovery detects the changed filesystem identity. Expired
workspace entries cannot supply discovery snapshots. Config, agent definitions,
and skill MCP sidecars retain their existing synchronous dependency checks; the
new reuse introduces no timer-based stale window.

Hosted MCP also reuses a projection digest only after rebuilding the live
projection and comparing every field with a private previous snapshot. A changed
view follows the original JSON encoding and SHA-256 algorithm, including error
handling for invalid schemas. The memo is owned by an active bind, isolated from
caller mutations, and synchronized for concurrent reads. It is released with the
bind rather than retained in an unbounded map. Sorting now uses one cloned view
slice instead of allocating a second intermediate slice.

A follow-up Go CPU profile before the fifth correction isolated the residual cost: 52.66% of sampled CPU passed
through the native policy resolver, with 41.37% in skill-source checks. Cumulative
profile percentages overlap and are not additive. A narrow workspace agent-config
operation now loads configuration and agent definitions without discovering skills.
It returns a distinct type and owns a separate cache, preventing a partial result
from entering the full workspace cache. Registration, profile availability,
configuration dependencies, agent definitions, sandbox validation, and cache
invalidation retain their existing checks. The resource agent catalog shares its
lookup logic between full and policy-only inputs, retaining profile precedence,
builtin fallback, and detached agent definitions.

The full projection continues to reevaluate live availability and policy. An
authoritative generation is not invented for mutable native callbacks. This is
necessary because session state, workspace files, extension state, and permissions
can change independently of a global daemon epoch.

The existing workspace benchmark with 200 synthetic skills measured full cached
resolution at 26.5–32.0 ms/op, about 6.30 MB/op, and 47,585 allocations. The new
registration operation measured 49.8–109 microseconds/op, about 12 KB/op, and 120
allocations across three runs on the busy host. A separate 128-tool native
projection benchmark improved from a median 2.234 ms/op to 0.149 ms/op across five
runs; allocations fell from 29,989 to 1,060 and bytes/op from 1,966,657 to 367,665.
These are per-operation results, not a prediction of whole-application CPU
reduction. The existing hosted unknown-generation projection benchmark improved
from a median 0.472 ms/op to 0.303 ms/op after digest reuse, with 1,125 to 1,012
allocations and approximately 670 KB/op to 425 KB/op. These runs overlapped other
test work on the host; runtime measurements remain the delivery evidence.

## Controlled runtime measurements

The isolated lab runs the actual daemon, two ACP subprocesses, and two attached
hosted MCP clients. An ACP fixture supplies deterministic sessions; no inference
provider is involved. Both clients retain their projection streams for a roughly
30-second measurement window. CPU is the daemon's user-plus-system CPU time
divided by elapsed wall time. The same lab is restarted with the released beta.24
binary and each implementation stage, using 23 and 200 synthetic skill
directories. Other applications remain active on the host.

| Implementation | 23 skills | 200 skills |
| --- | ---: | ---: |
| Released beta.24 | 103.560% | 216.216% |
| Identity-only resolution | 51.875% | 156.575% |
| Identity and native descriptor reuse | 34.691% | 112.026% |

These intermediate diagnostic builds used CGO; the release uses `CGO_ENABLED=0`
and `-trimpath`. Delivery measurements use matching release build settings. The
intermediate table documents how the profile narrowed the investigation and
does not isolate source changes from build configuration effects.

The representative workload reproduces the observed workspace's filesystem shape
using synthetic content: 89 skill definitions, 872 regular files, 501 directories,
and five rejected links, totaling 1,378 entries below the skill root. It also
includes 22 synthetic global skills. This reproduces traversal size, not the
user's content or inference-provider activity.

Repeated exploratory restarts accumulated stopped sessions in the lab. A later
profile showed that session maintenance added cost, so the authoritative
comparison removes stopped lab sessions through the supported CLI and verifies
exactly two active and two total sessions before and after each window. Both
binaries use `CGO_ENABLED=0` and `-trimpath`. The released binary uses Go 1.26.4;
the initial fixed builds use Go 1.26.6. The final build explicitly pins Go 1.26.4
for the authoritative matching-toolchain comparison.

| Clean representative workload | Go version | CPU-seconds | Wall seconds | Daemon CPU |
| --- | --- | ---: | ---: | ---: |
| Released beta.24 | 1.26.4 | 56.94 | 30.471 | 186.867% |
| First four corrections | 1.26.6 | 12.73 | 30.361 | 41.929% |
| All five corrections | 1.26.6 | 7.32 | 30.417 | 24.065% |
| All five corrections, matching release toolchain | 1.26.4 | 6.89 | 30.318 | 22.726% |

The first four corrections reduced CPU by 77.56% in this controlled comparison.
A 30.17-second Go CPU profile then recorded 12.23 CPU-seconds and attributed the
largest remaining cost to filesystem checks in policy resolution. This profile
motivated the agent-config correction. The Go 1.26.6 representative result is
87.12% less daemon CPU (7.76x lower) than released beta.24; this comparison includes
the compiler patch-version difference. The authoritative Go 1.26.4 result is
**87.84% less CPU (8.22x lower)**. The remaining 22.726% is measured overhead,
not a claim of zero idle cost. Two other worktrees were running frontend
validation during this shared-host window. Thirty observations and before/after
logs confirm this task's gate remained queued for the entire measurement.

Final Go 1.26.6 scale checks, also with exactly two active and two total sessions:

| Synthetic skill roots | CPU-seconds | Wall seconds | Daemon CPU |
| --- | ---: | ---: | ---: |
| 23 workspace, no global files | 6.35 | 30.378 | 20.903% |
| 200 workspace, no global files | 6.90 | 30.380 | 22.712% |

These checks retained the same 237-tool catalog. They demonstrate the remaining
polling cost no longer grows with the previous repeated full-tree work; they are
not paired clean-state comparisons with the earlier diagnostic builds.

Every measured client returned the same 237 descriptors, with canonical catalog
SHA-256 `92ff728f80cb445b7bde44f2dfcb6d82c264548c8110d13c5472655db6f6790c`.
The runtime canary verified matching active session IDs over MCP and CLI, a
successful skill list, and a successful status request over UDS. Both connected
clients observed extension disable/enable transitions of 237 → 234 → 237 tools,
and workspace `tools.enabled` changes of 237 → 0 → 237, restoring the exact
original catalog hash each time. Nested skill creation, rename, and removal
converged across MCP, CLI, and source provenance in 3.534, 2.896, and 3.085 seconds
on the final Go 1.26.4 build. That catalog uses the existing three-second watcher interval; the resolver's own
synchronous mutation contract is covered separately by its regression suite.

Behavior equivalence is checked separately for each correction:

| Correction | Preserved contract and owning evidence |
| --- | --- |
| Registration-only resolution | Same aliases, canonical identity and errors; registration behavior suite and unchanged runtime catalog. |
| Native descriptor reuse | Same ordering, conflict decisions, descriptors and live availability; registry/native suites, mutation regressions and catalog checksum. |
| Skill discovery reuse | Same names, precedence, provenance and containment; scanner/cache/integration suites, nested-change canaries and catalog checksum. |
| Hosted digest reuse | Same sorted JSON and SHA-256, including empty and malformed inputs; independent encoding oracle, mutation/concurrency suites and catalog checksum. |
| Agent-config resolution | Same config, agent selection and profile scope; narrow/full parity tests, shared catalog precedence tests and live policy canaries. |

No correction changes tie-breaking, floating-point calculations or RNG behavior.
The prioritization scores (impact × confidence / effort) were 25, 10, 6.7, 5 and
10 respectively; these are selection heuristics, not performance measurements.

## Change impact

This is the owning impact audit, following `docs/_memory/change-impact.md`.

- **Native tools:** tool IDs, schemas, dispatch policy, and hosted MCP names retain
  their existing contracts. The affected paths are extension tool discovery,
  native descriptor indexing, workspace and agent-config resolution, and digest calculation during
  hosted projection refresh. Native availability and dispatch policy remain live;
  digest bytes and wire encoding retain their existing contracts.
- **Extensibility and hooks:** extension workspace canonicalization must continue
  to accept the same aliases and fail closed for invalid identity or missing
  roots. Configuration keys, hook dispatch, and extension SDK contracts do not
  change.
- **Workspace data isolation:** registered workspace identity remains the
  authority; no identity result is cached with a time-based stale window. The
  owning workspace boundary retains root reconciliation and identity validation.
- **Official Compozy skill:** `skills/compozy/` documents public operations;
  their syntax, results, and ownership rules remain unchanged.
- **Web/Docs:** no Web DTO, cache, component, or layout contract changes. This
  report and the native-tool QA scenario record the performance regression and
  the corresponding runtime verification.

## Regression and delivery verification

The owning suites cover observable behavior at their existing boundaries:

- Workspace registration: aliases, durable identity, missing roots, invalid identity,
  and config-loader independence.
- Workspace and skill discovery: nested additions, edits, rename/removal, atomic
  replacement, rejected and changing symlinks, expiry, config/agent/profile freshness,
  explicit invalidation, and separation of narrow and full results.
- Registry and hosted MCP: dynamic availability with unknown generations, malformed
  dynamic schemas, conflicts, ordering, policy changes, exact digest compatibility,
  concurrency, and mutation isolation of inputs and outputs.
- Resource agent catalog: scope precedence and tie-breaking, builtin and snapshot
  fallback, errors, detached results, and changes between consecutive resolutions.

Passed before delivery:

```sh
CGO_ENABLED=1 go test -race ./internal/tools/... ./internal/mcp -count=1
CGO_ENABLED=1 go test -race -tags integration ./internal/skillscan ./internal/workspace -count=1
CGO_ENABLED=0 go build -trimpath -o .cache/cpu-investigation/compozy-policy-final ./cmd/compozy
```

The daemon policy and resource-catalog race suite also passed:

```sh
CGO_ENABLED=1 go test -race ./internal/daemon -run 'TestDaemonNativeRuntimePolicyResolver|TestResourceAgentCatalog|TestAgentCatalogLens|TestLoopEffectRelay' -count=1
```

The three targeted daemon integration tests passed in 17.899 seconds against the
Go 1.26.6 CGO-disabled production build:

```sh
python3 .cache/cpu-investigation/run-targeted-integration.py .cache/cpu-investigation/compozy-policy-final
```

That helper acquires the repository's shared verification lock and runs
`go test -race -tags integration -gcflags=modernc.org/...=-d=checkptr=0 -p 2
-parallel 4 -timeout 20m ./internal/daemon` with the existing hosted MCP workspace
API, extension authoring, and extension lifecycle parity integration tests selected.
It supplies the real daemon and ACP fixture binaries through the existing test
configuration. Final runtime canaries passed on both toolchains. The matched
Go 1.26.4 production build passed the representative CPU window and all canaries.
Its SHA-256 is `c4bfad8774cf066f13549a163d176903e3988834dd1e63dfc94ff7eb7f6e385f`.

```sh
GOTOOLCHAIN=go1.26.4 CGO_ENABLED=0 go build -trimpath -o .cache/cpu-investigation/compozy-policy-go1264 ./cmd/compozy
```

`make gate` passed with zero lint issues: the affected Go race suites passed in
276 seconds, including the daemon package in 253.321 seconds. The initial lint
run found three promoted-field selector style issues in mutation-isolation tests;
they were corrected without changing production behavior or assertions. The final
run passed all affected lanes. Subsequent edits only record delivery evidence.

The strict QA audit passed with zero blockers and zero warnings. Canonical lab
teardown completed with `clean: true` and zero surviving lab processes. The
teardown first verified process identities and targeted only the six current lab
processes; it did not stop the user's daemon or another worktree's processes.

Final local evidence root:
`/Users/pedronauck/dev/qa-labs/compozy-beta24-cpu-20260910-162426-604899-lab/qa-artifacts/qa/`.
It contains `qa-audit-report.json`, `teardown.json`, `verification-report.md`,
`logs/final-make-verify.log`, the final binary proof and the
`final-policy-go1264-clean-state-representative-*` measurement/queue records.
Formatting and commit-message checks were run directly before committing, avoiding
lint-staged's automatic stash behavior. No schema, configuration key, tool ID, public
DTO, or persisted shape changes, so migrations and wire generation are not needed.

## Verification limits

The installed application remains the released beta.24 until the user updates it.
An isolated result must not be presented as a measurement of a patched live user
daemon. The renderer sample alone does not establish a JavaScript hotspot; the
confirmed incident path is the Go daemon's hosted projection work.

The controlled ACP subprocesses use a deterministic fixture. This verifies real
daemon, MCP, CLI, UDS, extension, and filesystem behavior; it does not measure
inference-provider throughput or establish a patched renderer result. The initial
and final samples run on a shared active host, not a dedicated benchmark machine.
