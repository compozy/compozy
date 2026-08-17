# QA Run Report — 2026-08-16 — agent-plugins

- **Scope:** Agent Plugins ingestion, lifecycle, diagnostics, secret-header delivery, Marketplace/Web surfaces, provider delivery, and Agent Plugins 1.0.0 conformance
- **Cadence tier:** targeted
- **Build:** source `f85601523c9ebdf85d6931d151acf19f8033d14f`, QA diff `0a5a6051b299bab3aa05180c1e2ea0fa55eb07d56f2fdf7cbff5e815e0260e1a`, binary `45f7087df5c91caf24745f95425b26ba41efb195402bc48cb95d0f31f96b1fea`
- **Environment:** fresh isolated `northstar-pay` lab at `http://127.0.0.1:57301`; manifest `/Users/pedronauck/dev/qa-labs/compozy-agent-plugins-20260816-20260816-061032-351590-lab/qa-artifacts/qa/bootstrap-manifest.json`
- **Started:** 2026-08-16T06:10:59Z · **Status:** terminal scenarios; full-gate result owned by the repository gate record

## Personas

| Persona | Base | Device / Network / Locale | Sessions |
|---|---|---|---|
| Ada | Power User | desktop / wifi-fast / en-US | conformance, provider delivery, diagnostics parity, Marketplace |
| Bruno | Power User | desktop / wifi-fast / en-US | lifecycle recovery, dev isolation, remote secrets, native canary |

## Setup Evidence

- The official `agentplugins/agent-plugins-example` repository was acquired at commit `5f3f5084a821aefa792e79500dd8f0462ab83473` for the local-directory and pinned-git comparison.
- One enabled generation, `acme.tools` 1.2.0, exposed two skills and two MCP servers with checksum `fac77a9d8bc71706a64d83eaeb95b4f340fbed2e2362a6106d42559bf0c0195a`.
- The remote MCP fixture returned 401 without its bound header and 200 with it. Its retained log records authorization presence/result and tenant only: `/Users/pedronauck/dev/qa-labs/compozy-agent-plugins-20260816-20260816-061032-351590-lab/qa-artifacts/qa/remote-mcp-requests.jsonl`.
- `make test-e2e-runtime` and `make test-e2e-web` both passed. The final full repository gate remains reserved for workstream close.

## Session Matrix & Results

| # | Charter | Scenarios | Persona | Tour | Status | Issue |
|---|---|---|---|---|---|---|
| 1 | CH-agent-plugin-conformance | validation; native precedence; conformance walk | Ada | Garbage Tour | Pass | path projection; validation exit; hosted projection fixed and re-walked |
| 2 | CH-agent-plugin-provider-delivery | provider delivery | Ada | Feature Tour | Pass (known limitation) | Claude Code/Hermes pass; OpenClaw ACP fails closed |
| 3 | CH-agent-plugin-lifecycle-recovery | source install; data removal | Bruno | Interrupt Tour | Pass | — |
| 4 | CH-agent-plugin-diagnostics-parity | degraded inventory | Ada | Network Tour | Pass | adjacent daemon stop timeout tracked |
| 5 | CH-agent-plugin-dev-isolation | dev reload | Bruno | Multi-Tab Tour | Pass | — |
| 6 | CH-agent-plugin-remote-secrets | remote header | Bruno | Paste Tour | Pass | hosted projection fixed and re-walked |
| 7 | CH-agent-plugin-marketplace | Marketplace install | Bruno | Back-Button Tour | Blocked (needs verify) | synthetic release URL returned 404 at install |
| 8 | CH-agent-plugin-native-canary | native published-source install | Bruno | Feature Tour | Pass | — |
| 9 | CH-agent-plugin-real-scenario-runtime | task-role activation canary | Bruno | Feature Tour | Blocked (needs verify) | kickoff and provider-backed task-role journey were not executed |

Status legend: `Pass | Pass (known limitation) | Blocked (needs verify)`

## Session Debriefs

### Conformance, validation, and native precedence

Human, JSON, JSONL, TOON, and native-tool reads agreed on portable format, ingested components,
ordered skips, and fatality. Validation did not install, create data, launch a process, or publish a
resource. The dual-manifest fixture selected native `format: compozy`, emitted one informational note,
and leaked no portable resources. Fatal portable validation initially exited 0; the fix loop made it
exit 1 without changing warning-only exit behavior.

### Lifecycle, data, development, and diagnostics

Local and pinned-git acquisition converged on the same portable lifecycle without a format flag. Data
was lazy-created by stdio launch, survived update, and was removed before deterministic name reuse;
the focused deletion/quarantine/fail-closed owner passed. A workspace dev overlay reloaded only its
generation and retained its data. CLI, HTTP/UDS-backed native reads, and Web showed the same two
skills, two MCP servers, and three ordered component-skipped diagnostics. Ingest diagnostics survived
restart while live health was recomputed.

### Remote header and redaction

Binding targeted only `remote:Authorization`; unset made the remote call fail and rebind recovered it.
Secret listing exposed only `REMOTE_AUTH`, server `remote`, header `Authorization`, and stale state.
Session events, structured extension reads, daemon logs, and the remote request log contained neither
the plaintext credential nor its Vault reference.

### Marketplace and browser

The real browser showed the neutral Agent Plugin badge on card and detail, the normal trust dialog,
installed management, and the skipped-component inventory. The synthetic catalog's GitHub release
URL returned 404 on the final Web install request, so acquisition remains `blocked-verify`. The same
bytes were installed through the public CLI to exercise the installed Web state; the production HTTPS
and SSRF boundary was not weakened for the fixture.

- Recording: `/Users/pedronauck/.config/browser-harness/agent-workspace/recordings/agent-plugin-marketplace`
- Screenshots: `/Users/pedronauck/dev/qa-labs/compozy-agent-plugins-20260816-20260816-061032-351590-lab/qa-artifacts/browser-screenshots/installed-agent-plugin.png`; `/Users/pedronauck/dev/qa-labs/compozy-agent-plugins-20260816-20260816-061032-351590-lab/qa-artifacts/browser-screenshots/agent-plugin-inventory.png`; `/Users/pedronauck/dev/qa-labs/compozy-agent-plugins-20260816-20260816-061032-351590-lab/qa-artifacts/browser-screenshots/extensions-settings.png`

### Provider delivery

Claude Code session `sess-881a3c777fe36d33` and Hermes session `sess-718859a9a9963f8a` each loaded
`/acme.tools:release`, invoked the canonical local tool, observed absolute package/data paths and a
successful data write, then invoked the daemon-hosted remote tool and received
`{"authorized":true,"tenant":"acme"}`. Both final-code walks passed on their first attempt.

OpenClaw session `sess-36b29c3eab89472f` failed before provider launch because CompozyOS truthfully
advertises `session_mcp=false` for OpenClaw. Current OpenClaw ACP bridge documentation says
per-session MCP servers are unsupported. The user narrowed the product claim to the complete Claude
Code and Hermes paths. The raw provider matrix retains the OpenClaw block; the final verdict does not
turn it into a provider-delivery pass.

### Real scenario runtime

The isolated bootstrap manifest records `KICKOFF_POSTED=false`. No Northstar Pay kickoff, autonomous
task-role run, or strict playbook audit was executed in this targeted pass. This charter therefore
remains `blocked-verify`; provider delivery evidence from the separate managed sessions does not
substitute for the startup scenario.

## Conformance Walk

The numbered artifact at
`docs/qa/evidence/2026-08-16-agent-plugins/conformance-checklist.json` records one observable and
evidence path for each required item:

1. directory loading;
2. exact closed plugin schema `https://agent-plugins.org/schemas/1.0.0/plugin.schema.json`;
3. ignored unowned extensions;
4. fixed-location discovery;
5. exact MCP schema `https://agent-plugins.org/schemas/1.0.0/mcp.schema.json`, stdio, and streamable-http;
6. absolute `PLUGIN_ROOT` and `PLUGIN_DATA` expansion in args, environment, and cwd;
7. single-token stdio command and package-root default cwd;
8. support for both skills and MCP servers.

All eight items passed. The separate provider artifact is
`docs/qa/evidence/2026-08-16-agent-plugins/provider-matrix.json`.

## What Was Fixed

The reviewed remediation batch is commit `35100d40b55c`.

| Bug | Root cause | Verification |
|---|---|---|
| BUG-20260816-agent-plugin-path-projection | lexical package root and absolute synthesized skill paths violated canonical manifest containment | two skills and two MCP servers projected to both passing providers |
| BUG-20260816-agent-plugin-validation-exit | fatal result did not affect CLI process status | fatal exits 1; warning-only exits 0 |
| BUG-20260816-hosted-mcp-bootstrap-projection | no reconcile idle fence, incomplete required-tool bootstrap, unstable generation cache, and typed-nil output schema | focused race tests plus first-attempt Claude Code and Hermes walks |

## Known Limitations and Open Bugs

| Bug | Status | Effect |
|---|---|---|
| BUG-20260816-openclaw-session-mcp-gap | closed / accepted limitation | OpenClaw is excluded from the portable MCP delivery claim |
| BUG-20260816-daemon-stop-timeout | open | isolated daemon stop timed out; manifest teardown remains required and reliable |

The user chose the evidenced contract: complete provider delivery is claimed for Claude Code and
Hermes. OpenClaw remains a disclosed ACP limitation. The compatible-clients contribution is a
user-owned follow-up after the CompozyOS interoperability page is deployed.

## Paper Cuts

| Persona | Where | Felt | Outcome |
|---|---|---|---|
| Ada | Marketplace install | Fixture card looked real but its synthetic GitHub asset URL returned 404 | blocked-verify; retained HTTPS/SSRF safety |
| Bruno | daemon restart | exact stop command timed out during rebuild cycles | open bug; cleanup delegated to registered manifest teardown |

## Compozy Impact Audit

- **Native tools:** `compozy__extensions_validate`, `compozy__extensions_install`, `compozy__extensions_enable`, `compozy__extensions_info`, `compozy__extensions_list`, and `compozy__extensions_inventory` were exercised against the daemon. Secrets remain CLI/API-managed; there is no extension-secret native tool. No tool id, descriptor, schema digest, risk flag, or capability gate changed in the QA fix loop.
- **Extensibility and hooks:** Agent Plugins skill/MCP synthesis, resource reconciliation, hosted MCP bootstrap/cache/wire schema, lifecycle, and remote secret binding were checked. No extension hook, capability, bridge SDK, sidecar API, or `config.toml` key changed; the isolated lab enabled existing external-tool policy only for provider verification.
- **Workspace data isolation:** package bytes are instance-owned, dev overlays are workspace-scoped, plugin data is instance-scoped, and hosted tool projection is session-scoped. Dev reload and provider sessions used separate workspaces; inventory, caches, events, and provider projection did not cross workspace ids.
- **Official Compozy skill:** checked `skills/compozy/` against the new public install, validate, lifecycle, inventory, secret-header behavior, and the OpenClaw ACP delivery boundary. The reference now states the same Claude Code/Hermes evidence limit as the public docs.

## Final Status

- **Scenario coverage:** 10/10 terminal — 8 Pass (including 1 known limitation), 2 Blocked (needs verify), 0 untested/fail.
- **Automated E2E:** `make test-e2e-runtime` pass; `make test-e2e-web` pass.
- **Teardown:** pass — `/Users/pedronauck/dev/qa-labs/compozy-agent-plugins-20260816-20260816-061032-351590-lab/qa-artifacts/qa/teardown.json` records `clean: true`, no survivors, and all registered daemon, provider, fixture, and browser processes stopped.
- **Exit gate:** this immutable report does not duplicate the content-addressed gate result; `make gate-status` is the authoritative record for the exact closing tree.
- **Verdict:** **ready for the CompozyOS PR with blocked items disclosed.** The 8-item conformance walk and complete Claude Code/Hermes provider paths pass. OpenClaw is outside the portable MCP delivery claim. The user owns the external compatible-clients PR after the interoperability docs are deployed.
