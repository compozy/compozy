# QA Run Report — 2026-08-25 — skill-sources

- **Scope:** `.compozy/tasks/skill-sources`, tasks 01–09
- **Cadence:** targeted release-grade run: five changed journeys, ride-alongs, and the managed-session canary
- **Build:** `df739b0` on branch `skills-source`, rebased onto `origin/main`
- **Environment:** isolated lab `compozy-skill-sources-final-rebased-20260825-20260825-230120-931206-lab`
- **Started:** 2026-08-25T23:01:20Z
- **Status:** frozen for the content-addressed exit gate
- **Current verdict:** PASS only when the external final-gate log records exit 0 and both strict
  audit reports record PASS; otherwise BLOCKED

## Evidence Index

- Bootstrap manifest: `/Users/pedronauck/dev/qa-labs/compozy-skill-sources-final-rebased-20260825-20260825-230120-931206-lab/qa-artifacts/qa/bootstrap-manifest.json`
- Smoke evidence: `/Users/pedronauck/dev/qa-labs/compozy-skill-sources-final-rebased-20260825-20260825-230120-931206-lab/qa-artifacts/qa/install.json`
- Behavioral evidence: `/Users/pedronauck/dev/qa-labs/compozy-skill-sources-final-rebased-20260825-20260825-230120-931206-lab/qa-artifacts/qa/skill-sources/validation-summary.json`
- Runtime observation: `/Users/pedronauck/dev/qa-labs/compozy-skill-sources-final-rebased-20260825-20260825-230120-931206-lab/qa-artifacts/qa/observation-summary.json`
- Public-surface captures: `/Users/pedronauck/dev/qa-labs/compozy-skill-sources-final-rebased-20260825-20260825-230120-931206-lab/qa-artifacts/qa/skill-sources`
- Browser artifacts: `/Users/pedronauck/dev/qa-labs/compozy-skill-sources-final-rebased-20260825-20260825-230120-931206-lab/qa-artifacts/qa/browser-e2e`
- Local screenshots for Wrangler upload: `docs/qa/evidence/2026-08-25-skill-sources/`
- Browser-use recording: `/Users/pedronauck/.config/browser-harness/agent-workspace/recordings/skill-sources-final-qa`
- Browser lab teardown: `/Users/pedronauck/dev/qa-labs/compozy-skill-sources-browser-final-20260825-20260826-021903-446795-lab/qa-artifacts/qa/teardown.json` — `clean: true`, zero survivors
- Final verify: `/Users/pedronauck/dev/qa-labs/compozy-skill-sources-final-rebased-20260825-20260825-230120-931206-lab/qa-artifacts/qa/final-make-verify.log`
- Teardown: `/Users/pedronauck/dev/qa-labs/compozy-skill-sources-final-rebased-20260825-20260825-230120-931206-lab/qa-artifacts/qa/teardown.json` — `clean: true`, zero survivors

The lab used a non-default `COMPOZY_HOME`, port 60195, a lab-scoped UDS socket, a lab-scoped
`tmux-bridge` socket, and manifest-derived provider homes. Browser proxy configuration came from
`COMPOZY_WEB_API_PROXY_TARGET`; no default runtime state or hard-coded daemon port was used.

## Personas and Journeys

| Persona | Journeys |
|---|---|
| Dora — Runtime Administrator | absorb existing skill sources; diagnose source truth; manage scope policy |
| Bruno — Delivery Builder | share skills with other tools; open a teammate repository; manage installed skills |
| Théo — Returning Session User | use absorbed skills and the slash-command catalog |
| Ada — Autonomous Agent | operate sources headlessly; invoke native tools; load a skill in a managed session |
| Mateo Rivera — Helix CLI founder | one in-persona real-scenario kickoff and autonomous runtime observation |

## Session Matrix

| # | Charter | Scenario | Status | Finding or evidence |
|---|---|---|---|---|
| 1 | CH-skill-sources-live-apply | ET-manage-skill-source-policy | Fixed | profile override and workspace-field bugs verified |
| 2 | CH-skill-sources-live-apply | ET-live-skill-source-reload | Fixed | live generation and complete applied-root ledger verified |
| 3 | CH-skill-sources-live-apply | ET-skill-origin-attribution | Fixed | registered workspace id accepted across detail projections |
| 4 | CH-skill-expose-lifecycle-trust | ET-skill-exposure-lifecycle | Fixed | create, repeat, remove, partial result, and foreign conflict verified |
| 5 | CH-skill-session-suppression-matrix | ET-skill-session-source-injection | Pass | provider-aware catalog and explicit invocation verified |
| 6 | CH-skill-session-suppression-matrix | ET-session-command-catalog-parity | Pass | CLI, HTTP, and UDS catalogs agree |
| 7 | CH-skill-session-suppression-matrix | ET-session-composer-skill-chip | Pass | absorbed and native command rows remain distinct |
| 8 | CH-skill-sources-diagnostics-truth | ET-skill-source-diagnostics-cli | Fixed | deterministic validation, roots, and collisions verified |
| 9 | CH-skill-sources-diagnostics-truth | ET-skill-source-symlink-containment | Pass | linked-entry containment and skipped-link diagnostics verified |
| 10 | CH-skill-sources-diagnostics-truth | ET-skill-ecosystem-frontmatter-quiet | Pass | ecosystem frontmatter loads without warning floods |
| 11 | CH-skill-sources-settings-web | ET-web-skill-sources-settings | Fixed | configured path now joins canonical measurement |
| 12 | CH-skill-expose-web-repair | ET-web-skill-expose-panel | Fixed | picker render, repair, and foreign-entry protection verified |
| 13 | CH-skill-expose-web-repair | ET-web-marketplace-installed-management | Pass | installed detail and management survive reload |
| 14 | CH-skill-expose-web-repair | ET-web-marketplace-skill-install | Pass | Marketplace acquisition ride-along remains green |
| 15 | CH-skill-sources-agent-plane | ET-skill-source-agent-parity | Fixed | structured CLI, HTTP, UDS, and native-tool reads agree |
| 16 | CH-skill-sources-agent-plane | ET-skill-source-observe-ledger | Fixed | every effective root is present in the durable event |
| 17 | CH-skill-sources-agent-plane | ET-compozy-native-tool-invocation | Pass | hosted skill list and view descriptors are invocable |
| 18 | CH-skill-sources-agent-plane | ET-compozy-official-skill-discovery | Fixed | official skill policy and workspace projection agree with runtime |
| 19 | CH-skill-sources-repo-teammate | ET-workspace-skill-source-teammate | Fixed | committed native skill projects under the registered workspace id |
| 20 | CH-skill-sources-managed-session-canary | ET-managed-session-skill-loading | Pass | global playbook agent can load the omitted skill |

Zero in-scope rows remain Pending, Skipped, Fail, or Blocked. Scenario frontmatter contains the
latest `pass` verdict, evidence paths, and this report path.

## Real-Scenario Runtime

Mateo posted one operator kickoff. The scheduler barrier released 11 declared tasks together; the
observer reached `outcome: all_terminal`, reported no stall, and found the independent task catalog
identical to task detail reads. No follow-up prompt was sent to an agent under test.

| Check | Status | Evidence |
|---|---|---|
| One in-persona kickoff | Pass | `/Users/pedronauck/dev/qa-labs/compozy-skill-sources-final-rebased-20260825-20260825-230120-931206-lab/qa-artifacts/qa/kickoff-confirmation.json` |
| Eleven tasks released behind the barrier | Pass | `/Users/pedronauck/dev/qa-labs/compozy-skill-sources-final-rebased-20260825-20260825-230120-931206-lab/qa-artifacts/qa/task-release.json` |
| Observation reaches all terminal without a stall | Pass | `/Users/pedronauck/dev/qa-labs/compozy-skill-sources-final-rebased-20260825-20260825-230120-931206-lab/qa-artifacts/qa/observation-summary.json` |
| Catalog and task-detail parity | Pass | `/Users/pedronauck/dev/qa-labs/compozy-skill-sources-final-rebased-20260825-20260825-230120-931206-lab/qa-artifacts/qa/task-catalog-independent.json` |
| Strict evidence audit | Pending | runs after the final gate is recorded |

## Public-Surface Results

- User, profile, workspace, and read-only agent policy were exercised through fresh CLI reads plus
  HTTP and UDS settings routes. Unknown sources, duplicates, invalid relative paths, and agent-scope
  writes returned deterministic structured errors.
- Live apply changed the current generation without restarting the daemon. Source diagnostics,
  installed skill catalog, and session command catalog agreed after the change.
- CLI, HTTP, UDS, and native tools agreed on origin, owner scope, workspace identity, and exposure
  state. The durable `skills.sources.applied` event listed every effective preset and custom root.
- Expose created readable provider links, treated repeats idempotently, retained a missing-link
  record for repair, rolled back a partial failure accurately, and offered no destructive action for
  a foreign entry.
- Workspace-native and absorbed skills remained separately addressable. The composer showed a
  neutral origin only for absorbed commands.

Primary evidence is under `/Users/pedronauck/dev/qa-labs/compozy-skill-sources-final-rebased-20260825-20260825-230120-931206-lab/qa-artifacts/qa/skill-sources`; the scenario files link the exact JSON captures
used for each verdict.

## Browser Results

Focused production-bundle replay after fixes:

| Flow | Status | Result |
|---|---|---|
| E2E-008 custom folder | Pass | measured count appears, duplicate refuses, removal succeeds |
| E2E-010 command origin | Pass | exact absorbed token shows `agents`; native token stays unlabeled |
| E2E-011 exposure lifecycle | Pass | active, missing, repaired, and foreign-conflict states verified |
| Skills HTTP/UDS/CLI parity | Pass | strict detail query and current `skill info` command agree |
| Full daemon-served Web lane | Pass | 256 total: 253 passed, 3 intentional skips, 0 unexpected, 0 flaky |

The first full Web lane exposed three product defects and two stale test assumptions. Product
defects were registered and fixed. Test-only drift was corrected from `skill inspect` to
`skill info`, from the legacy detail `workspace` query to `workspace_id`, and from ambiguous
visible text to the exact command token. The post-fix 256-test official lane passed serially.

The direct browser-use walk reached the real Settings route but the shell remained in its existing
cross-tab window-sync retry state, already tracked by `BUG-20260729-session-window-cross-tab-focus`.
The attempt was recorded rather than duplicated as a new bug. The required product workflows were
then verified through the production daemon-served Playwright surface, and both labs ended with
`clean: true` teardown evidence.

## Bugs Found and Fixed

| Bug | Impact | Status | Regression proof |
|---|---|---|---|
| BUG-20260825-skill-source-profile-write-rejected | Blocks-Completion | Verified | settings core source-policy suite |
| BUG-20260825-workspace-skills-non-source-field-written | Blocks-Completion | Verified | native config and scope-validation suites |
| BUG-20260825-skill-detail-rejects-workspace-id | Blocks-Completion | Verified | skill detail registered-id suite |
| BUG-20260825-skill-source-event-omits-custom-roots | Blocks-Completion | Verified | settings diagnostics and generation fence |
| BUG-20260825-skill-source-agent-write-doc-mismatch | Trust-Damage | Verified | native config classifier plus documented replay |
| BUG-20260825-custom-source-stuck-pending | Trust-Damage | Verified | settings diagnostics plus E2E-008 |
| BUG-20260825-workspace-native-skill-missing | Blocks-Completion | Verified | registry root scope plus E2E-011 |
| BUG-20260825-expose-picker-crashes | Blocks-Completion | Verified | production-bundle E2E-011 |
| BUG-20260826-session-delete-return-race | Trust-Damage | Verified | session controller units plus full Web E2E |
| BUG-20260826-namespaced-skill-label-collapses | Trust-Damage | Verified | command projection units plus E2E-010 |

All fixes were small, root-caused, contained, and had no product trade-off. Each impacted and
adjacent flow was replayed from a fresh runtime.

## Automated Lanes

| Lane | Status | Result |
|---|---|---|
| Focused Go race tests | Pass | settings diagnostics and workspace resource projection |
| Root Turbo Web lint/typecheck/build | Pass | zero warnings and zero errors |
| Root Turbo Web unit suite | Pass | 719 files, 6,346 tests |
| `make test-e2e-runtime` | Pass | fresh full rerun completed at exit 0 after the competing external lane ended |
| `make test-e2e-web` | Pass | 256 total: 253 passed, 3 skipped, 0 unexpected, 0 flaky |
| `make gate-full` | Pending | runs once after the last mutation |

The first runtime attempt overlapped a full Bun suite in another checkout and timed out one loop-read
case. That exact case passed under `-race` in isolation; after the competing process ended, a fresh
unchanged full runtime target passed at exit 0. No retry, timeout, or assertion was added or weakened.

## Production-Parity Notes

- The scenario lab and browser E2E used a locally built production bundle and real daemon storage.
- ACP behavior used the repository's deterministic ACP mock driver at the unit-test I/O boundary.
- The real-scenario playbook used provider-backed managed sessions and the isolated provider home
  emitted by bootstrap.
- Tests were limited to desktop, wifi-fast, and en-US; responsive Marketplace captures cover
  desktop, tablet, and mobile viewports.

## Decision for a Human

### Workspace-created agents are listed but cannot start sessions

`BUG-20260825-workspace-agent-unusable-for-sessions` remains open outside the skill-sources change.
The catalog advertises a workspace-profile agent that `session new` rejects. The playbook continued
with global agents, so the managed-skill canary passed, but this separate workspace-agent journey
still needs a product decision about profile-aware session resolution. The bug file records options
and recommends making session resolution consume the same workspace-profile layer the agent catalog
publishes.

## Compozy Impact Audit

- **Native tools:** skill list/search/view descriptors and schema digests were exercised; no further
  tool id, risk flag, or capability-gate change was needed during QA.
- **Extensibility and hooks:** resource projection now publishes and looks up workspace skills with
  the same registered workspace id. Exposure links, event hooks, and config lifecycle were replayed;
  no extension or hook contract changed in the QA fixes.
- **Workspace data isolation:** skill catalog records are workspace-scoped. Registered workspace id
  now survives publication, cache lookup, HTTP/UDS reads, Marketplace queries, and session command
  projection without cross-workspace visibility.
- **Official Compozy skill:** configuration policy text was corrected and discovered through the
  bundled skill. No public command or tool id changed in the QA fixes.

## Final Status

EVIDENCE-CONDITIONAL PASS — this report is frozen before the content-fingerprinted close gate so the
gate remains current after it runs. The verdict is PASS exactly when
`qa-artifacts/qa/final-make-verify.log` records `make gate-full` exit 0 and the primary plus browser
`qa-audit-report.json` files record `PASS`; a missing/non-zero gate or either non-PASS audit makes
the verdict BLOCKED. All other conditions are already satisfied: every scenario is terminal, every
registered skill-sources defect is verified, both official E2E lanes are green, and both labs have
clean teardown evidence.
