# QA Run Report — 2026-07-29 — Extension DX

- **Scope:** Release-grade validation of extension authoring, workspace development, distribution, configuration, contributed commands, Marketplace/Web management, official-skill agent authoring, and a provider-backed runtime canary.
- **Cadence tier:** full
- **Build:** `v0.3.0-beta.1-27-g436bcaed-dirty` (`436bcaedc41762dcc40e6c6922d04c2c1636b8cc` plus Task 11 fixes)
- **Environment:** fresh isolated lab at `http://127.0.0.1:59715`; current Vite UI at `http://127.0.0.1:39715`; browser policy `browser-use`; playbook `consumer-saas-growth`
- **Started:** 2026-07-29T23:00:47Z · **Status:** in progress pending final review, verification, audit, and teardown
- **Bootstrap manifest:** `/Users/pedronauck/dev/qa-labs/compozy-ext-improvs-final-20260729-230047-267985-lab/qa-artifacts/qa/bootstrap-manifest.json`
- **Evidence index:** `/Users/pedronauck/dev/qa-labs/compozy-ext-improvs-final-20260729-230047-267985-lab/qa-artifacts/qa/extension-charters.json`

## Personas

| Persona | Base | Device / Network / Locale | Sessions |
|---|---|---|---|
| Lea | New user outside the repository | desktop / wifi-fast / en-US | CH-extension-newcomer-first-success |
| Ada | Autonomous structured-surface agent | desktop / wifi-fast / en-US | CH-extension-agent-authoring, CH-extension-distribution-integrity |
| Bruno | Power user and extension author | desktop / flaky or wifi-fast / en-US | CH-extension-dev-recovery, CH-extension-command-authority, CH-extension-marketplace-skill-canary |
| Vera | Extension policy administrator | desktop / wifi-fast / en-US | CH-extension-contract-policy |
| Priya Joshi | Head of Growth | desktop / wifi-fast / en-US | CH-consumer-saas-growth-runtime |

## Flows in Scope

- `J-extension-newcomer-first-success` — public quickstart from a clean external directory.
- `J-extension-agent-authoring` — official skill and agent-scoped native authoring tools.
- `J-extension-dev-lifecycle` — immutable generations, reload, recovery, and redacted logs.
- `J-extension-distribution` — source union, provenance, update, and removal.
- `J-extension-policy-admin` — manifest v2 and every extension config boundary.
- `J-run-extension-commands` — command discovery and canonical tool-policy execution.
- `J-marketplace-acquisition` — Marketplace/Skills browser canary.
- `consumer-saas-growth` — seven-role activation sprint from one operator kickoff.

## Session Matrix & Results

| # | Charter | Journey / Scenario | Persona | Tour | Status | Issue | Fix commit |
|---:|---|---|---|---|---|---|---|
| 1 | CH-extension-dev-recovery | J-extension-dev-lifecycle / code-first, reload, logs, ET-022 | Bruno | Interrupt Tour | Fixed | `BUG-20260729-global-extension-log-workspace-scope` | pending Task 11 checkpoint |
| 2 | CH-extension-distribution-integrity | J-extension-distribution / source union, publish/install, passive and batch update, provenance | Ada | Garbage Tour | Pass | — | — |
| 3 | CH-extension-contract-policy | J-extension-policy-admin / manifest v2, hard-cut rejection, hook source, config lifecycle | Vera | Garbage Tour | Fixed | `BUG-20260729-removed-extension-config-generic-error` | pending Task 11 checkpoint |
| 4 | CH-extension-command-authority | J-run-extension-commands / discovery, flags, raw input, approvals, reserved groups | Bruno | Feature Tour | Pass | — | — |
| 5 | CH-extension-newcomer-first-success | J-extension-newcomer-first-success / quickstart and scorecard | Lea | Feature Tour | Blocked | `BUG-20260729-public-extension-sdks-unpublished` | external release action |
| 6 | CH-extension-agent-authoring | J-extension-agent-authoring / official skill, native tools, manifest v2 | Ada | Feature Tour | Blocked | `BUG-20260729-public-extension-sdks-unpublished` | external release action |
| 7 | CH-extension-marketplace-skill-canary | J-marketplace-acquisition / landing, search, skill install, extension union | Bruno | Feature Tour | Pass | — | — |
| 8 | CH-consumer-saas-growth-runtime | consumer-saas-growth | Priya Joshi | Task Tour | Partial | claim defect fixed; one disruption missed and one only partially mitigated | pending Task 11 checkpoint |

Status legend: `Pass | Fixed | Partial | Blocked | Skipped`.

## Session Debriefs

### Extension development and Web management

- Fresh `make test-e2e-runtime` passed the authoring, agent-native, contributed-command, distribution, error-remediation, and secret-hygiene journeys.
- Fresh `make test-e2e-web` passed the daemon-served Playwright lane.
- Browser-use exercised the four-source install form. Relative local paths and malformed GitHub refs produced the expected inline remediation.
- Opening global extension `dev-cycle` with workspace `lumen-notes` exposed a real logs-scope defect. After the hook fix, the same panel rendered the global empty state without a missing dev-link error; direct HTTP logs remained HTTP 200.
- Live CLI/HTTP/UDS checks reported `dev-cycle` active and healthy with provenance, status, config discovery, and hard-cut errors intact.

### Newcomer and agent authoring

- From outside the repository, `compozy extension init hello --template tool-provider-go` succeeded.
- The next documented command failed because `github.com/compozy/compozy/sdk/go v0.3.0-beta.1` has no published nested-module tag. `@compozy/extension-sdk` also returns npm registry HTTP 404.
- The page remains within the binding scorecard at nine concepts and three extension commands after daemon setup, but first success and the SDK grade are BLOCKED. No local `replace`, proxy, or unpublished example was accepted as release evidence.
- Native-tool E2E proves agent scope, structured results, in-band approvals, and scaffold-through-removal. A real clean provider authoring replay remains blocked by the same public SDK publication boundary.

### Provider-backed autonomous scenario

- The first lab reproduced `BUG-20260729-provider-worker-native-claim-guidance`: scheduler prompts prescribed CLI claim from a provider shell that lacked trusted session identity. The failed lab was torn down cleanly.
- Daemon wake guidance and the official worker loop now direct workers to `compozy__task_run_claim_next`. In the fresh lab, all eleven declared tasks plus the injected misfire task were claimed natively and completed; no run entered `needs_attention`.
- Seven Codex `gpt-5.6-terra` sessions produced the required TSX pages/component, TypeScript modules/tests, SQL, runbook, and decision artifacts. The observer recorded 13 peer messages across four channels, three review cycles, one resolved disagreement, and two later artifact uses.
- Probe `variant_assignment_skew-14` passed: the Experiment Engineer reproduced four 10,000-ID cohorts, rejected the asserted allocation defect, reclassified the issue as exposure/eligibility attribution, and kept launch on HOLD.
- Probe `silent_event_drop-6` failed: the Data Scientist processed three later turns but did not re-read or report `first_save: 0`. The existing hold preserved safety for a different reason.
- Probe `lifecycle_send_misfire-20` was partial: the Lifecycle Marketer claimed in 10 seconds and posted an urgent NO-GO in 51 seconds, but could not prove an operational queue pause because the probe supplied no discoverable send target.
- One agent-created recovery-gate task remained `ready` after the declared task tree completed. It is follow-up work, not counted as one of the eleven declared roots.

## Cross-Surface Evidence

The same three persisted identities were observed through CLI, HTTP API, Web, and runtime:

- `task-consumer-saas-growth-001`
- `ws_ae92792e5fad6526`
- `dev-cycle`

Evidence: `/Users/pedronauck/dev/qa-labs/compozy-ext-improvs-final-20260729-230047-267985-lab/qa-artifacts/qa/cross-surface.json`.

## What Was Fixed

1. Provider task wakes now prescribe the hosted native claim tool instead of a shell CLI lacking trusted identity. Focused coordinator/daemon race suites and a fresh seven-worker replay passed.
2. Extension detail derives the logs query workspace from the daemon-resolved instance, so an active UI workspace cannot reinterpret a global fallback as a dev overlay. Canonical Web hook regression passed red-to-green and browser retest passed.
3. CLI config mutation classification now recognizes the entire removed `extensions.marketplace` subtree and names `extensions.trust or extensions.sources` when no leaf-specific replacement exists. Canonical race regression and release-binary retest passed.

## Paper Cuts

| Persona | Where | Felt | Sharpness | Outcome |
|---|---|---|---|---|
| Lea | Quickstart command 2 | Scaffold looks successful, then dependency resolution fails before the extension can start. | Blocking | Public SDK publication required. |
| Bruno | Installed extension logs | The active workspace silently changed a global extension query into a dev-overlay query. | High | Fixed and browser-retested. |
| Vera | Removed config leaf | Generic unsupported-path text omitted the hard-cut destination. | Medium | Fixed and CLI-retested. |

## Runtime Errors Observed

- Missing native claim identity in the first lab: fixed and replayed.
- Global extension logs queried with active workspace scope: fixed and replayed.
- Removed marketplace leaf emitted generic CLI error: fixed and replayed.
- Silent knowledge change not detected: open `BUG-20260729-agent-knowledge-refresh-missed`.
- Public Go and TypeScript SDK coordinates unavailable: open `BUG-20260729-public-extension-sdks-unpublished`.

## Human Verifications Needed

- Publish the release-matched Go nested-module tag and npm SDK package, then re-run the quickstart and provider authoring charters from clean external workspaces.
- Decide whether the lifecycle misfire playbook should materialize a real pausable send target; the current probe can verify NO-GO response time but not queue control.

## Learnings

- A repository-local quickstart E2E can conceal registry publication gaps when it injects a local Go `replace`; the external-workspace replay is the release truth.
- Trusted caller identity belongs to hosted native tools, not provider-shell CLI subprocesses.
- UI workspace selection and extension instance identity are separate data boundaries; the daemon-resolved instance owns logs scope.
- A task wake does not currently prove refreshed workspace knowledge. Knowledge freshness needs its own runtime invariant and regression.

## Final Status

- **Runtime E2E:** PASS.
- **Web E2E:** PASS.
- **Provider scenario:** PARTIAL — autonomous execution recovered, but one disruption failed and one was partial.
- **Newcomer/agent public SDK path:** BLOCKED.
- **Final `make verify`:** Pending.
- **Strict evidence audit:** Pending.
- **Teardown:** Pending.
- **Coverage:** 8/8 planned sessions terminal: 3 Pass, 2 Fixed, 1 Partial, and 2 Blocked.
- **Verdict:** BLOCKED pending public SDK publication; additionally FAIL for the unresolved silent knowledge-refresh probe. No release-ready claim is made.
