# CH-skill-auto-approve: Turn on skill auto-approve, hit the daily cap, audit and revert

```yaml
charter:
  id: CH-skill-auto-approve
  mission: "As Dora, turn on skill auto-approve for one agent, watch eligible proposals approve themselves and ineligible ones wait with a reason, see the fourth eligible proposal of the UTC day wait with daily_cap, then audit and revert an automatic approval."
  mode: charter-with-tour
  persona: {name: Dora, device: desktop, network: wifi-fast, locale: en-US}
  journey: J-31
  scenarios: [MS-web-stays-on-reports]
  tour: Feature Tour
  time_box_minutes: 45
  entry_path: "compozy agent instance skill-auto-approve on|off <agent> → compozy__agent_skill_propose (high-confidence eligible ×4, low confidence, script) → compozy agent skill review list --decided-via auto_approve → Web card → revert"
  expected_evidence:
    - "With auto-approve off, a proposal result carries auto_approve_skipped toggled_off."
    - "Turning on prints 'Skill auto-approve is on for <agent>.'; with agent identity on fails skill_auto_approve_operator_only."
    - "Three eligible proposals return approved with decided_via auto_approve; the fourth stays pending with daily_cap."
    - "Low confidence → low_confidence; a draft with scripts → runs_automatically."
    - "list --decided-via auto_approve lists exactly the automatic approvals; Web card 'Approved automatically: <name>. It's active now.'; screenshot."
    - "Revert of an automatic approval removes the skill and does not refund the cap (a fifth eligible proposal still waits with daily_cap)."
```

<!-- The charter is durable and immutable: each run's debrief belongs in its dated report. -->
