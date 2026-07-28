# CH-<slug>: <mission in one line>

```yaml
charter:
  id: CH-<slug>
  mission: "<one sentence — what we're looking for and why it matters>"
  mode: <charter-with-tour | freestyle | scenario-based | strategy-based | collaborative>
  persona:
    name: <from <qa-docs-path>/personas.md>
    device: <desktop | tablet | phone-small | phone-large>
    network: <wifi-fast | 4g | flaky>
    locale: <en-US | pt-BR | ...>
  journey: J-<slug>
  scenarios: [<scenario ids this session can settle>]
  tour: <exactly one — see qa-execution/references/tours.md>
  time_box_minutes: <30 | 60 | 90>
  guidance:
    must_try:
      - "<2-4 specific things to attempt>"
    must_avoid:
      - "<known-broken or out-of-scope areas>"
```

<!-- The charter is durable and immutable: re-run it in later cycles; each run's debrief goes in that run's report (Session Debriefs), never here. -->
