# CH-agent-comms-roster-and-docs-truth: Learn subagents from the docs alone and delegate to one you just described

```yaml
charter:
  id: CH-agent-comms-roster-and-docs-truth
  mission: "As Lea, meet agent communications for the first time through the published docs, author a described specialist from them, and delegate to it — finding every place the docs promise something the runtime does not do, or the roster shows something the author did not write."
  mode: charter-with-tour
  persona:
    name: Lea
    device: laptop
    network: wifi-fast
    locale: en-US
  journey: J-build-a-subagent-roster
  scenarios: [SITE-agent-comms-docs-area, RT-subagent-roster-injection, RT-agent-roster-call-compose]
  tour: Feature Tour
  time_box_minutes: 60
  guidance:
    must_try:
      - "Read the five agent-comms pages cold and follow each tutorial against a live daemon, comparing the published transcripts against what the runtime actually prints. Task_07 recorded a known divergence — _dx.md's illustrative transcripts differ from the shipped human CLI output for await outcomes, list columns and the agents route, and runtime truth won by operator decision — so the specific job is spot-verifying the published text, not re-reading the spec."
      - "Check the claims the runtime has to back: nine call states, no default deadline, the idle clock suspending while a call is in flight, no read or seen state, publish being one-way, and the [calls] keys and defaults in the config reference matching what compozy config get returns."
      - "Author one workspace definition with a description, one without, and one global definition colliding by name. Then read the roster from all five surfaces — compozy agent list, GET /agents, compozy__agent_list, the compozy__agent_call parameter description, and the web catalog — and confirm they agree on name, description, scope, shadowed and digest. The undescribed one must render its gap rather than have one invented; the shadowed one must be marked rather than hidden."
      - "Prove the caps bound the view and not the registry: the injected parameter renders at most 32 definitions at 120 characters each while the full roster stays reachable through agent list. Separately, a description over the 500-character authoring maximum must fail the load with the bound named rather than being silently truncated into the roster. Add a description while the daemon runs and confirm it converges without a restart, because rendering happens at serve time."
      - "Delegate by picking a name straight out of the injected roster with no lookup turn, then misspell one and confirm call_agent_unknown prints the live roster with descriptions inline and a corrected try line. Then use the web Call compose: an invalid contract must fail inline with the daemon's own call_expect_invalid, an accepted call must link to its new record, a zero instance count must render nothing at all, and the zero-definitions and large-roster states must both hold."
      - "Close on the hard cut as a reader would meet it: sessions/orchestration must document delegation as calls with no spawn sections, aliases or 'formerly known as'; the spawn CLI reference page must be gone with its verb; the regenerated API reference must render the Calls and Messages operations rather than an empty section; and a grep of the docs tree and skills/compozy for spawn vocabulary must come back clean."
    must_avoid:
      - "Using knowledge of the runtime to get unstuck — when the docs are insufficient, that is the finding, and it should be recorded before looking elsewhere."
      - "Verifying transcripts against _dx.md; the spec is not the contract here, the shipped output is."
      - "Leaving lab processes alive; cite teardown.json with clean true."
```

## Selection rationale

Targeted tier. ADR-007 is the decision under test — explicit registry-name invocation against an
injected, shadow-aware roster with a new `description` field — and the whole value claim is that
selection costs zero extra turns. That claim is only testable by someone who does not already know
the definitions, which is why Lea rather than Bruno holds this charter even though two of its three
scenarios name Bruno and Ada as their primary personas.

Docs and roster travel together deliberately. The roster is a discovery surface and the docs area is
the other one; testing them apart would let a gap fall between them, where the documentation teaches
a selection flow the injected parameter does not actually support. Task_07 also left a live
verification debt this session is the natural home for: it verified quoted transcripts against the
shipped Go renderers and contract types rather than against a running daemon, and explicitly left
live spot-verification open for the QA tail.

The spawn hard cut appears here in its documentation half — page removal, reference regeneration,
vocabulary grep — while its runtime half (absent verb, route, tool, catalog entry, generated client)
belongs to `CH-agent-comms-containment-fence`. Splitting it that way keeps each session's evidence in
one register instead of asking one walk to hold both a live probe and a docs sweep.

<!-- The charter is durable and immutable: re-run it in later cycles; each run's debrief goes in that run's report (Session Debriefs), never here. -->
