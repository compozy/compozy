# CH-session-launch-composer-handoff: Launch one session and arrive at its runtime-capable composer

```yaml
charter:
  id: CH-session-launch-composer-handoff
  mission: "As Dora, launch one durable session with only its legitimate launch details, then prove its owner workspace and destination composer preserve the user's ability to choose the first prompt runtime."
  mode: charter-with-tour
  persona:
    name: Dora
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-17
  scenarios: [MS-web-session-simple-advanced-launch, RT-010, RT-063, ET-web-session-prompt-runtime-and-create-navigation]
  tour: Feature Tour
  time_box_minutes: 60
  guidance:
    must_try:
      - "Enter from both Agents and agent detail. In Simple, choose the agent and verify there is neither a first-message composer nor a runtime selector; open Advanced to set workspace, optional name, working path, and Network participation."
      - "Change the workspace after choosing workspace-scoped launch details, then prove only those selections clear. Create once and independently fresh-read one durable session with no queued prompt."
      - "Observe immediate feedback, owner-workspace activation, route navigation, and focused destination composer. Refresh and use Back/Forward without a lingering modal, duplicated session, or silent workspace redirect."
      - "From the destination composer select a Next prompt runtime and submit the first message; prove the runtime belongs to that prompt rather than the creation request or agent default."
    must_avoid:
      - "Do not restore the deleted create-modal prompt/runtime controls or infer a pass from request inspection alone."
      - "Do not settle live runtime transition, queue snapshot, or typed rejection behavior here; CH-prompt-bound-runtime-transition and CH-prompt-runtime-fail-loud own those paths."
```

<!-- The charter is durable and immutable: re-run it in later cycles; each run's debrief goes in that run's report (Session Debriefs), never here. -->
