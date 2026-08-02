# QA Run Report — 2026-08-02 — bundles removal

- **Scope:** Extension-kit lifecycle, instance-scoped secrets, exact Network consent, Bundle product hard cut, surviving homonyms, and adjacent Marketplace/Skills canary.
- **Cadence tier:** targeted
- **Build:** `855b273` · **Environment:** fresh replacement lab `devtool-oss-launch-20260802-195112-911343`, daemon `http://127.0.0.1:63670`, manifest-owned runtime/provider homes, UDS, tmux socket, and isolated Web proxy. Attempt 1 was torn down cleanly before any verdict.
- **Started:** 2026-08-02T19:40:31Z · **Completed:** 2026-08-02T22:40:15Z · **Status:** blocked-decision

## Personas

| Persona | Base | Device / Network / Locale | Sessions |
|---|---|---|---|
| Bruno | Power User | desktop / wifi-fast / en-US | CH-extension-kit-lifecycle; CH-extension-secrets-instance-isolation; CH-extension-network-consent; CH-extension-marketplace-skill-canary |
| Ada | Power User (structured surfaces) | desktop / wifi-fast / en-US | CH-bundle-product-hard-cut |

## Flows in Scope

- `J-extension-kit-lifecycle` — install one inert kit, bind secrets, preview and consent, enable, update, disable, and remove it (`../journeys/J-extension-kit-lifecycle.md`).
- `J-bundle-product-boundary` — prove the retired product is absent while support archives and bundled skills retain their contracts (`../journeys/J-bundle-product-boundary.md`).
- `J-marketplace-acquisition` — keep the three-kind acquisition surface healthy after the hard cut (`../journeys/J-marketplace-acquisition.md`).

## Session Matrix & Results

| # | Charter | Journey / Scenario | Persona | Tour | Status | Issue | Fix commit |
|---:|---|---|---|---|---|---|---|
| 1 | CH-extension-kit-lifecycle | J-extension-kit-lifecycle / ET-extension-code-first-authoring | Bruno | Feature Tour | Fixed | BUG-20260802-scaffold-sdk-version; BUG-20260802-manifest-mcp-tool-handler | `7866661`; `881a254` |
| 2 | CH-extension-kit-lifecycle | J-extension-kit-lifecycle / ET-ext-kit-enable | Bruno | Feature Tour | Fixed | BUG-20260802-extension-agent-edit-reset; BUG-20260802-manifest-mcp-tool-handler | `4f1ceef`; `881a254` |
| 3 | CH-extension-kit-lifecycle | J-extension-kit-lifecycle / ET-ext-inventory | Bruno | Feature Tour | Fixed | BUG-20260802-manifest-mcp-tool-handler | `881a254` |
| 4 | CH-extension-kit-lifecycle | J-extension-kit-lifecycle / ET-ext-preview | Bruno | Feature Tour | Pass | | |
| 5 | CH-extension-kit-lifecycle | J-extension-kit-lifecycle / ET-020 | Bruno | Feature Tour | Fixed | BUG-20260802-extension-agent-edit-reset | `4f1ceef` |
| 6 | CH-extension-kit-lifecycle | J-extension-kit-lifecycle / ET-021 | Bruno | Feature Tour | Fixed | BUG-20260802-extension-agent-edit-reset | `4f1ceef` |
| 7 | CH-extension-kit-lifecycle | J-extension-kit-lifecycle / ET-022 | Bruno | Feature Tour | Fixed | BUG-20260802-extension-agent-edit-reset | `4f1ceef` |
| 8 | CH-extension-kit-lifecycle | J-extension-kit-lifecycle / ET-window-manager-hooks-resources | Bruno | Feature Tour | Pass | | |
| 9 | CH-extension-kit-lifecycle | J-extension-kit-lifecycle / ET-web-extension-detail | Bruno | Feature Tour | Pass | | |
| 10 | CH-extension-kit-lifecycle | J-extension-kit-lifecycle / ET-web-extension-kit-inventory | Bruno | Feature Tour | Pass | | |
| 11 | CH-extension-kit-lifecycle | J-extension-kit-lifecycle / ET-web-extensions-manage | Bruno | Feature Tour | Pass | | |
| 12 | CH-extension-kit-lifecycle | J-extension-kit-lifecycle / ET-web-marketplace-installed-management | Bruno | Feature Tour | Pass | | |
| 13 | CH-extension-secrets-instance-isolation | J-extension-kit-lifecycle / ET-ext-secrets-binding | Bruno | Garbage Tour | Pass | | |
| 14 | CH-extension-network-consent | J-extension-kit-lifecycle / ET-ext-network-confirm | Bruno | Multi-Tab Tour | Pass | | |
| 15 | CH-extension-network-consent | J-extension-kit-lifecycle / ET-019 | Bruno | Multi-Tab Tour | Pass | | |
| 16 | CH-bundle-product-hard-cut | J-bundle-product-boundary / ET-bundle-product-surface-absent | Ada | Garbage Tour | Fixed | BUG-20260802-retired-marketplace-kind-alias | `7701a3f` |
| 17 | CH-bundle-product-hard-cut | J-bundle-product-boundary / ET-api-marketplace-namespace | Ada | Garbage Tour | Pass | | |
| 18 | CH-bundle-product-hard-cut | J-bundle-product-boundary / ET-cli-marketplace-search | Ada | Garbage Tour | Pass | | |
| 19 | CH-bundle-product-hard-cut | J-bundle-product-boundary / ET-cli-marketplace-info | Ada | Garbage Tour | Pass | | |
| 20 | CH-bundle-product-hard-cut | J-bundle-product-boundary / ET-cli-marketplace-refresh | Ada | Garbage Tour | Pass | | |
| 21 | CH-bundle-product-hard-cut | J-bundle-product-boundary / ET-033 | Ada | Garbage Tour | Pass | | |
| 22 | CH-bundle-product-hard-cut | J-bundle-product-boundary / ET-035 | Ada | Garbage Tour | Pass | | |
| 23 | CH-bundle-product-hard-cut | J-bundle-product-boundary / ET-049 | Ada | Garbage Tour | Pass | | |
| 24 | CH-bundle-product-hard-cut | J-bundle-product-boundary / ET-compozy-official-skill-discovery | Ada | Garbage Tour | Pass | | |
| 25 | CH-bundle-product-hard-cut | J-bundle-product-boundary / ET-site-docs-api-reference-ui | Ada | Garbage Tour | Fixed | BUG-20260802-site-topbar-client-boundary | `a817e37` |
| 26 | CH-bundle-product-hard-cut | J-bundle-product-boundary / ET-web-catalog-navigation | Ada | Garbage Tour | Fixed | BUG-20260802-retired-marketplace-kind-alias | `7701a3f` |
| 27 | CH-bundle-product-hard-cut | J-bundle-product-boundary / ET-web-marketplace-kind-navigation | Ada | Garbage Tour | Fixed | BUG-20260802-retired-marketplace-kind-alias | `7701a3f` |
| 28 | CH-bundle-product-hard-cut | J-bundle-product-boundary / RT-reserved-builtin-agent-names | Ada | Garbage Tour | Pass | | |
| 29 | CH-bundle-product-hard-cut | J-bundle-product-boundary / SITE-resource-mutation-boundary | Ada | Garbage Tour | Pass | | |
| 30 | CH-extension-marketplace-skill-canary | J-marketplace-acquisition / ET-web-marketplace-landing-browse | Bruno | Feature Tour | Pass | | |
| 31 | CH-extension-marketplace-skill-canary | J-marketplace-acquisition / ET-web-marketplace-search-fanout | Bruno | Feature Tour | Pass | | |
| 32 | CH-extension-marketplace-skill-canary | J-marketplace-acquisition / ET-web-marketplace-skill-install | Bruno | Feature Tour | Pass | | |
| 33 | CH-extension-marketplace-skill-canary | J-marketplace-acquisition / ET-compozy-official-skill-discovery | Bruno | Feature Tour | Pass | | |
| 34 | CH-extension-marketplace-skill-canary | J-marketplace-acquisition / ET-web-extension-union-install | Bruno | Feature Tour | Pass | | |

Status legend: `Pending | Pass | Fixed | Skipped | Blocked (needs human verify) | Blocked (human decision)`

## Session Debriefs

- **Extension kit lifecycle:** A 12-item static kit remained inert at install, exposed matching
  inventory and preview data across CLI, HTTP, UDS, native tools, and Web, then published and
  retired its owned resources as one unit. A real scaffolded Go provider also built, registered,
  and invoked its tool. The local-path fixture correctly refused Marketplace-only update semantics.
- **Secret isolation:** Global and two workspace instances kept bindings separate through set,
  rotate, invalid-binding attempts, unset, restart, disable, and remove. Presence checks and a
  plaintext corpus scan found no secret value in public evidence.
- **Network consent:** Missing and stale digests returned the exact current requirement without
  mutation; the accepted digest survived restart and remained instance-scoped. The browser exposed
  the same confirmation contract.
- **Bundle hard cut:** Former CLI, HTTP, UDS, native, Marketplace, Web, docs, resource, and storage
  surfaces were absent with no alias. Support archives and bundled-skill filtering continued to
  work, while the desired-state resource example completed create/update/conflict/delete.
- **Marketplace/Skill canary:** The three surviving kinds, installed Skill, extension detail, search,
  and responsive navigation stayed truthful. A failure in one removed kind no longer falls through
  to Skills.

## What Was Fixed

- `BUG-20260802-extension-agent-edit-reset` (`4f1ceef`): reconcile now reloads current
  dir-per-agent files, so an accepted public edit to an extension-owned agent remains authoritative.
  Four focused daemon journeys and the complete runtime E2E lane passed afterward.
- `BUG-20260802-retired-marketplace-kind-alias` (`7701a3f`): `/marketplace/bundles` now reaches the
  OS not-found posture without becoming a parent of valid Marketplace detail routes. The regression
  failed before the fix and the final Web E2E lane passed all 133 tests.
- `BUG-20260802-manifest-mcp-tool-handler` (`881a254`): MCP-backed manifest tools validate their
  `server + tool` binding without an extension-host handler. The rebuilt CLI validated and enabled
  the complete 12-item fixture.
- `BUG-20260802-scaffold-sdk-version` (`7866661`): both embedded Go templates now use one published
  SDK version. A fresh provider scaffold resolved dependencies, built, installed, and invoked.
- `BUG-20260802-site-topbar-client-boundary` (`a817e37`): the shared topbar hook now declares its
  Next.js client boundary. Surviving docs routes render and retired Bundle pages return 404.
- `BUG-20260802-automatic-title-persistence-race` (`855b273`): public title reads now wait for the
  complete session-identity transaction. The deterministic race regression and full runtime E2E
  passed.

## Paper Cuts

- A local-path extension is intentionally not eligible for `extension update`; the CLI error is
  deterministic, but the distinction from Marketplace-managed update deserves clearer nearby help.

## Runtime Errors Observed

- Attempt 1 (`devtool-oss-launch-20260802-194121-120252`) was invalidated before scheduler release: the daemon inherited the full bootstrap environment, and the Engineering Lead provider process discovered `qa-artifacts/` paths. No scenario verdict was recorded and no task run began. The targeted teardown killed daemon PID `82492` and Web PID `88484`; `/Users/pedronauck/dev/qa-labs/compozy-devtool-oss-launch-20260802-194121-120252-lab/qa-artifacts/qa/teardown.json` reports `clean: true` with zero survivors.
- Attempt 2 reproduced `BUG-20260719-autonomous-progress-unobservable`: after one kickoff, all 11 Task catalog entries completed, but the 1,800-second observer received no runtime-owned progress after scheduler resume and reported a stall. Evidence: `/Users/pedronauck/dev/qa-labs/compozy-devtool-oss-launch-20260802-195112-911343-lab/qa-artifacts/qa/observation-summary.json` and sibling `tasks-final-*.json` snapshots. Later `qa-observer` rows record the independent CLI comparison and do not claim runtime authorship.
- The first complete-fixture validation exposed `BUG-20260802-manifest-mcp-tool-handler`; after the
  red regression and production fix, the daemon was rebuilt and restarted against the same isolated
  home. Persisted workspaces, Tasks, installed Skills, and the hard-cut evidence remained readable.
- The strict real-scenario audit found that only one of the three required review cycles reached a
  verdict. The other review records remained `requested` or `in_review`; no synthetic verdict was
  added. Reconciled operator rows index real API/Network evidence but remain explicitly
  `qa-observer` authored.

## Human Verifications Needed

None. The unresolved items require a product/contract decision rather than visual confirmation.

## Decisions for a Human

### Runtime progress is invisible to the real-scenario observer (`BUG-20260719-autonomous-progress-unobservable`)

- What's broken: autonomous work completes, but the required observation surface reports a stall because no runtime component projects task, session, or Network lifecycle events into the journey log.
- Why not auto-fixed: the fix needs a new runtime-to-observer progress contract across several domains and fails the QA governor's small, contained, low-risk bounds; it is also outside the Bundle-removal product boundary.
- Options:
  1. Add a daemon-owned progress projection to the real-scenario contract, then replay the one-kickoff scenario.
  2. Redesign the observer to consume existing durable public event/catalog surfaces instead of a separately written log.
- Recommendation: choose option 2, because the durable runtime surfaces already own task/session/Network truth and avoid a second event authority.

### Release-grade review cycles did not reach terminal verdicts

- What's incomplete: the broad `devtool-oss-launch` contract required three completed review
  cycles. Only `review-88e7a867fe6f7d8d` reached a verdict; four other reviews ended in `requested`
  or `in_review` even though all eleven Tasks completed.
- Why not auto-fixed: completing those reviews would require another provider intervention after the
  one-kickoff boundary, which would invalidate the scenario rather than repair production behavior.
- Recommendation: treat terminal review completion as part of Task terminality for release-grade
  playbooks, or teach the observer to wait on durable review state separately from Task completion.

## Learnings

- The open bug registry was reviewed before execution. In particular, `BUG-0028` remains a known one-kickoff real-scenario risk, and `BUG-20260729-resource-docs-protected-kind` is directly in scope for re-verification after the documentation hard cut.
- The replacement bootstrap created 8 differentiated agents, 5 channels, 11 deterministic open tasks, all required knowledge projections, and the exact `devtool-oss-launch` deliverable/collaboration contract. Manifest: `/Users/pedronauck/dev/qa-labs/compozy-devtool-oss-launch-20260802-195112-911343-lab/qa-artifacts/qa/bootstrap-manifest.json`.
- Provider fidelity requires starting the daemon with only its manifest-owned runtime variables; exporting evidence variables such as `QA_OUTPUT_PATH` into the daemon leaks evaluator context to child providers and invalidates the walk.

## Evidence

- Main lab: `/Users/pedronauck/dev/qa-labs/compozy-devtool-oss-launch-20260802-195112-911343-lab/qa-artifacts/qa/`.
- Strict audit: `qa-audit-report.json` and `qa-audit-report.md`; the remaining blocker is the two
  missing terminal review verdicts.
- Teardown: `teardown.json` reports `clean: true`, an empty survivor list, and termination of every
  registered daemon, Web, browser, observer, and site process.
- Screenshots: extension inventory at 375/768/1280px, Network confirmation, installed Skill,
  three-kind Marketplace, retired Web route not-found, surviving API docs, and retired docs 404.

## Experiential Lens Pass

- **J-extension-kit-lifecycle:** The inventory-first model is understandable and responsive from
  mobile through desktop. Exact digest confirmation builds trust because the refusal and retry use
  the same visible value. The main friction is that update eligibility depends on install provenance
  but the local-path rejection does not explain where managed updates begin.
- **J-marketplace-acquisition:** Three kinds read as one coherent catalog without resurrecting the
  retired product. Installed state and extension inventory stayed stable across refresh and resize;
  the OS not-found posture made the removed route explicit instead of silently changing context.

## Compozy Impact Audit

- **Native tools:** `compozy__extensions_inventory` and `compozy__extensions_preview` were registered,
  available, authorized, and callable; a scaffolded extension tool was invoked; no
  `compozy__bundles_*` ID, Bundle toolset, or retired capability remained.
- **Extensibility and hooks:** The 12-item kit covered agent sidecars, Skill, Loop, tool, MCP server,
  hook, automation, layout, environment binding, Network consent, extension-host SDK scaffolding,
  and lifecycle cleanup. Disable/remove left no owned resource, scheduler, tool, agent, or binding
  residue.
- **Workspace data isolation:** Secret bindings and dev links were verified across global,
  workspace-A, and workspace-B instances. CLI, HTTP, UDS, Web, events, and post-restart reads showed
  no cross-workspace value, confirmation, or inventory leak.
- **Official Compozy skill:** Bundled discovery, inspect/view, router guidance, and native-tool
  reference retained the extension lifecycle and contained no retired Bundle teaching.

## Final Status

- **Exit gate:** `make gate` escalated to the repository-wide `make verify` and passed at fingerprint
  `3cd478e2ab39e7f196ce3fbbd5e4062fa8ae7f44`; evidence log
  `.cache/gate/logs/full-1785710879.log`. A final docs-only gate refresh follows this report update.
- **Automated E2E:** Web 133/133; runtime daemon 133, HTTP 8, UDS 32, testutil/e2e 8; focused site
  production build passed.
- **Strict real-scenario audit:** **BLOCKED** — the run produced 11/11 completed Tasks and real
  collaboration evidence, but only one of three required review cycles reached a verdict. The
  runtime-owned observer stream also remains absent under
  `BUG-20260719-autonomous-progress-unobservable`.
- **Issues by user impact:** Blocks-Completion 4 fixed, 1 open decision · Trust-Damage 2 fixed ·
  Data-Loss 0 · Friction 1 noted · Cosmetic 0.
- **Coverage:** 34 / 34 targeted scenarios settled; `RT-073` is `blocked-decision`.
- **Verdict:** **BLOCKED (human decision)** for the release-grade autonomous-collaboration contract.
  The Bundle-removal product boundary and all targeted extension, secret, consent, hard-cut,
  Marketplace, Web, docs, native, and homonym scenarios passed after the six recorded fixes.
