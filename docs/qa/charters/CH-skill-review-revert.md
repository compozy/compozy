# CH-skill-review-revert: Revert an approved new skill and an approved edit

```yaml
charter:
  id: CH-skill-review-revert
  mission: "As Dora, revert an approved edit and an approved new skill from the CLI and the Web, and see Revert offered only when the approval is still the current writer."
  mode: charter-with-tour
  persona: {name: Dora, device: desktop, network: wifi-fast, locale: en-US}
  journey: J-31
  scenarios: [MS-web-stays-on-reports]
  tour: Feature Tour
  time_box_minutes: 30
  entry_path: "compozy agent skill review show (revert.allowed) → compozy agent skill review revert <id> --expected-digest → repeat → Web Revert on a new-skill receipt"
  expected_evidence:
    - "Revert of the edit restores the previous files byte-exact (digest equals the base digest) and prints the receipt; a repeat succeeds without change."
    - "Reverting an older approval superseded by a newer one fails with skill_review_revert_conflict reason superseded; the Web hides its Revert."
    - "Web receipt 'Reverted <name>.' after Revert of a new skill; the skill leaves the instance catalog; screenshots."
```

<!-- The charter is durable and immutable: each run's debrief belongs in its dated report. -->
