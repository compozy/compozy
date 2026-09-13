# CH-marketplace-scope-isolation: Keep extension inputs and destination isolated

Mission updated for the approved Marketplace hard cut. Historical debriefs stay in their dated reports; task_10 owns the next run.

```yaml
charter:
  id: CH-marketplace-scope-isolation
  mission: "Keep extension inputs and destination isolated, with truthful origin, destination and persisted state."
  mode: charter-with-tour
  persona:
    name: Bruno
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-marketplace-acquisition
  scenarios: [ET-web-marketplace-sources-manage, ET-web-marketplace-mcp-authorize-installed, ET-web-marketplace-remove-scope-return]
  tour: Multi-Tab Tour
  time_box_minutes: 60
  guidance:
    must_try:
      - "Begin an extension install in a workspace, enter an input, switch destination, and confirm the new draft contains no previous secret."
      - "Use two windows on the same entry; each submits only its current profile/workspace and approved digest."
      - "Disable or remove the source while an installed extension remains; verify its inputs, manual MCPs and shared credentials are intact. Final instance removal may clean only its bindings and exclusively owned secrets."
    must_avoid:
      - "Do not use retired per-kind Marketplace acquisition as a fallback or expose credentials in evidence."
```
