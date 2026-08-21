# CH-loop-legibility-calm-catalog-parity: Prove one filtered set feeds every task-listing surface

```yaml
charter:
  id: CH-loop-legibility-calm-catalog-parity
  mission: "As Ada, list tasks during an active Loop across CLI, HTTP, UDS and the native tool and prove the calm default, the typed reveal, the facets, the counts and the documentation all compute over one server-side filtered set — with classification riding provenance columns, never an id string."
  mode: charter-with-tour
  persona:
    name: Ada
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-operate-loop-run-headless
  scenarios: [TA-task-list-calm-loop-default]
  tour: Feature Tour
  time_box_minutes: 60
  guidance:
    must_try:
      - "Run the identical query over CLI JSON, HTTP, UDS and compozy__task_list against the same persisted state and diff the results semantically — one filtered set or the contract is broken. Absent include_loop must behave as excluded; the calm read sends no include_loop at all, never an explicit false."
      - "Confirm facets, catalog counts, dashboard totals and inbox lanes all compute over the filtered population — a count that disagrees with its rows is the bug this exclusion exists to kill."
      - "Reveal with --include-loop, scope with --loop-run <run-id>, and drill with --parent <task-id>: --loop-run implies include, an unknown --loop-run returns an empty list at exit 0 because it is a filter and not a lookup, and --parent returns children with no flag at all. Read one coordinator and one cell through the single-task route and compare their loop provenance objects against the catalog rows, confirming no field is reconstructed from parsing a task id."
      - "Walk the deterministic failures and the documentation in the same pass: a non-boolean include_loop returns the field-addressed 400 invalid_query_field with field set, a cross-workspace id returns 404 rather than an empty success, and the official skill's task-listing reference plus the generated task list CLI page match what the runtime actually did — a documented flag that does not exist, or a default described wrongly, is a finding."
    must_avoid:
      - "Accepting a UI projection or a cached read as parity evidence; every comparison is a fresh structured read."
      - "Client-side filtering of any kind as an explanation for a matching result."
```

## Selection rationale

Targeted tier. Owns Safety Invariants 8, 9 and 10 with ADR-001 and ADR-004: exclusion is a
server-side SQL predicate shared by every listing surface, classification matches provenance columns
(`created_by`, `task_runs.loop_run_id`) and never an id or title string, and every new read is
workspace-scoped at the query layer. The Feature Tour is right because the whole claim is a promise
of sameness across four transports — the failure mode is a surface that drifted, not a surface that
crashed. Task 02 built one semantic fixture and matcher across all four transports; this session
walks it against a real daemon for the first time.

<!-- The charter is durable and immutable: re-run it in later cycles; each run's debrief goes in that run's report (Session Debriefs), never here. -->
