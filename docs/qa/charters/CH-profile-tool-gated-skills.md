# CH-profile-tool-gated-skills: Offer tool-gated skills in the owning Profile

```yaml
charter:
  id: CH-profile-tool-gated-skills
  mission: "As Ada, project a named-Profile skill catalog and trust that a skill requiring an extension tool is offered only where that tool is available."
  mode: strategy-based
  persona:
    name: Ada
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-offer-runnable-capabilities
  scenarios: [ET-skill-activation-gates]
  tour: Feature Tour
  time_box_minutes: 30
  guidance:
    must_try:
      - "Project a requires_tools skill in the named Profile that owns the extension tool and confirm the current catalog offers it."
      - "Read the same catalog in the default Profile and a peer workspace; the skill must remain manageable but inactive with the missing-tool reason."
      - "Disable and re-enable the tool, confirming the following catalog projection changes without a daemon restart."
    must_avoid:
      - "Treating administrative enabled state as runtime activation or using a direct registry helper as public-interface evidence."
```

<!-- The charter is durable and immutable: each run's debrief belongs in its dated report. -->
