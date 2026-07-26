# CH-agent-marketplace-parity: An agent acquires everything a human can, and the planes agree field-for-field

```yaml
charter:
  id: CH-agent-marketplace-parity
  mission: "As Ada — no web UI, no TTY, no browser — walk discovery and acquisition through their supported structured planes: CLI JSON, HTTP, and UDS for lifecycle operations, plus agh__marketplace_search for discovery. Prove those planes report the same daemon-owned state, every error is deterministic, and a rejected install provably writes nothing (the born-valid anchor on the agent plane)."
  mode: charter-with-tour
  persona:
    name: Ada
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-agent-marketplace-parity
  scenarios: [ET-api-marketplace-namespace, ET-cli-marketplace-search, ET-cli-marketplace-info, ET-cli-marketplace-refresh, ET-cli-mcp-install, ET-api-mcp-catalog-install, ET-cli-mcp-authorize, ET-002, ET-007, ET-008, ET-016, ET-025, ET-026, ET-027, ET-028, ET-049]
  tour: Feature Tour
  time_box_minutes: 90
  guidance:
    must_try:
      - "For one daemon state, capture `agh marketplace search -o json` (idle and queried), the HTTP and UDS responses, and the agh__marketplace_search result: grouped kind order fixed (mcp/extension/skill/bundle), totals only where real, installed/update fields identical across the four discovery planes. Do not infer native mutation support from this read-only tool."
      - "Resolve info by an entry whose display name differs from entry_id; then probe unknown kind, unknown entry, blank segments, invalid limit — each error deterministic (400/404) and identical across CLI/HTTP/UDS; confirm the deleted legacy browse routes return 404."
      - "Exercise the focused projections explicitly: `agh skill search <query> -o json`, resolve the selected entry with `agh skill info <entry_id> -o json`, install it and read effective metadata with `agh skill inspect <installed-name> -o json`, then run `agh extension search <query> -o json`. Compare each response with its unified namespace or installed-lifecycle owner."
      - "Install a curated MCP entry twice: once complete (typed + vault-ref values, next_step correct, provenance stamped, refs visible without values) and once missing a required value or overriding a locked template field — the rejection must leave config, vault, and provenance provably untouched on fresh reads."
      - "Install the same entry into two workspaces and authorize both via `agh mcp authorize`: distinct scoped tokens AND distinct canonical secret_env refs per workspace; each authorize command exits non-zero unless its exact target reaches authenticated && token_present."
      - "Run `agh bundle preview`, `agh bundle activate`, and `agh bundle update` against the same activation exercised through HTTP and UDS: preview writes nothing, activation inventory/scope agree, and update clears a deliberately induced spec_drift after re-applying the current profile."
      - "Refresh with an isolated feed: per-kind structured outcomes, --kind bundle rejected without projection mutation, a downed feed served stale-marked while installed-item management still works; break one kind's source and prove the grouped search isolates the failure to that kind's error field."
      - "Confirm `agh__marketplace_search` is registered and callable with search behavior mirroring HTTP/UDS, verify every other agh__* ID explicitly named by an in-scope scenario is registered and callable for its own documented contract (ET-049), and scan all structured output, events, and logs for plaintext secrets or OAuth material."
    must_avoid:
      - "Web surfaces entirely; extension policy administration beyond reading diagnostics (CH-extension-policy-admin-gates); manual paste-back completion (CH-remote-operator-manual-auth)."
  evidence_expectations:
    - "Field-level diff artifacts by supported plane: CLI vs HTTP vs UDS vs native for search; CLI vs HTTP vs UDS for info, install result, auth status, and bundle lifecycle — the SD-011 parity proof."
    - "For each rejected install: the deterministic error payload plus fresh config/vault/provenance reads proving zero residue — the agent-plane born-valid capture."
    - "The two-workspace isolation reads (tokens and secret_env refs) and the redaction scan notes."
```

<!-- The charter is durable and immutable: each run's debrief belongs in that run's dated report. -->
