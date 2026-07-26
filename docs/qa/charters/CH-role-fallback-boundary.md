# CH-role-fallback-boundary: Fallback fires only before acceptance — ordered, once each, observable, and never after

```yaml
charter:
  id: CH-role-fallback-boundary
  mission: "As Ada, break role primaries in every pre- and post-acceptance way and prove the fallback chain runs serialized attempts once each in declared order, emits one durable correlated role.fallback.used event per attempt, exhausts into the role's deterministic error with zero residue, and never reroutes an accepted session."
  mode: charter-with-tour
  persona:
    name: Ada
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-route-background-work
  scenarios: [MS-background-role-fallback]
  tour: Network Tour
  time_box_minutes: 60
  guidance:
    must_try:
      - "Happy advance: point a role's primary at a provider that fails pre-acceptance (dead command in the isolated lab) with one healthy fallback entry; trigger the role — work completes on the fallback, and exactly one `role.fallback.used` (attempt index, target provider/model, workspace + parent-session correlation) is durably readable in the cross-session runtime logs via `agh logs --workspace <ref> --session <parent-session-id> --type role.fallback.used --last 10 -o json`."
      - "Order and single-try: a two-entry chain with a dead first entry — events show attempt 1 then attempt 2 in declared order, one try each, never concurrent."
      - "Exhaustion: all routes dead — the invocation surfaces the deterministic exhausted error; then prove zero residue: no session rows appear for failed attempts, session list and spawn capacity match the pre-trigger baseline (Invariant 5's nothing-to-clean-up claim, checked, not assumed)."
      - "The fence: force a failure after acceptance (kill the provider once the session is accepted) — the failure follows the normal session lifecycle and produces zero fallback attempts and zero new fallback events."
      - "Empty chain: no fallback_chain → single attempt, no event."
    must_avoid:
      - "Treating memory_controller as a live fallback surface — the current runtime makes no controller LLM call (config-only seam); record that branch as skipped with this reasoning."
      - "Chronic-fallback masking questions (provider health diagnosis) beyond confirming events make the pattern visible — that is an observability concern, not this boundary."
  coverage:
    surfaces:
      - "roles.<role>.fallback_chain config; forced pre-acceptance provider failure; forced post-acceptance failure"
      - "agh logs --workspace <ref> --session <parent-session-id> --type role.fallback.used --last 10 -o json and GET /api/logs?workspace_id=<id>&session_id=<parent-session-id>&type=role.fallback.used&limit=10 as the durable cross-session query surfaces (workspace_id + parent session_id correlation)"
      - "session list / spawn-capacity baseline comparison for residue"
    invariants: [5, 6, 8]
    adrs: [ADR-004]
```

<!-- The charter is durable and immutable: each run's debrief belongs in that run's dated report. -->
