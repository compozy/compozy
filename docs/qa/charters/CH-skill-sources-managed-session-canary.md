# CH-skill-sources-managed-session-canary: Prove an agent can still reach the skill the prompt left out

```yaml
charter:
  id: CH-skill-sources-managed-session-canary
  mission: "As Ada in a real managed session whose prompt catalog omits an installed skill, load that skill's exact body through the native seam and prove the skill-sources cycle did not break the one path that exists for skills the prompt does not carry — while managed authority stays exactly as narrow as it was."
  mode: charter-with-tour
  persona:
    name: Ada
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-load-skill-in-managed-session
  scenarios: [ET-managed-session-skill-loading]
  tour: Feature Tour
  time_box_minutes: 60
  guidance:
    must_try:
      - "Install a marker-bearing workspace skill, record its exact body, and delay a real managed session's startup past the hosted-MCP bind TTL so the catalog it receives genuinely omits the skill. Then ask the agent to load it and confirm compozy__skill_view returns the exact marker-bearing body — byte-for-byte against what was recorded, not merely a plausible one."
      - "Confirm the omission and the recovery are separable. Read the harness diagnostics and establish whether this skill was omitted by truncation or by provider-aware suppression; both must be reachable through the same native seam, and a suppressed skill must be no harder to load than a truncated one."
      - "Read the new fields on the way through: the skill_view header must carry origin, owner_scope, and exposures[] with statuses inside healthy|missing|broken|foreign_conflict, and the values must agree with what the operator sees from compozy skill info for the same skill."
      - "Confirm managed authority did not widen. Every compozy skill command must still fail inside the managed environment before resolution or UDS access, the transcript must show no CLI call and no direct file read, and an operator must still read the same body with compozy skill view."
      - "Compare both reads at the end. The agent's body and the operator's body must match; a divergence means the native seam and the operator seam resolved different copies, which is exactly the failure the origin and precedence work could have introduced."
    must_avoid:
      - "Substituting a seeded fixture or a short-circuited session for a real delayed managed session. The point of a canary is that it walks the untouched path for real."
      - "Drifting into the suppression matrix itself — provider lanes, per-folder granularity, qualified homonyms belong to CH-skill-session-suppression-matrix. A finding there is a follow-up, not this debrief."
      - "Treating a successful load as sufficient without diffing the body. A truncated or wrong-copy body that loads without error is the regression this session exists to catch."
```

## Selection rationale

This is the targeted tier's mandatory adjacent canary, and the journey choice is the whole argument.
The skill-sources cycle changed how skills are discovered, attributed, and injected; it was never
supposed to change how a managed agent recovers a skill the prompt did not carry. That recovery path
is this journey's entire promise, and it is the place where a suppression bug becomes invisible:
before this cycle, a skill missing from a managed catalog meant truncation, and now it can also mean
deliberate provider-aware suppression. If the native seam degrades, the symptom is an agent that
simply cannot find a skill — with no error, no diagnostic the operator will look at, and a prompt
that looks intentionally lean.

Two concrete things in the diff reach into it. `compozy__skill_view` — the exact tool this journey
depends on — gained `origin`, `owner_scope`, and `exposures[]` in its header, with regenerated
descriptors and schema digests. And the eight-tier precedence work changed which physical copy wins a
name, so the agent's read and the operator's read could now resolve different directories while both
appear to succeed. That is why diffing the two bodies is a must-try rather than a formality.

`ET-managed-session-skill-loading` was `pass` on a build that predates this cycle; the planner reset
it because a stale pass on the canary would defeat the purpose of having one. Marketplace
install/remove was the other natural canary candidate and was deliberately not chosen: the diff
genuinely changed the marketplace installed card and detail, which makes it collateral inside
`CH-skill-expose-web-repair` rather than an untouched path worth proving.

Ada owns it because the journey is defined by a non-human actor operating through the native seam
with no operator CLI available to fall back on.

<!-- The charter is durable and immutable: re-run it in later cycles; each run's debrief goes in that run's report (Session Debriefs), never here. -->
