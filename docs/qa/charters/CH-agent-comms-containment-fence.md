# CH-agent-comms-containment-fence: Make each wall refuse in its own shape, and make the deleted one stay deleted

```yaml
charter:
  id: CH-agent-comms-containment-fence
  mission: "As Dora, set every [calls] limit and then attack it — recursive delegation past the depth wall, more children than the cap, more work than the budget, and every deleted spawn entry point — and prove each refuses in the exact shape it promises rather than in whichever shape is convenient."
  mode: strategy-based
  persona:
    name: Dora
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-contain-and-audit-delegation
  scenarios: [RT-delegation-depth-and-caps, RT-calls-config-effects, RT-session-spawn-removed]
  tour: Error Guessing Tour
  time_box_minutes: 60
  guidance:
    must_try:
      - "Keep the three refusal shapes apart, because conflating them is the bug this session exists to find. max_children is an admission wall and REJECTS with call_children_cap naming the cap and the current count; max_active_per_root is an execution budget and QUEUES visibly as a durable queued call a list read can see; max_batch rejects the whole batch with nothing partial run. Any wall behaving like another is a finding even if the work is contained."
      - "Delegate recursively to calls.max_depth and confirm the call tool is absent from the walled child's toolset entirely — not present and refusing. Confirm each child's context states its literal remaining depth, then plant a forged depth claim in a prompt and confirm depth still comes from durable lineage."
      - "Read and write every [calls] key through all three agent-manageable surfaces — CLI, native tool, config.toml — sequentially against the isolated home, never concurrently. Confirm the four overlays resolve as documented by setting the same key at two layers and checking which wins, and confirm max_depth applies to new calls while an in-flight call keeps its immutable snapshot."
      - "Check the lifecycle claims that are easy to state and hard to hold: there is no default deadline to configure; idle_ttl applies at call time and suspends while a call is in flight; overflow=store keeps the whole payload with bounded previews while overflow=reject fails with call_result_over_budget naming the declared budget."
      - "Probe every deleted spawn entry point — the CLI verb, the HTTP route, the UDS route, compozy__session_spawn, the native catalog and its schema digests, the generated OpenAPI document and TypeScript clients, and the generated CLI reference. Each must respond as genuinely absent, with no alias, shim, or hint redirecting to compozy call. Confirm boot-time bijective native-tool registration passes without it."
    must_avoid:
      - "Concluding a wall works because work stopped happening — name which code refused, on which surface, with which numbers in it."
      - "Treating the retained internal child-session engine as a surviving spawn surface; this session is about the deleted public one."
      - "Leaving lab processes alive; cite teardown.json with clean true."
```

## Selection rationale

Targeted tier. This session owns invariant 8 (depth enforced at create admission from durable
lineage, never from prompt claims, with the tool absent at the wall) and the config-lifecycle half of
the feature, and it carries ADR-002 (one agent-facing call verb; the spawn surface is deleted),
ADR-007's batch cap, ADR-008 (recursive delegation to a default depth of 3, contained by budgets) and
ADR-011 (accounting-only activations, which is why the per-root budget queues instead of rejecting).

The Error Guessing Tour fits because the risk is not that containment is missing — it is that two
similar-looking limits collapse into one behavior under pressure, and a tester who is not
specifically hunting for the wrong refusal shape will happily record "contained" and move on. A wall
that queues when it promised to reject is invisible to a functional check and very visible to an
agent author debugging why their fan-out silently stalled.

`RT-session-spawn-removed` sits here rather than in a documentation session because a hard cut is a
runtime property before it is a docs property: the greenfield rule forbids aliases and shims, and the
place to prove that is the live surface inventory. Its docs half is owned by
`CH-agent-comms-roster-and-docs-truth`.

<!-- The charter is durable and immutable: re-run it in later cycles; each run's debrief goes in that run's report (Session Debriefs), never here. -->
