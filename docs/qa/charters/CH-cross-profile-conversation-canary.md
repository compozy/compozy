# CH-cross-profile-conversation-canary: Prove the owner tag never became a delivery filter

```yaml
charter:
  id: CH-cross-profile-conversation-canary
  mission: "As Ada, run a real conversation between two agents that live in different profiles and prove the network stamping changed only who owns a row, never who receives a message — nothing dropped, delayed, reordered, or filtered because the two sides carry different owners, and peers and infrastructure still machine-level."
  mode: charter-with-tour
  persona:
    name: Ada
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-run-bounded-live-collaboration
  scenarios: [NB-cross-profile-conversation]
  tour: Feature Tour
  time_box_minutes: 60
  guidance:
    must_try:
      - "Start one agent session in each of two profiles, have them hold a real back-and-forth, and account for every message on both sides — arrival, order, and latency. Any drop or stall must be traced to a cause other than the differing owners before it is dismissed."
      - "Confirm peers and infrastructure are listed once and identically from both profiles — no per-profile peer catalog and no duplicated infrastructure row — and that the network audit and permission logs are still readable rather than erased by the owner dimension."
      - "List channels, threads, direct rooms, and network work scoped to each profile and then with the explicit aggregate: every row belongs to the side that created it, and every aggregate row names its owner. Open one conversation from the profile that did not create it and confirm the scoped read returns not found while the aggregate-by-id read returns it owner-labeled."
      - "Deliver unattended work through a bridge instance owned by one profile while the other profile is active, and prove the produced work carries the instance's owner. Then archive one of the two profiles mid-conversation and confirm delivery behaves as documented rather than silently dropping messages."
    must_avoid:
      - "Re-walking the general Network participation defaults the NB scenarios already own; default COMPOZY_HOME or ports — use the isolated lab from the bootstrap manifest."
```

## Selection rationale

This is the cycle's **adjacent canary**: a journey the diff did not set out to change but whose
components it touched. Migration `00082` stamped the four network work roots and the bridge
instances, and Safety Invariant 12 states the consequence that must hold — network delivery is never
predicated on profile, and cross-profile conversation is regression-tested. The tag is meant to
answer "whose is this?", not "may this arrive?", and the failure mode is a filter added for
consistency in a place where consistency is wrong. ADR-010 keeps peers and infrastructure
machine-level with no permission vocabulary between profiles, and ADR-011 makes the entry point
rather than a routing table decide who owns unattended inbound work — both are claims about the
absence of a mechanism, which is precisely what a canary walk is for.

## Evidence and entry points

- **Agent** — paired session transcripts from both sides with message timestamps and a full accounting of sent versus received.
- **CLI** — the peer and infrastructure listing from each profile; scoped and aggregate listings for all four network work families; the audit and permission log reads.
- **HTTP and UDS** — the scoped-versus-aggregate detail pair for one conversation on both listeners.
- **Runtime** — the bridge-delivered work with its recorded owner while the other profile was active, and the delivery outcome at the moment one profile was archived.
- **Web** — none required.

<!-- The charter is durable and immutable: each run's debrief belongs in that run's dated report. -->
