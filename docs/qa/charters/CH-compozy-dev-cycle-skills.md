# CH-compozy-dev-cycle-skills: Managed sessions receive exactly eight immutable workflow skills

```yaml
charter:
  id: CH-compozy-dev-cycle-skills
  mission: "As Ada, enroll dev-cycle and prove every managed session receives exactly the eight declared immutable workflow skills, with workspace-local shadowing and no retired installer or duplicate runtime skill."
  mode: scenario-based
  persona:
    name: Ada
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-offer-runnable-capabilities
  scenarios: [ET-dev-cycle-skill-bundle, ET-dev-cycle-legacy-skill-retired]
  tour: Feature Tour
  time_box_minutes: 45
  guidance:
    must_try:
      - "Inspect extension resources, compozy skill list/view, compozy__skill_list/view, and a managed-session prompt for the exact seven cy-* skills plus git-rebase."
      - "Create one workspace-local override and prove it shadows only in that workspace while another workspace retains the immutable global declaration."
      - "Re-enroll dev-cycle and confirm no extension-owned compozy skill, cy-capture-decisions, tenth skill, external agent-CLI installer, or external CLI home write appears."
      - "Verify the reviewer role explicitly receives cy-review-round and every referenced workflow skill is actually runnable."
    must_avoid:
      - "Editing the global bundled source through a workspace management surface."
      - "Counting the separate official Compozy runtime skill as part of the eight dev-cycle resources."
  coverage:
    tier: targeted
    surfaces: [extension-resources, skill-registry, CLI, HTTP, UDS, native-tools, managed-session-prompt]
    invariants: [12, 13]
    adrs: [ADR-004]
    expected_evidence: "Exact skill-name set, bodies/origins, re-enrollment result, and two-workspace shadowing comparison."
    exit_criteria: "Exactly eight dev-cycle skills are runnable from immutable global resources and no retired installation path or duplicate skill exists."
```

<!-- The charter is durable and immutable: each run's debrief belongs in that run's dated report. -->

