# CH-background-role-routing-scopes: Routing changes exactly the scope I chose — live, bounded, and nowhere else

```yaml
charter:
  id: CH-background-role-routing-scopes
  mission: "As Dora, drive the whole [roles] write plane — global config set, workspace overlay, and the native agh__config_* tools — and prove every accepted route becomes live for the next eligible invocation in exactly the chosen scope, every invalid or deleted key is rejected with its exact path, and routed catalog agents still behave as their role."
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
      - "Baseline first: `agh config list -o json` (read the exact roles.* leaves — `agh config get` takes one flattened leaf like roles.dream.model, never the roles branch) and `agh roles list -o json` on a fresh home must show the pinned defaults (coordinator off; dream/checkpoint on builtin dreaming-curator; auto_title/extractor inherit; controller haiku@250ms)."
      - "Global route: `agh config set roles.dream.model <m>` then trigger a dream (`agh memory dream trigger`) — the hidden dream session resolves the routed model with the builtin identity, and the session stays out of fleet/session lists (Invariant 10)."
      - "Workspace route: `agh config set --scope workspace --workspace <root> roles.dream.agent <local-curator>` — that workspace's next dream runs the catalog agent (AGH role overlay still applied, ADR-003/Invariant 12) while a sibling workspace keeps global routing; survive a fresh config read (Invariant 11)."
      - "Live toggle: `roles.auto_title.enabled false` → next session gets no title spawn; re-enable → titles resume, all without daemon restart."
      - "Native plane: `agh__config_list` serves the exact roles.* leaves and `agh__config_get roles.auto_title.model` reads one; `agh__config_set roles.auto_title.model` accepted and `agh__config_unset` restores the inherited value; `agh__config_path` proves only the selected global/workspace config file target and scope; every removed path (`memory.dream.agent`, `memory.controller.llm.model`, `session.auto_title_enabled`, `autonomy.coordinator.*`) rejected deterministically by both `agh config set` and `agh__config_set` (Invariant 9)."
      - "Rejection quality: `roles.coordinator.max_children 6` names the ≤5 cap (Invariant 8); `roles.dream.timeout` in TOML fails load naming `roles.dream.timeout` (Invariant 7); an old deleted key inside config.toml fails load naming the key; the prior good config stays authoritative after every rejection."
      - "Ghost route: `roles.dream.agent ghost` — the next invocation fails explicitly (`role_agent_not_found` / role.resolve.error), with no silent builtin fallback (Invariant 4)."
    must_avoid:
      - "The Settings web panel (CH-settings-roles-live-truth owns it) and fallback chains under failure (CH-role-fallback-boundary owns them)."
      - "Editing role policy knobs that stayed subsystem-side (dream cadence/gates, extractor pipeline) — the routing-vs-policy split means they are out of mission."
  coverage:
    surfaces:
      - "[roles] via agh config set (global) + --scope workspace --workspace overlay"
      - "agh__config_list|get|set|unset roles.* accept + removed-path reject; agh__config_path scope-target proof only"
      - "routed invocation evidence (dream, auto_title) incl. hidden-session visibility"
      - "docs entry origin: runtime/core/configuration/config-toml [roles] worked example matches observed behavior"
    invariants: [4, 7, 8, 9, 10, 11, 12]
    adrs: [ADR-002, ADR-003]
```

<!-- The charter is durable and immutable: each run's debrief belongs in that run's dated report. -->
