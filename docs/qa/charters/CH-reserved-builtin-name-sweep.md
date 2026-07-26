# CH-reserved-builtin-name-sweep: No surface lets me author, smuggle, or shadow a builtin identity

```yaml
charter:
  id: CH-reserved-builtin-name-sweep
  mission: "As Ada, attack every agent-authoring path with the reserved names coordinator and dreaming-curator — CLI, HTTP, UDS, native tool, duplicate, rename, bundle, and an on-disk shadow directory — and prove each rejects with agent_name_reserved leaving zero residue, while the builtins themselves never surface in any catalog."
  mode: charter-with-tour
  persona:
    name: Ada
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-32
  scenarios: [RT-reserved-builtin-agent-names]
  tour: Garbage Tour
  time_box_minutes: 60
  guidance:
    must_try:
      - "Create sweep: `agh agent create coordinator` (CLI→UDS), `POST /api/agents` over HTTP with dreaming-curator, and native `agh__agent_create` — each rejects with the exact `agent_name_reserved` envelope (422-class), creates no directory, and leaves `agh agent list` byte-stable."
      - "Mutation sweep: rename an existing agent to a reserved name and duplicate onto a reserved target — both reject; the source agent stays untouched."
      - "Normalization edges: case/whitespace variants (`Coordinator`, ` coordinator `) reject; near-miss `coordinator-helper` succeeds and is cleaned up — reservation is exact-name after normalization, not prefix."
      - "Bundle path: activate a bundle whose profile ships an agent named coordinator — activation fails with `agent_name_reserved` naming the bundle path and materializes nothing from that profile."
      - "Shadow path: plant `$AGH_HOME/agents/coordinator/AGENT.md` before boot — boot succeeds, a warning diagnostic names the skipped path, the directory never enters agent list/catalog, and the coordinator role still resolves the virtual builtin (no shadow resolution)."
      - "Catalog hiding throughout: after every attempt, `GET /api/agents?workspace=<id>` and `GET /api/agents/catalog?workspace=<id>` (HTTP and UDS) and the fleet UI contain neither builtin name (Invariant 1)."
    must_avoid:
      - "General agent CRUD depth (J-32's duplicate/delete/restart lifecycle beyond the reservation fence) — CH owns only the reserved-name boundary."
      - "Deleting or modifying the operator's real agents; use disposable names for the near-miss probe."
  coverage:
    surfaces:
      - "agh agent create|update|duplicate (CLI/UDS); POST/PUT /api/agents (HTTP); agh__agent_create; bundle activation"
      - "boot-time discovery skip of a pre-existing reserved directory + its warning diagnostic"
      - "GET /api/agents?workspace=<id> + GET /api/agents/catalog?workspace=<id> (HTTP/UDS), fleet UI catalog hiding"
      - "docs entry origin: runtime/core/configuration/agent-md reserved-names note matches enforcement"
    invariants: [1, 2, 3]
    adrs: [ADR-001, ADR-004]
```

<!-- The charter is durable and immutable: each run's debrief belongs in that run's dated report. -->
