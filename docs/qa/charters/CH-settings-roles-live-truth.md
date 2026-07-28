# CH-settings-roles-live-truth: The Roles panel tells the truth, applies live, and survives every navigation I throw at it

```yaml
charter:
  id: CH-settings-roles-live-truth
  mission: "As Dora, work the Settings → Roles panel with hostile navigation — back, refresh, deep-link, dirty drafts — and prove it renders only what the daemon models (badges, honest nulls, no session-role timeout, no prompt editor), applies valid edits live with truthful confirmation, recovers from invalid fallback entries without losing my draft, and keeps builtins out of the fleet — while Memory settings no longer carries the role-owned controls."
  mode: charter-with-tour
  persona:
    name: Dora
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-route-background-work
  scenarios: [MS-settings-roles-panel, MS-026]
  tour: Back-Button Tour
  time_box_minutes: 60
  guidance:
    must_try:
      - "Truthful render: six roles in product order (Coordinator, Dream, Checkpoint summary, Memory extractor, Auto title, Memory controller) with BUILTIN/INHERIT/OFF states, 'Resolves at invocation.' on null fields, provenance chips only where the projection provides them, timeout input only on memory_controller, and no prompt customization control anywhere (ADR-003)."
      - "Live save: edit auto_title model → save bar → 'Saved · applied immediately' → hard reload → value persisted; then press back from the panel, return, and confirm no stale draft resurrects."
      - "Back-button hostility: dirty the form and navigate away/back (browser back, nav sidebar, refresh mid-save, bookmark /settings/roles and reopen) — a discarded draft never half-applies, and the last applied config remains authoritative (journey abandonment path)."
      - "Fallback editor recovery: add/remove/reorder entries; submit an entry missing provider/model — first invalid field focused, draft retained, nothing submitted; fix it and save clean."
      - "Diagnostics: with dream routed at a ghost agent, the row shows the role_agent_not_found warning (mono agent name); after repairing the route, a fresh projection clears it."
      - "Boundary checks: Agents fleet page shows neither coordinator nor dreaming-curator; Memory settings no longer offers controller-LLM/dream-enable/dream-agent/extractor-model controls while its retained runtime controls still edit and save (settles MS-026); spot-check the panel at a compact viewport width for layout survival."
    must_avoid:
      - "Full screen-reader/keyboard a11y depth — the panel composes the settings primitives already swept by the Sol settings charters (CH-011 lineage); record this as a deliberate skip and propose a follow-up Sol charter if any regression is suspected."
      - "Transport parity and provenance-layer proofs (CH-roles-projection-truthfulness owns them); config-file/CLI writes (CH-background-role-routing-scopes)."
  coverage:
    surfaces:
      - "Web /settings/roles (render, edit, fallback editor, diagnostics, compact viewport)"
      - "GET/PATCH /api/settings/roles apply path + GET /api/roles projection read"
      - "Live lifecycle confirmation + reload persistence + dirty-draft abandonment"
      - "Agents fleet builtin-hiding; Memory settings role-controls hard cut (MS-026)"
    invariants: [1, 4, 7, 11]
    adrs: [ADR-003, ADR-004]
```

<!-- The charter is durable and immutable: each run's debrief belongs in that run's dated report. -->
