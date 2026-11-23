# CH-skill-edit-review: Review an edit to an installed skill and refuse a stale base

```yaml
charter:
  id: CH-skill-edit-review
  mission: "As Dora, review an agent's edit to an installed skill as a diff, approve it, then see a second edit from the old base refused as stale."
  mode: charter-with-tour
  persona: {name: Dora, device: desktop, network: wifi-fast, locale: en-US}
  journey: J-31
  scenarios: [MS-web-stays-on-reports]
  tour: Feature Tour
  time_box_minutes: 30
  entry_path: "compozy__agent_skill_propose mode edit (twice, same base) → compozy agent skill review show → Web viewer Changes → approve the first → approve the second"
  expected_evidence:
    - "Tool result kind edit with base_digest; CLI show prints Base, both automatic counts, Files and a unified diff."
    - "Web card 'Change to review: <name>', header 'N modified, N added, N removed', diff card; screenshot."
    - "After the first approve, installed SKILL.md contains the new step."
    - "Second approve fails with skill_review_base_stale (409); viewer says the skill changed, Approve disabled, Reject available; screenshot."
  guidance:
    must_try:
      - "Change the installed skill by hand after a proposal and approve it: same stale refusal, files unchanged."
```

<!-- The charter is durable and immutable: each run's debrief belongs in its dated report. -->
