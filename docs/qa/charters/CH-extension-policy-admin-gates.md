# CH-extension-policy-admin-gates: Feed the trust gates garbage and prove every block is real and every flip is live

```yaml
charter:
  id: CH-extension-policy-admin-gates
  mission: "As Vera, the policy owner, run the Garbage Tour across the acquisition trust surface — hostile side-loads, tampered curated archives, invalid config values, a pulled catalog entry — and prove the two-level consent gate, the no-bypass digest verification, the live policy lifecycle, the settings split, and the kill-switch all hold on every plane."
  mode: charter-with-tour
  persona:
    name: Vera
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-extension-policy-admin
  scenarios: [ET-web-settings-extensions-policy, ET-web-settings-hooks, ET-cli-extension-sideload-policy-block, ET-ext-curated-digest-verify, ET-017, ET-018, ET-023, ET-044, ET-045, MS-031, MS-marketplace-catalog-live-config, ET-marketplace-kill-switch]
  tour: Garbage Tour
  time_box_minutes: 90
  guidance:
    must_try:
      - "Highest blast radius first — digest gate: with an isolated feed and a controlled release asset, swap archive bytes under the pinned version and prove the curated install hard-fails before extraction with no residue and no allow-unverified escape; then install the clean asset and confirm verified provenance (checksum_verified=true, distinct archive digest vs tree checksum) via API/CLI and the web provenance panel."
      - "Under default allow_unverified=false, attempt side-loads with and without the per-request flag over web, CLI (-o json), and API: every block must be the deterministic 422-class diagnostic pointing at Settings › Extensions, with zero install files or registry rows left behind."
      - "Flip allow_unverified on /settings/extensions and prove live apply: no restart, Marketplace blocked affordances flip to warning-confirm immediately, and the request-level consent is still required (two-level consent never collapses)."
      - "Feed each owning config surface garbage: invalid extensions.marketplace.registry/base_url values through `agh config set` and the Settings › Extensions form; invalid marketplace.catalog.base_url, zero TTL, and negative timeout through `agh config set`/agh__config_set (these catalog fields have no settings form). Every rejection preserves the prior applied value; then prove a valid live catalog base_url change affects the next refresh using two isolated feeds."
      - "Kill-switch: remove an installed entry from the isolated feed, refresh, and prove it is gone from search/browse/detail on web, CLI, and API while the installed item stays fully manageable (Safety Invariants 3 + 12); also prove a failed refresh serves the prior projection marked stale, never a prune."
      - "Settings split: /settings/hooks carries hooks + presets only (toggle persists, restart-required reported truthfully, enabled=false honored independently of required=true); extension policy shows exactly registry, base_url, allow_unverified and a policy-only save never claims a resource restart."
    must_avoid:
      - "Acquisition happy paths and timing (CH-marketplace-under-a-minute); MCP authorization (the two authorize charters); inventing policy fields the daemon doesn't expose."
  evidence_expectations:
    - "The digest-mismatch failure with a post-attempt filesystem/registry read proving no residue; the clean install's verified provenance JSON."
    - "The structured policy-block diagnostics per plane; before/after affordance screenshots around the live flip."
    - "Kill-switch proof: entry present → feed edit → refresh output → absent on all discovery planes → installed item still editable; config-rejection reads showing prior values preserved."
```

<!-- The charter is durable and immutable: each run's debrief belongs in that run's dated report. -->
