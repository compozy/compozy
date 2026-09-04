# CH-terminal-shared-control-docs: Follow the shared-control documentation without prior knowledge

```yaml
charter:
  id: CH-terminal-shared-control-docs
  mission: "As Lea meeting the terminal for the first time, follow only the published terminal pages, generated CLI reference, and official Compozy skill, looking for any instruction that still makes her negotiate control or expect a removed tool or permission."
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
      - "Run the terminal tutorial verbatim, including interactive attach, and confirm input is available immediately while the printed banner and detach chord match."
      - "Read the safety page and verify it separates ordinary command approval from shared terminal input, keeps terminal output untrusted, and names no typing grant or control handoff."
      - "Compare the generated CLI reference with accepted flags and verbs; control, force, takeover, claim, and yield must be absent."
      - "Resolve the official skill's terminal and native-tools references through its public read planes; both must teach nine tools and the same shared-control contract as the daemon."
    must_avoid:
      - "Do not consult source, archived specifications, or old QA charters to resolve ambiguity; anything the current public material cannot explain is a finding."
```

## Focus areas

- A first-time user can attach and type without learning ownership machinery.
- Public docs, generated reference, official skill, and runtime catalogs tell the same story.
- Safety copy preserves real approval and redaction guarantees without carrying removed policy.

<!-- The charter is durable and immutable: re-run it in later cycles; each run's debrief goes in that run's report (Session Debriefs), never here. -->
