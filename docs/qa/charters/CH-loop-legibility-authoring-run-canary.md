# CH-loop-legibility-authoring-run-canary: Prove the authoring-to-run path is untouched by the legibility diff

```yaml
charter:
  id: CH-loop-legibility-authoring-run-canary
  mission: "As Bruno, author a fan-out Loop, run it, and follow the CLI's link into the web run it created — the adjacent journey this cycle's diff was never supposed to touch — and prove nothing in the exclusion, read-layer or run-page work broke it."
  mode: charter-with-tour
  persona:
    name: Bruno
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-complete-partial-loop
  scenarios: [LP-loop-run-deep-link, LP-fanout-progress-naming]
  tour: Feature Tour
  time_box_minutes: 60
  guidance:
    must_try:
      - "Run compozy loop run non-dry and confirm the effective daemon URL for /loop-runs/<run_id> is the final human-readable line, that web_url matches in JSON and TOON, and that the URL opens the matching persisted run. Confirm a dry run still emits no URL in any format."
      - "Author a nested fan-out with bind_as and index_as, confirm the declared names round-trip without shadowing reserved roots and are visible only inside the body, then read the qualified nodes.<fanout>.progress and the body-local alias consistently through routing, gating, CLI, HTTP, native tools and the web."
      - "Include one nesting-name collision and one reserved-root collision as recoverable authoring errors — the lint must name the exact field and leave the draft intact."
      - "Confirm the loop DSL reference and the official skill's loops reference still describe the authoring surface truthfully after this cycle's documentation regeneration."
    must_avoid:
      - "Drifting into the run page's two registers or the Tasks exclusion — those are owned by their own charters, and a finding there belongs to a follow-up, not this debrief."
      - "Substituting a seeded fixture for a real authored run; the point of a canary is that it walks the untouched path for real."
```

## Selection rationale

The targeted tier's mandatory adjacent canary. This cycle's four fronts all changed how loop work is
*read*; authoring and kickoff were explicitly out of scope, and this session exists to prove that
claim rather than assume it. The two scenarios are the shared-surface seams most likely to take
collateral damage: `LP-loop-run-deep-link` crosses the CLI-to-web boundary the run page was
redesigned behind, and `LP-fanout-progress-naming` reads the same progress values the roster and
briefing now serve — both were reset by task_05's diff without being named in any task's QA-impact
line, so this charter is what keeps them from going unwalked. A regression here means the read
redesign leaked into the authoring path, which is the highest-cost outcome this program could have.

<!-- The charter is durable and immutable: re-run it in later cycles; each run's debrief goes in that run's report (Session Debriefs), never here. -->
