# Go Modernization Handoff

Checkpoint date: 2026-08-04
Repository: `/Users/pedronauck/Dev/compozy/compozy2`
Branch: `refac-go`
Pull request: [#293 — refactor: modernize Go runtime packages](https://github.com/compozy/compozy/pull/293)
Goal status: **all implementation and behavioral owners closed; final automated gate pending**

## Read this first

The restarted Go 1.26.4 audit and implementation are source-frozen. Do not restart the eight-slice
analysis, run another Deep Review round, or reopen a Verified owner without a new source-backed defect.
The canonical execution record is:

- `analysis/golang-master/current-backlog.md`

Its closeout matrix contains 98 `Verified`, 11 `Complete`, 2 `Rejected`, and 0 `Active` rows.

Shell commands must be prefixed with `rtk`. Preserve the unrelated local commit
`f40c110c build: change explorer subagent`. Never use destructive Git commands without explicit
operator permission.

## Repository and PR state

Current local HEAD before the final modernization commit:

```text
f40c110ce7d2faadbf4afee8382d304604daa261
f40c110c build: change explorer subagent
```

The modernization history immediately below it is:

```text
c8e2171b docs: record Go modernization handoff
3073a824 refactor: modernize Go runtime packages
53521cec refactor: go improvements
```

At this checkpoint:

- PR #293 is open and draft.
- Local `origin/main...HEAD` is 2 commits behind and 4 commits ahead.
- The remote checks reflect the older pushed head. The failing shard is
  `TestProviderAuthLoginCommand/Should_run_configured_native_login_command_locally`, where the old
  expectation wanted `unknown` but runtime truth was `missing_cli`; the current working tree already
  contains the provider hard cut and its focused verification.
- Do not rebase the dirty working tree. If the operator later requests a rebase, activate
  `git-rebase`, create a recoverable safety branch, and preserve every modernization commit plus
  `f40c110c`.

## Completed implementation

The eight fresh evidence slices cover the requested roots and are retained beside this file:

- `analysis/golang-master/01_analysis_command-config.md`
- `analysis/golang-master/02_analysis_api-transports.md`
- `analysis/golang-master/03_analysis_daemon-lifecycle.md`
- `analysis/golang-master/04_analysis_sessions-workspaces.md`
- `analysis/golang-master/05_analysis_persistence-memory.md`
- `analysis/golang-master/06_analysis_orchestration-domain.md`
- `analysis/golang-master/07_analysis_extensions-bridges.md`
- `analysis/golang-master/08_analysis_tools-security-sdk.md`

The final inventory contains 149 buildable packages in the requested roots: 146 under `cmd/**` and
`internal/**`, plus 3 SDK packages. It contains 5,424 Go paths; all 28 untracked Go sources created by
the implementation are assigned to the eight slice owners.

The source-frozen implementation includes:

- selective adoption or explicit rejection of all 20 requested Go 1.21–1.26 features;
- complete value-consuming `errors.AsType` migration with predicate-only residual classification;
- bounded SDK framing, callbacks, cleanup, and redaction;
- capability-bound config, registry, SessionDB, Daytona, memory, and managed-install file access;
- atomic Task lifecycle commands, fencing, recovery, and audit publication;
- exact SessionDB workspace/physical-family ownership and clear/delete recovery;
- bounded daemon, subprocess, ACP, bridge, hook, scheduler, and worker shutdown;
- provider cache, terminal launch identity, Daytona ownership, and write-only login commands;
- injective Notification/Bridge identity and exact opaque transport filters;
- extension startup rollback, publish/install archive budgets, outbound URL policy, Git 2.37 minimum,
  trust consent, and CLI/HTTP/UDS/native/Web management;
- Vault `aes-gcm-v1` ciphertext authenticated to canonical ref and kind;
- removed inert `memory.recall.signals.metrics_enabled` contract across runtime, config, API, Web,
  generated output, docs, tests, and fixtures;
- aligned site docs, official `skills/compozy/` references, QA scenarios, and generated contracts.

PM14 remains deliberately Rejected: there is no truthful durable removal event to publish. The
runtime must not invent one.

## QA-discovered correction

Fresh extension QA found and fixed
`docs/qa/bugs/BUG-20260804-native-extension-remediation.md`.

Before the fix, human CLI, JSON, and HTTP carried the authored missing/outdated-Git remediation, but
`compozy__extensions_install` collapsed the same failure to generic `dependency_missing`. Production
now:

- maps missing and outdated Git to `extension_git_unavailable` and
  `extension_git_version_unsupported`;
- reuses one canonical Git dependency diagnostic;
- projects only the explicitly safe `operator_cause` and `operator_recovery` fields through tool
  API/CLI transport;
- filters every other backend detail.

The invariant is owned by the existing daemon native-extension suite, API core tool-error suite, and
CLI structured tool-error suite. Focused tests passed, and real-daemon missing-Git plus Git-2.36
native invocations passed after the correction.

## Verification evidence

Before closeout QA, the source-frozen tree passed:

- base and integration typed-errcheck with zero findings;
- focused and full affected race suites recorded in the backlog, including 12,466 root executions for
  the final `errors.AsType` wave and the full SDK suite;
- focused SessionDB, Task, provider, extension, registry, Vault, memory, transport, and lifecycle
  owners;
- Go lint with zero issues;
- the explicit source-freeze `make codegen` followed by `make codegen-check`;
- generated OpenAPI, TypeScript types, host API contracts, migration references, CLI references, and
  design-token checks;
- formatting, protected migration hashes, production file caps, source-policy scans, Windows
  cross-compilation where applicable, and `git diff --check`.

The mandated Web QA startup also ran its documented `make web-dev` bootstrap, which invokes code
generation internally. No generated contract changed after the source-freeze codegen check.

The first workstream `make gate-full` attempt exposed three production regressions after all earlier
lanes passed: an extension diagnostic bypassed its constructor, SQLite trigger text lost its domain
sentinel identity under modernc v1.53 formatting, and two E2E artifact collectors shared one directory.
All three production fixes and their affected race suites pass. A fresh `make gate-full` must run after
this handoff and the QA report are the last repository mutations. Read the current evidence with
`rtk make gate-status`.

That fresh attempt reached the final boundary lane after 20,057 Go tests and failed because the strict
leaf allowlist predated `gitsrc` adoption of the shared `outboundpolicy` package. `outboundpolicy` has
only standard-library imports, so the exact leaf dependency is now declared without widening a prefix;
`make boundaries` passes.

## Closeout QA

Canonical report:

- `docs/qa/reports/2026-08-04-go-modernization-closeout.md`

Isolated lab evidence:

```text
/Users/pedronauck/dev/qa-labs/
  compozy-go-modernization-closeout-20260804-121411-946266-lab/qa-artifacts/qa
```

Passed or fixed:

- `MS-026` — Memory queue/retry truth, Web abandonment/save, restart retention, removed-key rejection,
  and removed-control absence;
- `RT-025` and `RT-027` — provider public descriptors, write-only command handling, and related
  CLI/native/HTTP/UDS/Settings/Doctor/Web surfaces;
- `ET-extension-cli-error-remediation` — passed after the QA-discovered native remediation fix;
- `ET-web-extension-union-install` — all three source members, separate Git Version field, inline
  HTTP/SSH/credential/query/fragment recovery, explicit unverified consent, and daemon truth;
- Marketplace/Skills canary — local extension install/enable/list/remove and bundled `compozy` skill
  discovery.
- `ET-extension-published-source-installs` — public GitHub release and pinned direct-Git installs,
  matching sidecar integrity, deliberate mismatch rejection before mutation, provenance, invocation,
  cleanup, release restoration, and fresh reinstall;
- `ET-extension-publish-install-round-trip` — public `v0.1.0` and behavior-changing `v0.2.0`
  archives/sidecars, install, invoke, update, invoke, and remove;
- `MS-040` — metadata-only public Vault flow plus a permanent real-SQLite/key-file corruption harness
  for copied-reference, changed-kind, obsolete-format, and plaintext-free failure invariants.

Visual evidence includes deterministic 1440×900 and 320×800 Memory/Extensions captures plus browser
captures for the Git hint and all five validation states. Visual Contract Mode did not apply because
the implementation spec/backlog did not cite the updated OpenDesign artifact as a parity reference.

The lab teardown is complete. `teardown.json` records `clean: true`, zero survivors, and reaping of the
registered Vite and daemon PIDs. The daemon required the helper's bounded signal escalation after its
graceful stop window.

The QA bootstrap now supports an explicit `targeted` evidence profile with operator-declared required
surfaces. The continuation lab uses CLI/API/runtime requirements and retains strict final-report and
full-gate checks without inventing agents, channels, Tasks, artifacts, Web, or provider sessions.
Its `teardown.json` records `clean: true`, daemon PID `51570` reaped, and zero survivors.

## Resolved verification capabilities

- `08-F5` is Verified through public fixture
  `https://github.com/compozy/compozy-extension-qa-fixture`, tagged releases `v0.1.0` and `v0.2.0`,
  and the continuation lab at
  `/Users/pedronauck/dev/qa-labs/compozy-go-modernization-targeted-f5-f8-20260804-134807-481811-lab/qa-artifacts/qa`.
- `08-F8` is Verified through
  `TestGlobalDBVaultCiphertextIdentityEnforcement` in the existing GlobalDB Vault suite. The harness is
  permanent and runs against a real SQLite database and key file; no test-only production endpoint or
  reusable manual corruption script was added.

Provider `RT-026` separately remains `blocked-verify` only for its real-Daytona clause. RT-025/RT-027
are sufficient to accept `08-F2`; no fake Daytona backend was substituted.

## Compozy Impact Audit

- **Native tools:** No tool ID, toolset, descriptor, input/output schema, digest, risk flag, or
  capability gate changed at closeout. `compozy__extensions_install` now returns specific missing/old
  Git reason codes plus the two safe operator remediation fields in the existing details map. API/CLI
  sanitizers, native mapping, descriptor lookup, and real-daemon calls were checked; arbitrary detail
  remains masked. Accumulated Task/provider/notification/registry/MCP/automation descriptors and tests
  remain aligned.
- **Extensibility and hooks:** Checked extension registry/install/publish, GitHub/ClawHub/MCP outbound
  policy, Marketplace Web union, CLI/HTTP/UDS/native management, extension resources, bridge SDK,
  hooks, capabilities, and `extensions.trust.allow_unverified`. No new hook event, capability,
  sidecar, or config key/default was added. Public GitHub and direct-Git install/invoke/update/remove
  passed with explicit unverified consent; a mismatched release sidecar failed before mutation.
- **Workspace data isolation:** Closeout added no durable datum. Memory queue/retry settings are
  daemon-global config; the deleted metrics key stores nothing. Extension dependency remediation is
  transient invocation metadata and contains no workspace content. Existing session/workspace owner,
  cache, SSE, event, and SQLite-family checks remain source-frozen. Vault ciphertext is bound to its
  canonical ref/kind and public projections remain metadata-only.
- **Official Compozy skill:** `skills/compozy/references/capabilities.md` co-ships the extension source
  union, credential-free public HTTPS rule, Git 2.37 minimum, exact reason codes, and trust consent.
  Runtime/Task references co-ship their accumulated public behavior. The safe-detail filter and
  physical Vault/SessionDB identities are internal and need no additional public wording.
- **Web/Docs impact:** Memory removed the inert metrics control; Marketplace exposes the closed source
  union and inline Git grammar. Generated contracts, site/runtime docs, design reference, official
  skill, and QA tracker are aligned. Fresh desktop/narrow screenshots and browser recovery captures
  are recorded in the closeout lab.

## Resume instructions

1. Read this file, the current backlog, the plan, and the active memory ledger.
2. Read `rtk make gate-status`; the completion note after this handoff owns the final gate result.
3. Do not reopen Verified rows or run another Deep Review.
4. Do not reopen `08-F5` or `08-F8`; both are Verified with named permanent evidence.
5. If asked to ship PR #293, commit the current remediation batch, update/push the branch, and wait
   for fresh remote checks. Do not represent the old failing CI run as current-source evidence.
