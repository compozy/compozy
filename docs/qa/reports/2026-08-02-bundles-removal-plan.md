# QA Plan — 2026-08-02 — Bundles removal

- **Scope:** extension-only kit lifecycle, secret binding, exact Network consent, public Bundle hard
  cut, affected Web/docs/official-skill surfaces, and the support/bundled homonym keep-set.
- **Cadence tier:** targeted — four risk-owner sessions plus one adjacent Marketplace/Skills canary.
- **Status:** planning only. No runtime was launched, no persona walk occurred, and this file records
  no verdict or evidence.
- **Execution rule:** task 06 creates one fresh execution report at
  `docs/qa/reports/2026-08-02-bundles-removal.md` before the first session and updates scenario
  verdicts incrementally after each debrief.

## Execution matrix (ordered by risk)

| Order | Charter | Persona | Journey | Scenarios to settle | Planned evidence |
|---:|---|---|---|---|---|
| 1 | CH-extension-kit-lifecycle | Bruno | J-extension-kit-lifecycle | ET-extension-code-first-authoring; ET-ext-kit-enable; ET-ext-inventory; ET-ext-preview; ET-020; ET-021; ET-022; ET-window-manager-hooks-resources; ET-web-extension-detail; ET-web-extension-kit-inventory; ET-web-extensions-manage; ET-web-marketplace-installed-management | Build receipt, inert install, cross-plane preview/inventory, scheduler and owner reads, browser lifecycle, update/disable/remove cleanup |
| 2 | CH-extension-secrets-instance-isolation | Bruno | J-extension-kit-lifecycle | ET-ext-secrets-binding | Global/workspace binding matrix, real spawn injection, rollback/GC reads, value-free output/log/event/SSE/transcript scan |
| 3 | CH-extension-network-consent | Bruno | J-extension-kit-lifecycle | ET-ext-network-confirm; ET-019 | Missing/stale/current digest matrix, pre-swap byte comparison, batch refusal, actor truth, dev-link scope, restart parity |
| 4 | CH-bundle-product-hard-cut | Ada | J-bundle-product-boundary | ET-bundle-product-surface-absent; ET-api-marketplace-namespace; ET-cli-marketplace-search; ET-cli-marketplace-info; ET-cli-marketplace-refresh; ET-033; ET-035; ET-049; ET-compozy-official-skill-discovery; ET-site-docs-api-reference-ui; ET-web-catalog-navigation; ET-web-marketplace-kind-navigation; RT-reserved-builtin-agent-names; SITE-resource-mutation-boundary | Cross-plane absence matrix, three-kind catalog, native/skill prompt, zero residue, protected-resource and reserved-agent errors, homonym receipts |
| 5 | CH-extension-marketplace-skill-canary (reuse unchanged) | Bruno | J-marketplace-acquisition | ET-web-marketplace-landing-browse; ET-web-marketplace-search-fanout; ET-web-marketplace-skill-install; ET-compozy-official-skill-discovery; ET-web-extension-union-install | Marketplace/Skills/Extensions navigation, per-kind failure isolation, installed state, official skill identity |

## Required isolated environment

1. Run `eng-qa-bootstrap` once with a fresh scenario slug for this cycle and playbook
   `devtool-oss-launch`; never reuse a previous manifest or the default home/port.
2. Consume the bootstrap manifest as the only authority for `COMPOZY_HOME`, HTTP port and base URL,
   UDS path, `TMUX_BRIDGE_SOCKET`, `PROVIDER_HOME`, `PROVIDER_CODEX_HOME`, `QA_OUTPUT_PATH`,
   `COMPOZY_WEB_API_PROXY_TARGET`, and `TEARDOWN_COMMAND`. Do not reconstruct or hardcode values.
3. Follow provider policy: brokered credentials use the provider homes; `native_cli` with
   `home_policy = operator` preserves the operator login. Run config writes sequentially inside the
   one isolated home.
4. Register every daemon, Web server, browser helper, watcher, and other long-lived process at
   `<QA_OUTPUT_PATH>/qa/pids/<name>.pid` immediately after spawn. E2E lanes run serially under their
   machine-wide lock.
5. Use browser control for the extension-detail inventory and confirmation workflow with
   `COMPOZY_WEB_API_PROXY_TARGET` derived from the manifest. Shell-only checks cannot settle the UI
   scenarios.
6. On every pass, fail, block, abort, or interruption path, run `eval "$TEARDOWN_COMMAND"`. Task 06
   may close only after `<QA_OUTPUT_PATH>/qa/teardown.json` reports `"clean": true` and the strict
   evidence auditor passes.

## Coverage decisions

- Safety Invariants 1–4, 9–10, and 16 map to the kit session; 5–7 and 17 to secrets; 8, 16,
  and 17 to consent; 11–15 to the hard-cut boundary. ADR-001..008 are all represented by those
  owners.
- The five taxonomy dimensions are covered: complete journeys, cross-plane mechanics, human Web
  experience, realistic errors/abandonment, and global/workspace plus adjacent-kind continuity.
- The support archive and `--source bundled` are checks inside the hard-cut boundary, not standalone
  product scenarios. Historical Bundle bug files remain retired-surface evidence and are not reopened.
- The canary reuses `CH-extension-marketplace-skill-canary` unchanged because its durable mission
  exactly matches the adjacent shared-surface risk; no duplicate charter is minted.
