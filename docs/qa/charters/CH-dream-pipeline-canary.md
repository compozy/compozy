# CH-dream-pipeline-canary: Default background memory still works end-to-end after the roles rewiring

```yaml
charter:
  id: CH-dream-pipeline-canary
  mission: "As Dora, walk the untouched-default memory pipeline end-to-end — real session work, extractor harvest, dream trigger, hidden dreaming-curator run, health, recall — as the adjacent canary proving the six-consumer roles rewiring caused zero default-behavior drift."
  mode: charter-with-tour
  persona:
    name: Dora
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-digest-sessions-into-memory
  scenarios: [MS-011, MS-016, MS-017]
  tour: Feature Tour
  time_box_minutes: 30
  guidance:
    must_try:
      - "On pristine default [roles] config, do real session work, confirm extractor evidence (`agh memory extractor status` / `list-pending`), then `agh memory dream trigger` — a truthful running (or skipped+reason) response, never a fake run."
      - "The dream session runs the builtin dreaming-curator hidden: absent from session list, fleet, and agent catalogs while `agh memory dream status|show <date-or-run-id>` reports it truthfully to a terminal outcome (retry available on failure)."
      - "Pipeline health: `GET /api/memory/health` (and `agh memory health`) returns ok with real counts after the run; a fresh memory list/recall returns the consolidated knowledge."
      - "Disabled branch: set `roles.dream.enabled false`, trigger → skipped naming the reason; re-enable → next trigger runs — both without daemon restart (live roles apply on the pipeline path)."
    must_avoid:
      - "Routing overrides beyond the single enabled-toggle probe (CH-background-role-routing-scopes owns routing) and fallback failure injection (CH-role-fallback-boundary)."
      - "Knowledge-catalog depth — paging, FTS recovery, identity isolation belong to J-25's charters."
      - "Memory-v2 dream promotion — out of the feature's scope by spec."
  coverage:
    surfaces:
      - "session work → extractor status/list-pending → POST /api/memory/dreams/trigger + agh memory dream trigger|status|show|retry"
      - "GET /api/memory/dreams*; GET /api/memory/health + agh memory health; fresh memory list/recall"
      - "hidden-session visibility across session list, fleet, and agent catalogs"
      - "live roles.dream.enabled toggle on the real pipeline"
    invariants: [10, "default-preservation regression (TechSpec Known Risks: default-behavior drift)"]
    adrs: [ADR-001]
```

<!-- The charter is durable and immutable: each run's debrief belongs in that run's dated report. -->
