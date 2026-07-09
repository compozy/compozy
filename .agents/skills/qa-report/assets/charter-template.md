# CH-<NNN>: <mission in one line>

```yaml
charter:
  id: CH-<NNN>
  mission: "<one sentence — what we're looking for and why it matters>"
  mode: <charter-with-tour | freestyle | scenario-based | strategy-based | collaborative>
  persona:
    name: <from <qa-docs-path>/personas.md>
    device: <desktop | tablet | phone-small | phone-large>
    network: <wifi-fast | 4g | flaky>
    locale: <en-US | pt-BR | ...>
  journey: J-<NN>
  scenarios: [<state.csv ids this session can settle>]
  tour: <exactly one — see qa-execution/references/tours.md>
  time_box_minutes: <30 | 60 | 90>
  guidance:
    must_try:
      - "<2-4 specific things to attempt>"
    must_avoid:
      - "<known-broken or out-of-scope areas>"
```

<!-- The charter is durable: re-run it in later cycles and append a fresh debrief per run. -->

## Debrief — <YYYY-MM-DD> (<report path>)

- **Ran:** <started> → <ended> (box respected: yes/no)
- **Findings:**
  - <finding, with impact-tier rationale>
- **Bugs filed/updated:** [BUG-<NNNN>, ...]
- **Scenarios settled:** <id → verdict, ...>
- **Paper cuts:** <persona-felt friction, severity noted>
- **Surprises:** <unexpected observations>
- **Suggested next charter:** <what this session did not reach>
