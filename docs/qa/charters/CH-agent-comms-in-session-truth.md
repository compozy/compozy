# CH-agent-comms-in-session-truth: Read a delegation entirely from inside the conversation it happened in

```yaml
charter:
  id: CH-agent-comms-in-session-truth
  mission: "As Théo, return to a session that delegated and was delegated to, and reconstruct the whole story — the ask, the messages, the wake, the calls in both directions, the evidence published to a channel — from the transcript and the inspector alone, without a count or an order the daemon did not produce."
  mode: scenario-based
  persona:
    name: Théo
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-supervise-delegation-trees
  scenarios: [RT-in-context-call-messages, RT-session-calls-inspector-panel, NB-agent-call-publish]
  tour: Feature Tour
  time_box_minutes: 60
  guidance:
    must_try:
      - "Open a child session and confirm it shows the ask that started it and its bound call context, then open its caller and confirm the compozy__agent_call turn renders as a call card and a batch as one fan-out card. Confirm the order is the daemon's durable order rather than a timestamp splice — interleave a message and a call closely in time and check the sequence survives a reload."
      - "This is the mailbox's only in-context home, so exercise it here: a message must render provenance-stamped ('from agent reviewer (ses_…), not the operator') inside a bounded untrusted frame, with a delivery receipt that transitions in place from queued to delivered or woke on the same record. Put an embedded command and an approval-looking phrase in a body and confirm both arrive inert and cannot approve a pending permission."
      - "Confirm the completion wake row carries the daemon's own wake line verbatim, with the call identity and preview — compare it character for character against the runtime output rather than accepting a plausible paraphrase. Then confirm no read or seen state renders anywhere in the transcript, because the runtime models none."
      - "Open the inspector Calls tab and confirm both directions are listed and distinguished by arrow rather than colour, each carrying its own daemon count for its own exact filter while fewer rows are loaded. Confirm message pages render no total at all — bounded loading wording, never a page length standing in for a count. Prune a counterpart session and confirm the row keeps its identity and state while the jump link goes absent rather than 404ing."
      - "Publish a completed call into a Network channel thread from this session's context, then publish the same call to the same conversation again — the replay must return the recorded message id with published false rather than posting twice — and to a different conversation, which publishes anew. Then attempt every reverse path: nothing in Network may mutate, reopen or annotate the call it came from."
      - "Attempt to publish from each non-completed state, including every resultless terminal, and confirm call_publish_not_settled; attempt without active Live participation and confirm call_publish_no_participation; confirm channel-thread conversations are the only target and there is no direct-room publish."
    must_avoid:
      - "Reconstructing the story from Activity or the CLI and then confirming the transcript agrees — the point of this session is whether the conversation alone is sufficient."
      - "Accepting a receipt from the compose response; re-read it from the durable record after a reload."
      - "Leaving lab processes alive; cite teardown.json with clean true."
```

## Selection rationale

Targeted tier. This is the surface a returning user actually lands on, and it carries the design
decision that most changed the shape of the feature: S3, the standalone inbox location, was cut on
2026-08-23 because the mailbox is a runtime channel rather than a place. That makes the transcript
and the inspector the *only* in-context home for messages, so if provenance stamps, inert framing or
delivery receipts are wrong here, they are wrong everywhere a human would look. Théo is the session
hero persona and the right one for a scenario-based walk that reconstructs a story rather than
probing a control.

`NB-agent-call-publish` belongs with these two rather than in a Network-owned session because publish
is *our* surface — a one-way bridge out of the call domain, and the reason the adjacent canary in this
cycle is Loops rather than Network. ADR-005 is the decision under test, and its negative half (nothing
ever flows back from Network into a call) is only provable by trying the reverse paths deliberately,
which is why the guidance names them.

The two truthfulness rules from task_06 are the standing regression risk this charter guards: counts
come from `CallsResponse.total` on an exact-filtered read, and `MessagePage` carries no total at all,
so any message count on screen is invented. Both would pass a functional check and fail a user.

<!-- The charter is durable and immutable: re-run it in later cycles; each run's debrief goes in that run's report (Session Debriefs), never here. -->
