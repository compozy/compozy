# J-22 — Provider settings & display surfaces stay truthful (adjacent canary)

The cycle's adjacent canary (`qa-report` flows-before-matrix; `model-selector` _spec §6.2 display-only inventory, invariant §7.3). The settings/catalog projection was unified (`internal/settings/models.go` now delegates to the modelcatalog service) and the catalog gained curation columns — a change that does not touch the selector but rides through every surface that *reads* provider/model data. This canary walks the display-only surfaces to prove the refactor didn't quietly break the settings inspector, the provider edit form, the model-catalog status card, or the session/task summaries that echo provider·model labels and logos.

```mermaid
flowchart TD
    E1[Entry: Settings → Providers] --> LIST[Provider list · summaries + auth state]
    LIST --> INSPECT[Inspect a provider · projected curated models]
    INSPECT --> STATUS[Model catalog status card · source freshness]
    LIST --> EDIT[Edit provider · GET→PUT preserves raw membership]
    EDIT --> PUT{Unchanged GET→PUT}
    PUT -->|no membership intent| NOOP[Server-owned raw overlay preserved]
    PUT -->|explicit empty curated[]| CLEAR[Authoritative clear, fails closed if catalog unavailable]
    LIST --> DISPLAY[Display-only echoes: agent-sessions-list, tasks-execution-profile-card, provider-logo]
    DISPLAY --> CONSIST{Labels + logos consistent with catalog?}
    CONSIST -->|yes| OK[No regression]
    CONSIST -->|drift| BUG[Projection/display regression]
    OK --> BACK[Navigate to another settings tab → browser Back → refresh the route]
    BACK --> REREAD[Fresh re-read of every read-side surface renders identical projected data + brand marks; the no-op edit wrote nothing back to raw config — true_end_state]
    INSPECT -.->|degraded catalog| DEGRADE[Stale rows shown honestly; no crash, no empty firehose]
```

```yaml
journey:
  id: J-22
  name: "Provider settings & display surfaces stay truthful (adjacent canary)"
  value_statement: "The parts of the app I didn't change — settings inspector, edit form, status card, session/task summaries — still show the same truthful provider and model data after the catalog refactor."
  personas: [Marina]
  entry_points:
    - url: "web Settings → Providers (list / inspect / edit)"
      origin: in-app-nav
    - url: "web session list + task execution profile card (display-only echoes)"
      origin: in-app-nav
  actions:
    - step: 1
      verb: "Open the provider list and inspect a provider"
      expected_observable: "Provider summaries show auth state and the projected curated models — the same merged rows the catalog serves (single projection), not a raw curated-config echo."
    - step: 2
      verb: "Read the model-catalog status card"
      expected_observable: "Source freshness/stale is reported truthfully; a degraded catalog shows stale rows honestly without crashing or emptying."
    - step: 3
      verb: "Edit a provider and PUT unchanged"
      expected_observable: "An unchanged GET→PUT is a no-op for source metadata (raw membership preserved); a present empty curated[] is an authoritative clear that fails closed when the catalog is unavailable."
    - step: 4
      verb: "Check display-only echoes (session list, task execution profile, provider logos)"
      expected_observable: "Provider·model labels and brand logos are consistent with the catalog; the new KindIcon marks resolve; no surface renders a stale or wrong provider name/logo after the refactor."
  goal:
    observable: "Every read-side surface that consumes provider/model data stays truthful and consistent post-refactor; no regression in the settings lifecycle or the display echoes."
    side_effects: [settings-projection-read, provider-put-membership-preserved]
  true_end_state: "After navigating away, returning with browser Back, and refreshing, every read-side surface shows the same freshly projected data; a provider edit that changed nothing wrote nothing back into raw config; logos/labels match the catalog everywhere."
  exit:
    natural: "The evaluator confirms the refactor was invisible to the surfaces it wasn't supposed to change."
  abandonment:
    - at_step: 3
      how: "Evaluator edits a fallback-only provider's membership while catalog sources are unavailable."
      resume: "The explicit membership change fails without writing rather than materializing a partial/wrong set."
  crosses: [settings-projection, provider-put-membership, model-catalog-status, kind-icon-registry, display-only-surfaces]

design_reference:
  screens:
    - "settings provider-card-summary / provider-inspect-view / provider-edit-form / provider-model-catalog-status; agent-sessions-list; tasks-execution-profile-card; provider-logo"
  truthful_ui_checks:
    - "Settings models projection equals catalog projection for the same provider (single projection — invariant §7.3)."
    - "Unchanged GET→PUT preserves raw operator metadata; present empty curated[] clears authoritatively and fails closed when sources are unavailable."
    - "New KindIcon provider marks (cursor/opencode/openrouter/xai/groq/zai/kiro/pi/mistral/minimax/moonshot) resolve; providers without marks keep the lucide fallback (no invented marks)."

e2e_backbone:
  integration:
    - "Settings projection equals catalog projection; provider PUT distinguishes omitted vs present-empty curated (task 01 suite)."
  manual:
    - "Charter CH-033 (Marina) walks the settings inspect/edit + display-only echoes as the regression canary."
```
