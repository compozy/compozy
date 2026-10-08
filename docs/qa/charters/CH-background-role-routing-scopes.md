# CH-background-role-routing-scopes: Routing changes exactly the scope I chose — live, bounded, and nowhere else

```yaml
charter:
  id: CH-background-role-routing-scopes
  mission: "As Dora, drive the whole [roles] write plane — global config set, workspace overlay, and the native compozy__config_* tools — and prove every accepted route becomes live for the next eligible invocation in exactly the chosen scope, every invalid or deleted key is rejected with its exact path, and routed catalog agents still behave as their role."
  mode: charter-with-tour
  persona:
    name: Dora
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-route-background-work
  scenarios: [MS-background-role-routing]
  tour: Feature Tour
  time_box_minutes: 90
  guidance:
    must_try:
      - "Baseline first: `compozy config list -o json` (read the exact roles.* leaves — `compozy config get` takes one flattened leaf like roles.auto_title.model, never the roles branch) and `compozy roles list -o json` on a fresh home must show the pinned defaults (coordinator off; auto_title inherit) and exactly those two roles."
      - "Global route: `compozy config set roles.auto_title.model <m>` then start a session that gets an automatic title — the hidden auto-title child resolves the routed model with the builtin identity, and the child stays out of fleet/session lists (Invariant 10)."
      - "Workspace route: `compozy config set --scope workspace --workspace <root> roles.auto_title.agent <local-titler>` — that workspace's next title generation runs the catalog agent (Compozy role overlay still applied, ADR-003/Invariant 12) while a sibling workspace keeps global routing; survive a fresh config read (Invariant 11)."
      - "Live toggle: `roles.auto_title.enabled false` → next session gets no title spawn; re-enable → titles resume, all without daemon restart."
      - "Native plane: `compozy__config_list` serves the exact roles.* leaves and `compozy__config_get roles.auto_title.model` reads one; `compozy__config_set roles.auto_title.model` accepted and `compozy__config_unset` restores the inherited value; `compozy__config_path` proves only the selected global/workspace config file target and scope; every removed path (`memory.*`, `session.compaction.*`, `roles.dream.*`, `session.auto_title_enabled`, `autonomy.coordinator.*`) rejected deterministically by both `compozy config set` and `compozy__config_set` (Invariant 9)."
      - "Rejection quality: `roles.coordinator.max_children 6` names the ≤5 cap (Invariant 8); `roles.auto_title.timeout` in TOML fails load naming `roles.auto_title.timeout` (Invariant 7; the auto-title deadline is a 60s constant); an old deleted key inside config.toml fails load naming the key; the prior good config stays authoritative after every rejection."
      - "Ghost route: `roles.auto_title.agent ghost` — the next invocation fails explicitly (`role_agent_not_found` / role.resolve.error), with no silent builtin fallback (Invariant 4)."
    must_avoid:
      - "The Settings web panel (CH-settings-roles-live-truth owns it) and fallback chains under failure (CH-role-fallback-boundary owns them)."
      - "Retired role tables (`[roles.dream]`, `[roles.checkpoint_summary]`, `[roles.memory_extractor]`, `[roles.memory_controller]`) — they are archived on load, not rejected; RT-upgrade-memory-removal-home owns that leg."
  coverage:
    surfaces:
      - "[roles] via compozy config set (global) + --scope workspace --workspace overlay"
      - "compozy__config_list|get|set|unset roles.* accept + removed-path reject; compozy__config_path scope-target proof only"
      - "routed invocation evidence (auto_title) incl. hidden-session visibility"
      - "docs entry origin: runtime/core/configuration/config-toml [roles] worked example matches observed behavior"
    invariants: [4, 7, 8, 9, 10, 11, 12]
    adrs: [ADR-002, ADR-003]
```

<!-- The charter is durable and immutable: each run's debrief belongs in that run's dated report. -->
