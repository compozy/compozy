# CH-terminal-docs-first-success: Follow the published terminal pages literally and see where they stop being true

> Superseded on 2026-09-04 by `CH-terminal-shared-control-docs`. Preserve this file as historical QA
> memory; do not schedule its removed terminal-scoped typing-grant checks.

```yaml
charter:
  id: CH-terminal-docs-first-success
  mission: "As Lea meeting the terminal for the first time, work only from the published pages and the generated reference — never from the specification — and find the first place where the documentation and the running daemon disagree; then read the same facts as an agent would from the official skill."
  mode: charter-with-tour
  persona:
    name: Lea
    device: laptop
    network: wifi-fast
    locale: en-US
  journey: J-learn-terminal-from-docs
  scenarios: [SITE-terminal-docs-truth, ET-compozy-official-skill-discovery]
  tour: Feature Tour
  time_box_minutes: 60
  guidance:
    must_try:
      - "Start at the Terminal overview, confirm its navigation lists exactly the published pages, then run the tutorial verbatim against a real runtime, substituting only what it tells you to substitute. Every printed command must run and every documented result shape must match."
      - "Check the safety page against behaviour: the approval tiers, the per-terminal typing grant, and the instruction that terminal output is data rather than authority."
      - "Check the journal, recording, profile, and platform pages against behaviour — what is always kept, what dies with the terminal, what archiving closes versus keeps, and whether your own project can open an interactive terminal at all."
      - "Open the generated command-line reference and compare it against what the tool accepts: a documented verb the tool rejects, or an accepted verb the page omits, is the finding."
      - "Read the official skill's terminal reference the way an agent would, through the router; confirm it resolves on every read plane and that its tool identifiers, platform facts, and safety rules agree with both the daemon and the public pages."
    must_avoid:
      - "Do not consult the specification, the task files, or the source to resolve an ambiguity — an ambiguity a reader cannot resolve from the page is itself the finding."
```

## Focus areas

- **Documentation truth** — the published pages describe the terminal the daemon runs; runtime behaviour
  wins over aspirational wording, and no page may claim a capability the runtime refuses.
- **First-run success** — the tutorial works verbatim for someone who has never seen the feature, and the
  platform limit is stated before a reader could hit a failure.
- **Agent-facing guidance** — the official skill's terminal reference teaches the same tool identifiers,
  platform facts, and safety rules as the public pages, and the router actually resolves it.
- **Generated reference integrity** — the generated command reference carries every verb the tool accepts
  and no verb it does not.

<!-- The charter is durable and immutable: re-run it in later cycles; each run's debrief goes in that run's report (Session Debriefs), never here. -->
