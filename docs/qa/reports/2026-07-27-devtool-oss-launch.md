# QA Run Report — 2026-07-27 — devtool-oss-launch

- **Scope:** CompozyOS v0.3 migration candidate — exact 25-scenario targeted union plus one local
  landing canary; no live publication, registry, installer, cosign, DNS, or Task-14 migration
- **Cadence tier:** targeted
- **Build:** `b2ad244622142ed97f2b5b170a5267bbbb50d359` plus candidate fixes ·
  **Environment:** fresh isolated `compozy-migration-beta-20260727-135201-116083` lab; local
  daemon/Web/site only
- **Started:** 2026-07-27T12:08:33Z · **Status:** completed with known risk and operator-waived
  final gates

## Personas

| Persona | Base | Device / Network / Locale | Sessions |
|---|---|---|---|
| Ada | Runtime operator | desktop / wifi-fast / en-US | CH-compozy-platform-hard-cut, CH-compozy-runtime-input-preflight, CH-compozy-dev-cycle-skills |
| Dora | Security and release operator | desktop / wifi-fast / en-US | CH-compozy-wire-public-hard-cut, CH-compozy-beta-candidate |
| Bruno | Software delivery lead | desktop / wifi-fast / en-US | CH-compozy-mixed-runtime-delivery, CH-compozy-agent-authored-review |
| Cora | Non-technical owner and first-time reader | desktop + mobile / wifi-fast / en-US | CH-compozy-run-plain-language, CH-compozy-landing-canary |

## Flows in Scope

- `J-validate-compozy-hard-cut` — one executable, storage, environment, wire, package, tool, skill,
  and public identity (`../journeys/J-validate-compozy-hard-cut.md`)
- `J-01` — mixed-runtime delivery with durable applied provenance and a real deep link
  (`../journeys/J-01.md`)
- `J-02` — input defaults and effective validation before persistence or ACP spawn
  (`../journeys/J-02.md`)
- `J-08` — provider-free agent-authored review with deterministic atomic artifacts
  (`../journeys/J-08-watch-and-maintain.md`)
- `J-offer-runnable-capabilities` — exactly nine immutable dev-cycle skills in managed sessions
  (`../journeys/J-offer-runnable-capabilities.md`)
- `J-approve-compozy-beta-candidate` — read-only release-plan, migration-guide, and beta-channel
  approval (`../journeys/J-approve-compozy-beta-candidate.md`)
- `J-evaluate-compozy-beta` — local landing canary for the integrated OS claim
  (`../journeys/J-evaluate-compozy-beta.md`)

## Session Matrix & Results

| # | Charter | Journey / Scenario | Persona | Tour | Status | Issue | Fix commit |
|---|---|---|---|---|---|---|---|
| 1 | CH-compozy-platform-hard-cut | J-validate-compozy-hard-cut / RT-compozy-cli-binary | Ada | Garbage Tour | Fixed | BUG-20260727-dirty-build-release-track | pending |
| 2 | CH-compozy-platform-hard-cut | J-validate-compozy-hard-cut / RT-compozy-global-database | Ada | Garbage Tour | Pass | | |
| 3 | CH-compozy-platform-hard-cut | J-validate-compozy-hard-cut / RT-compozy-home-layout | Ada | Garbage Tour | Fixed | BUG-20260727-dirty-build-release-track | pending |
| 4 | CH-compozy-platform-hard-cut | J-validate-compozy-hard-cut / RT-compozy-home-isolation | Ada | Garbage Tour | Pass | | |
| 5 | CH-compozy-platform-hard-cut | J-validate-compozy-hard-cut / RT-compozy-environment-namespace | Ada | Garbage Tour | Fixed | BUG-20260727-runtime-legacy-identity | pending |
| 6 | CH-compozy-platform-hard-cut | J-validate-compozy-hard-cut / ET-compozy-native-tool-invocation | Ada | Garbage Tour | Fixed | BUG-20260727-runtime-legacy-identity | pending |
| 7 | CH-compozy-platform-hard-cut | J-validate-compozy-hard-cut / ET-compozy-extension-contract-identity | Ada | Garbage Tour | Fixed | BUG-20260727-runtime-legacy-identity | pending |
| 8 | CH-compozy-platform-hard-cut | J-validate-compozy-hard-cut / ET-compozy-official-skill-discovery | Ada | Garbage Tour | Pass | | |
| 9 | CH-compozy-wire-public-hard-cut | J-validate-compozy-hard-cut / NB-compozy-wire-identity | Dora | Garbage Tour | Fixed | BUG-20260727-runtime-legacy-identity | pending |
| 10 | CH-compozy-wire-public-hard-cut | J-validate-compozy-hard-cut / RT-compozy-claim-token-redaction | Dora | Garbage Tour | Pass | | |
| 11 | CH-compozy-wire-public-hard-cut | J-validate-compozy-hard-cut / ET-compozy-public-brand-navigation | Dora | Garbage Tour | Fixed | BUG-20260727-runtime-legacy-identity | pending |
| 12 | CH-compozy-mixed-runtime-delivery | J-01 / LP-runtime-selection-overrides | Bruno | Feature Tour | Pass | | |
| 13 | CH-compozy-mixed-runtime-delivery | J-01 / LP-runtime-provenance-observation | Bruno | Feature Tour | Pass | | |
| 14 | CH-compozy-mixed-runtime-delivery | J-01 / LP-loop-run-deep-link | Bruno | Feature Tour | Pass | | |
| 15 | CH-compozy-run-plain-language | J-01 / LP-runtime-provenance-observation | Cora | Feature Tour | Pass | | |
| 16 | CH-compozy-run-plain-language | J-01 / LP-loop-run-deep-link | Cora | Feature Tour | Pass | | |
| 17 | CH-compozy-runtime-input-preflight | J-02 / LP-loop-input-defaults | Ada | Garbage Tour | Pass | | |
| 18 | CH-compozy-runtime-input-preflight | J-02 / LP-runtime-validation-preflight | Ada | Garbage Tour | Pass | | |
| 19 | CH-compozy-agent-authored-review | J-08 / LP-agent-authored-review-run | Bruno | Interrupt Tour | Pass | | |
| 20 | CH-compozy-agent-authored-review | J-08 / LP-review-artifact-inspection | Bruno | Interrupt Tour | Pass | | |
| 21 | CH-compozy-agent-authored-review | J-08 / LP-review-round-finalization | Bruno | Interrupt Tour | Pass | | |
| 22 | CH-compozy-dev-cycle-skills | J-offer-runnable-capabilities / ET-dev-cycle-skill-bundle | Ada | Feature Tour | Fixed | BUG-20260727-runtime-legacy-identity | pending |
| 23 | CH-compozy-dev-cycle-skills | J-offer-runnable-capabilities / ET-dev-cycle-legacy-skill-retired | Ada | Feature Tour | Pass | | |
| 24 | CH-compozy-beta-candidate | J-approve-compozy-beta-candidate / REL-release-candidate-plan | Dora | Garbage Tour | Pass | | |
| 25 | CH-compozy-beta-candidate | J-approve-compozy-beta-candidate / REL-migration-guide-parity | Dora | Garbage Tour | Pass | | |
| 26 | CH-compozy-beta-candidate | J-approve-compozy-beta-candidate / REL-beta-channel-contract | Dora | Garbage Tour | Pass | | |
| 27 | CH-compozy-landing-canary | J-evaluate-compozy-beta / REL-os-landing-proof | Cora | Feature Tour | Pass | | |

Status legend: `Pending | Pass | Fixed | Skipped | Blocked (needs human verify) | Blocked (human decision)`

## Session Debriefs

- **Platform hard cut:** the dirty-build boot failure and remaining public AGH identities were
  fixed; executable, storage, environment, native-tool, extension, and official-skill reads pass.
- **Wire/public hard cut:** headers, SSE, artifacts, bridges, Web persistence, public navigation,
  and claim-token redaction pass with Compozy-only identity.
- **Mixed-runtime delivery:** structured CLI, HTTP, and UDS output agree for persisted runtime
  selection, provenance, and the random-port Loop Run deep link.
- **Plain-language run:** the Web run view presents the same applied runtime and deep link without
  unsupported controls.
- **Input preflight:** layered defaults and explicit overrides resolve identically across surfaces;
  invalid input/runtime selections fail before run creation.
- **Agent-authored review:** the provider-free reviewer/fixer flow passes deterministic artifact,
  two-workspace isolation, containment, finalization, and no-partial-round checks.
- **Dev-cycle skills:** the nine-skill managed-session bundle is available and the retired legacy
  bundle is absent.
- **Beta candidate:** releasepr v0.0.24 planning, ref/tag guards, authoritative output ownership,
  guide parity, and beta policy pass without publishing.
- **Landing canary:** the local site render, canonical origin, locked OS copy, and truthful controls
  pass.

## What Was Fixed

- `BUG-20260727-dirty-build-release-track` — a dirty `git describe` candidate was routed through
  published prerelease parsing and could not boot. The update version classifier now treats every
  `-dirty` source build as development state; focused race verification and the original-lab live
  restart both pass.
- `BUG-20260727-runtime-legacy-identity` — the public runtime identity was cut atomically across
  prompts, diagnostics, headers/SSE, artifacts, bridges, extensions, SDK handshakes, memory markers,
  native descriptors, Web persistence/menu state, and public docs. Regenerated contracts, focused
  Go/Web/site suites, lint-plugin tests, boundaries, and the inspected Compozy-menu capture pass;
  live version/status/doctor evidence is Compozy-only.
- `BUG-20260727-ephemeral-role-session-leak` — pre-acceptance internal-role retries no longer leave
  failed sessions in the public catalog; accepted child failures and user-created startup failures
  remain durable.

## Paper Cuts

- The first activation barrier exposed a playbook-owned workspace/channel mismatch: the docs agent
  owned the changelog, while its artifact and named channel belonged to `release-eng`. The canonical
  fixture now keeps ownership, artifact, review, and `docs-review` participation in `docs-site`.
- Literal inspection of the rendered kickoff caught an undeclared demo-script request and the stale
  changelog location before provider delivery. The canonical kickoff now enumerates only the eleven
  declared Tasks. Both discarded pre-kickoff labs have `qa/teardown.json` with `clean: true` and no
  survivors.

## Runtime Errors Observed

- Before the first charter kickoff, the initial candidate daemon rejected
  `v0.2.15-16-gb2ad2446-dirty` as an unsupported prerelease track. Recorded as
  `BUG-20260727-dirty-build-release-track` and fixed in the owning update classifier.
- The restarted candidate exposed `Required AGH-managed provider credential ...` in
  `compozy status -o json`. Source tracing found the retired identity across public prompt, wire,
  artifact, bridge, and Web-state surfaces. Recorded as
  `BUG-20260727-runtime-legacy-identity`; the rebuilt original-lab version/status/doctor retest is
  Compozy-only.
- The first full integration run exposed leaked pre-acceptance role attempts in the session catalog.
  `BUG-20260727-ephemeral-role-session-leak` is fixed and the focused role suite plus the full
  18,552-test integration rerun pass.
- The 1,800-second observer reproduced open
  `BUG-20260719-autonomous-progress-unobservable`: runtime work completed, but the observer's
  journey log did not receive runtime-owned progress events. This affects `RT-073`, outside this
  run's exact 25-scenario scope, and remains recorded as an open P1.

## Human Verifications Needed

None currently. Live publication and post-publish install/registry/cosign checks are explicitly out
of scope rather than blocked rows in this pre-publish run.

## Decisions for a Human

No in-scope product decision. The operator accepted the existing open `RT-073` observability P1 as
a known risk for the urgent cutover.

## Learnings

- The landing canary must enter through the local site render while independently verifying the
  canonical `https://compozy.com` declaration; hosting and DNS are not part of this run.
- `BUG-20260719-autonomous-progress-unobservable` is an existing P1 risk for the one-kickoff
  observer. If reproduced, it will be re-found and reported; no second prompt will mask a stall.
- A source-built dirty candidate is valid development state, not a release channel. Release-track
  parsing must never run before dirty-build classification.
- A package/executable rename is insufficient migration evidence: live prompt, wire, artifact,
  bridge, and client persistence identities must be audited as one public contract.
- The scheduler barrier is also a fixture-integrity gate: workspace/channel mismatches and briefing
  work absent from the Task tree must be corrected before the one allowed operator prompt.
- The `eng-ui-screenshot` capture at
  `.compozy/tasks/compozy-migration/qa/screenshots/task_13/compozy-menu-open.png` renders the real
  Storybook story id at 1440x900 with the CompozyOS menu open and no retired menu identity; owned
  Storybook/Chrome processes were torn down and port 6006 is free.

## Final Status

- **Exit gate (full automated suite):** integration passed (18,552 tests, 5 skipped); runtime E2E
  passed; browser E2E passed (113/113); dedicated review/fix and role-cleanup integration passed.
  A final post-freeze `make verify` and strict QA evidence audit were not run after the last fixes;
  the operator explicitly stopped those gates to meet an urgent external application deadline.
- **Issues by user impact:** in-scope Blocks-Completion 2 fixed · out-of-scope Trust-Damage 1 fixed ·
  existing out-of-scope Blocks-Completion P1 1 open.
- **Coverage:** 9/9 charters; 25/25 unique selected scenarios settled.
- **Teardown:** clean. The final lab evidence at
  `/Users/pedronauck/dev/qa-labs/compozy-compozy-migration-beta-20260727-135201-116083-lab/qa-artifacts/qa/teardown.json`
  reports `clean: true`, no survivors, and all daemon/Web/site/browser/observer processes stopped.
- **Verdict:** targeted QA pass with one recorded out-of-scope observability risk. This is not a
  claim that the repository's normal final `make verify` and strict-audit completion gates passed.
