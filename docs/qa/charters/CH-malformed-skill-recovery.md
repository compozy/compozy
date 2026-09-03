# CH-malformed-skill-recovery: Repair one broken skill without losing its neighbors

```yaml
charter:
  id: CH-malformed-skill-recovery
  mission: "As Dora, find and repair one malformed workspace skill while every healthy neighbor remains available and repeated scans stay quiet."
  mode: charter-with-tour
  persona:
    name: Dora
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-diagnose-skill-sources
  scenarios: [ET-skill-ecosystem-frontmatter-quiet]
  tour: Garbage Tour
  time_box_minutes: 30
  guidance:
    must_try:
      - "Place one malformed SKILL.md beside one healthy skill, then compare the human and JSON source/catalog reads: the broken definition must be withheld and the healthy one must remain usable."
      - "Read diagnostics and daemon logs across repeated scans, confirm the broken file is named once without a warning flood, then repair it and verify it appears without a daemon restart."
      - "Keep a directory without SKILL.md beside both skills and confirm it remains ignored."
    must_avoid:
      - "Do not expand into source-policy writes, collision precedence, or hostile symlink coverage; their existing charters own those behaviors."
```

<!-- The charter is durable and immutable: re-run it in later cycles; each run's debrief goes in that run's report (Session Debriefs), never here. -->
