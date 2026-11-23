# CH-skill-lesson-review: Review an agent's skill lesson and approve it from the CLI and the Web

```yaml
charter:
  id: CH-skill-lesson-review
  mission: "As Dora, let an always-on agent propose a new skill with a lesson and evidence sessions, read the lesson in the CLI and the Web viewer, and approve it by the digest that was read."
  mode: charter-with-tour
  persona: {name: Dora, device: desktop, network: wifi-fast, locale: en-US}
  journey: J-31
  scenarios: [MS-web-stays-on-reports]
  tour: Feature Tour
  time_box_minutes: 30
  entry_path: "agent turn calling compozy__agent_skill_propose → compozy agent skill review list/show → Web report card → viewer → compozy agent skill review approve --expected-digest"
  expected_evidence:
    - "Native tool result with kind new, lesson and evidence_session_ids, state pending."
    - "CLI show prints the Lesson block before the files; list shows KIND and DECIDED VIA columns."
    - "Web card reads 'When: <cue>'; viewer Lesson block links each session; screenshot."
    - "Approve prints '<name>: approved'; the skill is active for the instance."
  guidance:
    must_try:
      - "Reuse the E2E-001 walk on PR #1965 when the head still carries the same lesson surfaces; re-walk only the Web card and viewer on the integrated head."
    must_avoid:
      - "Do not approve from the Web without reading the digest the viewer shows."
```

<!-- The charter is durable and immutable: each run's debrief belongs in its dated report. -->
