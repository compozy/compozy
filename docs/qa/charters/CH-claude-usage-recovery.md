# CH-claude-usage-recovery: Recover Claude usage without repeated Keychain approvals

```yaml
charter:
  id: CH-claude-usage-recovery
  mission: "As Ada, read Claude usage after startup and a background refresh while retaining control over Keychain consent."
  mode: charter-with-tour
  persona:
    name: Ada
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-provider-usage
  scenarios: [ET-provider-usage-surfaces, APP-provider-usage-tray]
  tour: Feature Tour
  time_box_minutes: 30
  guidance:
    must_try:
      - "Start only an isolated daemon with authorized consent and an existing native login."
      - "Read Claude usage through CLI and independently through the public HTTP API."
      - "Request a background refresh and confirm the resulting state through a separate read."
      - "Withdraw consent in the lab and confirm credentials and usage become unavailable."
    must_avoid:
      - "Never alter the operator daemon, credentials, ACL, or partition list."
      - "Never print or save tokens; capture only sanitized status and source metadata."
      - "Do not claim a packaged desktop or interactive authorization walk from API evidence."
```
