# CH-agent-plugin-marketplace: Acquire portable content without letting catalog metadata become truth

```yaml
charter:
  id: CH-agent-plugin-marketplace
  mission: "As Bruno, discover and install an Agent Plugins catalog entry in the browser, then prove its neutral badge, trust gate, runtime-detected format, and skipped rows agree with structured surfaces."
  mode: charter-with-tour
  persona:
    name: Bruno
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-marketplace-acquisition
  scenarios: [ET-agent-plugin-marketplace-install]
  tour: Back-Button Tour
  time_box_minutes: 60
  guidance:
    must_try:
      - "Open /marketplace/extensions?tab=market, find the portable catalog card, inspect detail, traverse back/forward, complete trust consent, install, and follow Manage to installed inventory."
      - "Capture the neutral Agent Plugin badge on card and detail plus ordered Skipped rows on installed detail; native extensions remain unbadged and fully degraded portable state never uses native empty copy."
      - "Compare the feed, marketplace CLI and HTTP/UDS catalog reads with installed CLI/API/native payloads; acquired bytes override absent or deliberately stale feed format metadata."
      - "Open /settings/extensions to confirm policy remains the existing extension setting, refresh/deep-link the installed detail, and verify no new Agent Plugins config key or product section exists."
    must_avoid:
      - "Using Storybook or mocked routes as runtime evidence, accepting the feed marker as installed truth, or inventing a separate plugin lifecycle or settings surface."
```

<!-- The charter is durable and immutable: each run's debrief belongs in its dated report. -->
