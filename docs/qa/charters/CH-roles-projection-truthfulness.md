# CH-roles-projection-truthfulness: The roles projection never guesses — parity, provenance, and honest nulls everywhere

```yaml
charter:
  id: CH-roles-projection-truthfulness
  mission: "As Ada, read the six-role projection through every transport and prove CLI, HTTP, and UDS agree field-for-field, provenance names each field's true winning layer, inherit-mode values stay null instead of fabricated, diagnostics surface broken routes on a 200, and unknown roles fail with role_unknown — with the docs describing exactly this contract."
  mode: charter-with-tour
  persona:
    name: Ada
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-route-background-work
  scenarios: [MS-inspect-background-role-routing]
  tour: Feature Tour
  time_box_minutes: 60
  guidance:
    must_try:
      - "Three-way parity: `agh roles list -o json`, HTTP `GET /api/roles?workspace=<id>`, and the UDS route return the same six roles with identical fields; repeat for the single read `agh roles show dream -o json` vs `GET /api/roles/dream` on both transports."
      - "Per-field provenance: set one field globally and a different field in a workspace overlay — each non-null routed field reports its own layer (default/global/workspace), including a workspace override that repeats the global value (must still say workspace); fields the projection omits get no invented provenance."
      - "Honest nulls: auto_title and memory_extractor in inherit mode show agent null and invocation-dependent fields null — nothing fabricated; timeout appears only on memory_controller."
      - "Diagnostics without failure: route dream at a ghost agent — list/show still return 200/exit 0 with a `role_agent_not_found` diagnostic naming the missing agent; `agh roles show bogus` exits non-zero with `role_unknown`, and HTTP/UDS return 404 with the `role_unknown` body code."
      - "Truth boundary: while builtins drive dream/checkpoint, `GET /api/agents?workspace=<id>` and `GET /api/agents/catalog?workspace=<id>` stay builtin-free (Invariant 1 re-check from the read side); the api-reference/roles and config-toml docs pages claim exactly the fields and semantics observed — flag any doc claim the runtime does not honor."
    must_avoid:
      - "Writing routes (CH-background-role-routing-scopes owns mutations beyond the minimal overlay needed for provenance evidence) and the web panel rendering (CH-settings-roles-live-truth)."
      - "Inventing a context-specific resolver expectation — the projection is configuration, not an invocation simulation; do not fail it for not predicting inherit-mode outcomes."
  coverage:
    surfaces:
      - "agh roles list|show -o json; GET /api/roles + GET /api/roles/{role} over HTTP and UDS"
      - "role_unknown 404 contract; role_agent_not_found diagnostics on 200"
      - "per-field provenance truth incl. workspace-equal-to-global; inherit nulls; timeout only in-process"
      - "agents catalog builtin-hiding re-check; docs runtime/api-reference/roles + config-toml [roles] parity"
    invariants: [1, 4, 11]
    adrs: [ADR-001, ADR-003]
```

<!-- The charter is durable and immutable: each run's debrief belongs in that run's dated report. -->
