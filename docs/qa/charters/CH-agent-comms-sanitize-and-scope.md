# CH-agent-comms-sanitize-and-scope: Plant secrets and foreign owners, then sweep every stage that runs after admission

```yaml
charter:
  id: CH-agent-comms-sanitize-and-scope
  mission: "As Dora, plant claim-token-shaped values and foreign-profile targets through calls, results, messages and extension hooks, then sweep every stage that runs after admission — validation, hook dispatch, repair prompts, events, logs, projections — and prove sanitization ran first and scoping ran at the store."
  mode: strategy-based
  persona:
    name: Dora
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-contain-and-audit-delegation
  scenarios: [RT-call-payload-sanitize-sweep, RT-call-profile-scope-isolation, ET-call-hooks-host-api-reads]
  tour: Garbage Tour
  time_box_minutes: 60
  guidance:
    must_try:
      - "Plant a claim-token-shaped value three ways — in a call prompt, in a returned result, and in a message body — then sweep every downstream sink for the raw value: stored payload, public projection, daemon log, SSE, canonical event, an installed extension's hook payload, and the repair prompt the child is shown. Only the redaction marker may appear, and correlation ids and hashes must survive intact. Checking one sink and stopping is the failure mode here; the invariant is about ordering, so it only means something once the later stages are checked."
      - "Aim at the two stages that would leak if sanitization ran second: make the payload fail validation so the validator error text is built from it — errors verbatim must mean verbatim from the sanitized output — and construct a payload where redaction cannot preserve contract validity, which must fail with a fixed typed error naming paths but never values."
      - "Create calls and messages in two profiles and two workspaces and try to reach across every seam in both directions, through CLI, HTTP, UDS, native tools and the web. Cross-workspace must be call_workspace_denied before any side effect; cross-profile typed calls must be denied; neither may raise a prompt, because delegation is not a consent seam."
      - "Confirm the deliberate exceptions rather than assuming there are none: --all-profiles aggregate reads return owner-labelled rows that authorize no mutation, Global scope with no workspace works, and Network publish keeps the profile-blind delivery exception — as the only one. Then switch profile in the web and confirm the query cache cannot serve the previous profile's calls, and that a foreign profile's call addressed by id is not-found rather than forbidden-with-a-hint."
      - "Install a real test extension declaring the whole call hook family and drive one call and one message through their lifecycles: exactly seven events must fire — created, settled, canceled, published, message_sent, message_delivered, subtree_drained — carrying the resolved profile owner and sanitized data only. Exercise a narrowing hook mutation (accepted, and re-validated AFTER the mutation) and a widening one (rejected with the atoms named), confirm the retained spawn-governance hooks still fire inside the call's spawn path, then take the extension down mid-call and confirm the path fails open."
      - "Exercise Host API calls/list, calls/get, calls/result and messages/list under calls:read, confirm there is no mutation method to reach in v1, and confirm a foreign profile's call is unreachable through the Host API too."
    must_avoid:
      - "Grepping only for the exact planted string — check hash-form and partial-prefix leakage as well, and confirm the marker is the canonical one rather than an ad-hoc mask."
      - "Using an in-process fake for the extension; the consent gate and fail-open behavior only mean something through a really installed one."
      - "Leaving lab processes alive; cite teardown.json with clean true."
```

## Selection rationale

Targeted tier. This session owns the three invariants that protect everything the feature touches
but that no functional walk would exercise: 10 (sanitize-before-everything — classification and
contract-preserving redaction run on the raw bytes as the *first* admission stage, before schema
validation, validator-error construction, hook dispatch, repair-prompt rendering, event emission and
persistence), 9 (every read, write and stream path filters by immutable profile, scope and workspace
at the store layer, with aggregate reads labelled and non-authorizing) and 7 (permission narrowing is
validated before *and re-validated after* every hook mutation). ADR-014 is the decision under test on
the scoping side, and the `call` hook family plus the `calls:read` Host API area on the extension
side.

The Garbage Tour is the right lens for all three: each is defined by what the system does with input
it should refuse to propagate — a secret, a foreign owner, a hook trying to grant itself more than it
was given. The three scenarios travel together because they share one setup cost (a second profile, a
second workspace, one installed test extension) and because a hook payload is simultaneously a
redaction sink and a scoping surface — testing them apart would mean building the same lab twice and
would still miss the case where a hook payload leaks what the projection redacted.

This is a sibling of `CH-secret-redaction-sweep`, not a duplicate of it: that charter owns *where*
planted secrets may surface across durable stores and streams on `J-keep-secrets-contained`; this one
owns *when* sanitization runs inside call admission. The distinction is recorded on
`RT-call-payload-sanitize-sweep` and cross-linked from both scenarios.

<!-- The charter is durable and immutable: re-run it in later cycles; each run's debrief goes in that run's report (Session Debriefs), never here. -->
