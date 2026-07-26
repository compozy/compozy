# QA Run Report — 2026-07-15 — Marketplace

- **Scope:** Marketplace catalog, four-kind acquisition, MCP authorization and repair, extension trust policy, structured agent parity, adjacent provider-catalog canary, and the release-grade Northstar Pay scenario delivered by `.compozy/tasks/marketplace`.
- **Cadence tier:** Targeted release gate with one adjacent canary and one autonomous real-provider scenario.
- **Build:** Source-freeze tree `55aa6b498297ad483d89d355b3febce60e6d7f68` from detached verification commit `2d129da2809b0edacb9624c317fd9bd54beb7d55` (parent `282366f84`).
- **Execution window:** 2026-07-15–2026-07-16.
- **Primary environment:** Fresh isolated lab `agh-marketplace-task11-final-20260715-20260716-011529-818379-lab`; daemon `http://127.0.0.1:54865`; Web proxy `AGH_WEB_API_PROXY_TARGET=http://127.0.0.1:54865`.
- **Autonomous environment:** Fresh isolated lab `agh-marketplace-northstar-capacity-final-20260715-20260716-001326-274237-lab`; exactly one real-provider kickoff, 30-minute observation, clean teardown.
- **QA verdict:** **PASS after fixes**. The full source-freeze `make verify`, 42-finding review remediation, strict Northstar seal, and Phase D SHIP closure are complete.

## Personas

| Persona | Base | Device / Network / Locale | Sessions |
|---|---|---|---|
| Vera | Marketplace policy owner | desktop / wifi-fast / en-US | CH-extension-policy-admin-gates |
| Bruno | Mid-session operator | desktop / wifi-fast / en-US | CH-mcp-authorize-repair-truth, CH-marketplace-under-a-minute |
| Iris | Remote operator away from the daemon host | laptop / wifi-slow / en-US | CH-remote-operator-manual-auth |
| Ada | Agent-plane operator without Web, TTY, or browser | desktop / wifi-fast / en-US | CH-agent-marketplace-parity |
| Marina | Adjacent-surface reviewer | phone-large / 4g / en-US | CH-033 |
| Sofia Mendes | Founder / Product Manager | desktop / isolated real-provider home / en-US | Northstar Pay one-kickoff scenario |

## Flows in Scope

- `J-marketplace-acquisition` — acquire one capability of each kind born-valid and prove its truthful management state.
- `J-mcp-authorize-repair` — repair OAuth interactively or manually without false green, token loss, or secret echo.
- `J-extension-policy-admin` — administer trust policy, digest verification, live config, and the catalog kill-switch.
- `J-agent-marketplace-parity` — prove CLI, HTTP, UDS, and registered native discovery agree for the same daemon state.
- `J-22` — canary the provider/model-catalog projection and unchanged display surfaces.
- `northstar-pay` — prove one-kickoff collaboration and serial scheduler-capacity policy with real providers.

## Session Matrix & Results

| # | Charter | Journey / Scenarios | Persona | Tour | Status | Corrected defects |
|---|---|---|---|---|---|---|
| 1 | CH-extension-policy-admin-gates | J-extension-policy-admin / 12 | Vera | Garbage Tour | Fixed | live config reachability, trust-root policy, stale truth, digest and kill-switch management |
| 2 | CH-mcp-authorize-repair-truth | J-mcp-authorize-repair / 4 | Bruno | Interrupt Tour | Fixed | OAuth name encoding, null install payloads, extension projection, confirmed-state truth |
| 3 | CH-remote-operator-manual-auth | J-mcp-authorize-repair / 4 | Iris | Paste Tour | Fixed | secret TTY echo and non-loopback callback handling |
| 4 | CH-agent-marketplace-parity | J-agent-marketplace-parity / 16 | Ada | Feature Tour | Fixed | native extension parity, agent-manageable catalog timing, deterministic reachability |
| 5 | CH-marketplace-under-a-minute | J-marketplace-acquisition / 12 | Bruno | Money Tour | Fixed | focus visibility, Vault-ref casing, stale projection, management lifecycle truth |
| 6 | CH-033 | J-22 / 2 | Marina | Back-Button Tour | Fixed | unchanged builtin provider overlay and false restart requirement |

All 50 scenario files referenced by these six charters parse and now carry `qa_status: pass`. No charter has an unexplained, skipped, blocked, or untested row.

## PRD Anchor Capture

### Time to acquire

| Kind | Start | True end | Elapsed | Target | Verdict | Evidence |
|---|---|---|---:|---:|---|---|
| Skill | 2026-07-16T02:27:10.300Z | installed and truthful management detail visible | 9s | < 60s | Pass | `qa-artifacts/qa/web/marketplace-skill-under-minute.png` |
| Extension | 2026-07-16T02:27:31.300Z | installed with official verified provenance | 9s | < 60s | Pass | `qa-artifacts/qa/web/marketplace-extension-under-minute.png` |
| Bundle | 2026-07-16T02:27:51.300Z | previewed activation visible in the workspace | 15s | < 60s | Pass | `qa-artifacts/qa/web/marketplace-bundle-under-minute.png` |
| MCP | 2026-07-16T02:28:12.300Z | structurally valid server visible on `/mcp` | 31s | < 60s | Pass | `qa-artifacts/qa/web/marketplace-mcp-under-minute.png` |

The machine-readable timing record is `qa-artifacts/qa/notes/marketplace-under-minute.json` in the primary lab.

### Born-valid and truthful readiness

| Probe | Observed result | Verdict | Evidence |
|---|---|---|---|
| Invalid MCP install | Missing required values, dangling refs, and locked-template overrides return deterministic rejection; fresh config, Vault, and provenance reads show zero residue. | Pass | `qa-artifacts/qa/notes/marketplace-agent-parity-final.json` |
| OAuth interruption / failed re-auth | Live cancel, supersession, malformed/manual failures, and failed re-auth preserve the prior token and public status. The five-minute expiry branch is additionally owned by the canonical OAuth session suite. | Pass | parity note plus `mcp-oauth-waiting.png` and `mcp-oauth-confirmed.png` |
| OAuth confirmed success | Success renders only after the scoped read reports `authenticated=true` and `token_present=true`; a successful tools probe alone never creates a green state. | Pass | `qa-artifacts/qa/notes/mcp-oauth-name-segment.json` |
| Workspace isolation | Two workspaces using the same server name retain distinct scoped OAuth tokens and canonical `secret_env` references across CLI, HTTP, and UDS. | Pass | `qa-artifacts/qa/notes/marketplace-agent-parity-final.json` |
| Extension trust gates | Tampered archives fail before extraction with no residue; clean official archives preserve distinct catalog/archive/tree hashes and verified provenance. Unverified installs still require policy plus request consent. | Pass | `marketplace-management-lifecycle.json` and policy screenshots |
| Kill-switch / stale truth | A pulled feed entry disappears from discovery while the installed item stays manageable; a failed refresh keeps the prior projection visibly stale instead of pruning it. | Pass | `marketplace-kill-switch.json` and `marketplace-skill-stale-served.png` |
| Secret redaction | Fixture codes/tokens, pasted callback data, verifiers, and typed secret values do not appear in structured output, UI, logs, or events; only ref identifiers remain. | Pass | `mcp-manual-tty-redaction.json`, `mcp-editor-vault-ref-case.json`, and final redaction scans |

## Session Debriefs

### CH-extension-policy-admin-gates

The clean curated archive installed with verified provenance; swapped bytes failed before extraction and wrote no registry or filesystem residue. Default policy blocked unverified acquisition, the live policy flip exposed the second consent gate without a restart, and restoring `allow_unverified=false` blocked the fixture again. Catalog `base_url`, `ttl`, and `timeout` now validate at their owning trust boundaries and apply live where allowed. Bundle conflict, deactivation, extension disable/enable/remove/reinstall, canonical reactivation, pulled-entry removal, and stale-serving management all stayed truthful.

### CH-mcp-authorize-repair-truth

The Web matrix kept configured, authorization, runtime, and probe status independent. Browser PKCE, closing and superseding attempts, deliberate failure against an existing credential, and refetch-confirmed success preserved the prior working token until a new credential was actually confirmed. A server name with spaces exposed the Vault owner-path bug and passed after collision-safe segment encoding. The five-minute expiry boundary was not delayed live solely to wait out the clock; its exact invalidation/preservation contract passed in the canonical session tests while the representative interruption branches were exercised against the live daemon.

### CH-remote-operator-manual-auth

A daemon bound to `0.0.0.0` rejected automatic callback completion with HTTP 403 and manual-exchange guidance without reflecting code or state. Bare-code and full-redirect completion remained available through the copyable URL/manual lane. Real PTY replay exposed pasted callback echo; hidden TTY input fixed it while line-oriented piped stdin remained scriptable. Malformed, mismatched, stale, and failed exchanges remained deterministic and preserved an existing credential.

### CH-agent-marketplace-parity

CLI JSON, HTTP, raw UDS, and `agh__marketplace_search` returned fixed `mcp / extension / skill / bundle` grouping and matching discovery fields. Lifecycle reads and mutations matched across the supported CLI/HTTP/UDS planes; the registered native tool remained intentionally read-only for Marketplace discovery. Unknown inputs produced deterministic 400/404 results, deleted legacy routes stayed 404, rejected installs wrote nothing, two-workspace OAuth stayed isolated, and bundle preview/activate/update cleared induced drift consistently.

### CH-marketplace-under-a-minute

All four capability kinds reached their truthful end state in 9/9/15/31 seconds. Fresh reads agreed with the management surfaces after acquisition. The MCP editor preserved exact byte-sensitive Vault refs, stale catalog sections kept unaffected kinds usable, and the extension/bundle management lifecycle survived reload. Keyboard navigation exposed the existing low-contrast focus bug; the shared UI owner now provides a 2px contrast-safe focus treatment across UI, Web, and site with lint and rendered evidence.

### CH-033

At 430×932, the builtin Claude projection and model source remained stable through inspector navigation, Back, and refresh. An unchanged save initially wrote a false overlay and claimed restart work. The corrected normalized no-op leaves config bytes identical, emits no `settings.changed`, performs no runtime apply/model reprojection, reports immediate application, and preserves source `BUILTIN`.

## Automated and Autonomous Gates

- `make test-e2e-runtime`: passed against fresh runtime fixtures.
- `make test-e2e-web`: 78/78 Playwright journeys passed; `.tmp/playwright/test-results/.last-run.json` reports `status: passed` with no failed ids.
- Focus contract: lint plugin 32/32; `make codegen-check`; UI 532 tests; Web 3,599 tests; site 247 tests; zero obsolete token aliases or low-contrast 1px focus patterns in the three frontend surfaces.
- Northstar Pay capacity retest: all 12 root tasks completed after exactly one real-provider kickoff. Eleven differentiated roles produced 23 peer messages, two complete review cycles, nine active channels, four cross-surface objects, and three successful disruption recoveries. One frontend session processed three compatible tasks serially while public starvation and needs-attention counts stayed zero; no elastic worker appeared.
- Northstar report: `docs/qa/reports/2026-07-15-northstar-pay-capacity-retest.md`.

## What Was Fixed

| Defect | Correction | Status |
|---|---|---|
| BUG-0028 | Deterministic runtime task activation behind a confirmed one-kickoff scheduler barrier and agent-only workspace registration | Verified by 12/12-task real-provider retest |
| BUG-20260715-scheduler-resume-starvation | Pause time excluded from post-resume starvation age | Verified by barrier retest |
| BUG-20260715-serial-pool-starves-backlog | Structural compatibility separated from momentary capacity; compatible busy work freezes escalation | Verified by serial handoff retest |
| BUG-20260715-mcp-oauth-name-segment | Collision-safe Vault segment encoding without changing public MCP identity | Verified live and in Vault/GlobalDB/Settings suites |
| BUG-20260715-mcp-manual-tty-echo | Hidden interactive TTY input with piped stdin preserved | Verified in real PTY and CLI suite |
| BUG-20260715-extension-cli-slow-boot-offline | Reachable UDS status owns daemon truth after slow metadata publication | Verified live and in CLI suite |
| BUG-20260715-config-set-late-metadata | Live config writes use reachable daemon truth during late metadata publication | Verified live and in owner suites |
| BUG-20260715-marketplace-config-set-live | Catalog paths and reload diff support truthful live application | Verified live and in CLI/Settings suites |
| BUG-20260715-marketplace-native-config-policy | Agent tool may manage duration paths while feed root remains operator-only | Verified live and in native-tool policy suite |
| BUG-20260715-native-marketplace-extension-parity | Native Marketplace projection refreshes extension registry state after daemon boot | Verified across all discovery planes |
| BUG-20260715-mcp-install-null-values | Input-free curated installs omit absent values instead of sending invalid nulls | Verified in Web/E2E owners |
| BUG-20260715-mcp-editor-vault-ref-case | `MonoId` preserves case only at byte-sensitive Vault-ref call sites | Verified by UI/Web tests and browser copy/read |
| BUG-20260715-marketplace-stale-report | Failed refresh retains and labels the usable stale projection | Verified on landing and kind views |
| BUG-20260715-provider-unchanged-overlay | Effective/raw no-op detection prevents false overlay and restart | Verified by mobile live replay and Settings suite |
| BUG-20260714-keyboard-focus-invisible | Shared 2px contrast-safe focus tokens, consumer hard cut, lint guard, and rendered proof | Verified across UI/Web/site |

All corrections are included in the completed Phase D remediation batch. The official review ledger records 42 resolved findings and no open finding.

## Paper Cuts

No unresolved paper cut is carried out of the six charters. The focus defect and Vault-ref casing problem were treated as product-contract failures and fixed rather than documented as workarounds.

## Runtime Errors Observed

No unexplained runtime error remains. Controlled negative probes produced the expected digest mismatch, policy denial, config validation failure, non-loopback callback refusal, malformed authorization failure, partial-feed stale result, and legacy-route 404 responses.

## Human Verifications Needed

None.

## Decisions for a Human

None. Policy A was explicitly selected: compatible busy capacity waits serially without elastic parallelism or premature starvation.

## Learnings

- Reachability, not a fixed process-age tolerance, owns daemon availability after slow startup.
- Public identity and secret-storage path encoding are different boundaries; only the Vault path segment may be encoded.
- A Marketplace feature is incomplete until CLI/HTTP/UDS/native discovery, config lifecycle, official skill, Web management, and workspace credential isolation agree.
- Shared accessibility defects must be repaired at the token and enforcement owner across every frontend consumer.
- Autonomous scenario tasks must be created before the one kickoff, held behind an evidenced barrier, and released only after that exact provider post is confirmed.

## Process Envelope

- Primary manifest: `/Users/pedronauck/dev/qa-labs/agh-marketplace-task11-final-20260715-20260716-011529-818379-lab/qa-artifacts/qa/bootstrap-manifest.json`.
- Primary runtime home: `/var/folders/7x/xg204hnd04b81fczcxvjlhzr0000gn/T/aghqa-0193d4b83412/runtime`.
- Primary provider home: `/var/folders/7x/xg204hnd04b81fczcxvjlhzr0000gn/T/aghqa-0193d4b83412/provider`.
- Primary evidence index: `qa-artifacts/qa/notes/` and `qa-artifacts/qa/web/` under the primary lab.
- Primary teardown completed at 2026-07-16T06:57:05Z. `qa-artifacts/qa/teardown.json` records `clean=true`, `survivors=[]`, and termination of the registered daemon, Web, and two fixture-server PIDs.
- Final redaction record: `qa-artifacts/qa/notes/final-redaction-scan.json`; no unexpected secret-class value was found outside the authorized fixture input or binary credential stores.
- Northstar capacity lab: `/Users/pedronauck/dev/qa-labs/agh-marketplace-northstar-capacity-final-20260715-20260716-001326-274237-lab`; teardown completed at 2026-07-16T01:10:09Z with `clean=true` and `survivors=[]`.
- Focus capture: `.tmp/bug-20260714-focus/resting.png`, `.tmp/bug-20260714-focus/focused.png`, and `.tmp/bug-20260714-focus/teardown.json` (`clean=true`).
- Runtime/Web E2E left no owned daemon, Playwright, browser, or watcher process after their lanes.

## AGH Impact Audit

- **Native tools:** `agh__marketplace_search` discovery parity and registration are verified. Marketplace catalog timing is now agent-manageable only for the safe `ttl`/`timeout` paths; `base_url` remains an explicit operator trust root. Native extension projection now uses the live registry. Generated descriptors and schemas are covered by codegen checks.
- **Extensibility and hooks:** Extension digests, provenance, policy, lifecycle, bundle preview/activate/update, MCP authorization, capability discovery, registry refresh, bridge projection, Vault refs, and config lifecycle were exercised. Hooks `enabled=false` remains independent of `required=true`. Official extension/skill/bundle state matched management surfaces.
- **Workspace data isolation:** Marketplace discovery is global/catalog-scoped; installs, MCP config, OAuth tokens, canonical refs, bundle activation, sessions, Tasks, events, SSE/Web caches, and runtime reads were checked at their owning global/workspace/session scope. Homonymous MCP targets in two workspaces retained distinct credentials and refs with no cross-workspace leakage.
- **Official AGH skill:** Marketplace, MCP, extension, bundle, config, and scheduler-capacity references in `skills/agh/` are updated to the verified public behavior. No undocumented native mutation surface is claimed.

## Final Status

Behavioral and charter verdict: **PASS after fixes**. Every referenced scenario is reconciled, both E2E lanes pass, the autonomous one-kickoff capacity retest passes, every QA lab/capture envelope is torn down cleanly, and the full `make verify` passes. The official 42-finding review ledger is fully resolved, the strict Northstar auditor reports PASS with zero blockers or warnings, and Phase D records SHIP.
