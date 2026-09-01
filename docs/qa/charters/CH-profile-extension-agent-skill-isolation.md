# CH-profile-extension-agent-skill-isolation: Follow one Agent through every profile layer

```yaml
charter:
  id: CH-profile-extension-agent-skill-isolation
  mission: "As Ada, install one Profile-bound extension Agent and layer one same-named Agent-local Skill from global through Workspace+Profile, then make the public CLI and HTTP API identify the same winner at every step without leaking the extension Agent into default."
  mode: charter-with-tour
  persona:
    name: Ada
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-layer-profile-resources
  scenarios: [ET-profile-extension-agent-skill-isolation]
  tour: Feature Tour
  time_box_minutes: 30
  guidance:
    must_try:
      - "Install and enable a local extension with an Agent declaration placed only in the finance Profile. Compare default and finance through `compozy agent list` and `GET /api/agents`; the Agent must be absent from default and present exactly once in finance, including after a repeated read."
      - "Give the same Agent and Agent-local Skill distinct global, default-Profile, finance-Profile, and Workspace+Profile descriptions and bodies. Walk the four contexts in precedence order through `compozy skill list --for-agent`, `skill where`, `skill view`, and HTTP skill list/detail, asserting the exact winner and source rather than only a count."
      - "Probe an unknown Agent and an empty `for_agent` query through public surfaces. Require typed not-found or validation behavior and no catalog mutation, then repeat the successful finance and Workspace+Profile reads to prove recovery."
    must_avoid:
      - "The released home, shared ports, Web UI, provider credentials, direct database reads, internal Go calls, or the separate diagnostic UI checkout. Use only the isolated bootstrap manifest and its literal teardown command."
```

## Selection rationale

The regression crosses two ownership seams: extension publication decides which Profile owns an
Agent, while Agent-scoped Skill resolution must preserve the projected Agent's source path after the
catalog applies layer precedence. A Feature Tour is the smallest useful walk because a count-only
check can pass while every scope reads the same lower-layer file. Distinct bodies and public CLI/API
reads make each winning layer observable.

## Evidence and entry points

- **CLI** — structured `agent list`, `skill list --for-agent`, `skill where`, and `skill view` output
  for default, finance, and Workspace+Profile contexts.
- **HTTP** — `/api/agents`, `/api/skills?for_agent=...`, and `/api/skills/{name}` responses from the
  isolated daemon listener.
- **Filesystem inputs** — four authored Agent roots and one local extension fixture created inside
  the QA lab before the walk; no released-home or repository mutation.
- **Cleanup** — strict evidence audit plus `qa/teardown.json` with `clean: true` and no survivors.

<!-- The charter is durable and immutable: each run's debrief belongs in that run's dated report. -->
