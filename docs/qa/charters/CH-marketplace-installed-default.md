# CH-marketplace-installed-default: Marketplace kind pages open on Installed and preserve explicit catalog scope

```yaml
charter:
  id: CH-marketplace-installed-default
  mission: "As Bruno, enter each Marketplace kind from normal navigation, manage installed items first, then browse the catalog and return from detail without losing the explicit scope."
  mode: charter-with-tour
  persona:
    name: Bruno
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-marketplace-acquisition
  scenarios: [ET-web-marketplace-landing-browse, ET-web-marketplace-kind-navigation, ET-web-marketplace-installed-management, ET-web-extensions-manage, ET-web-mcp-status-matrix]
  tour: Back-Button Tour
  time_box_minutes: 30
  guidance:
    must_try:
      - "Enter through the sidebar and `/marketplace`; Skills must open with Installed selected and no `tab` query."
      - "Choose Marketplace, switch through Skills, MCPs, and Extensions, and confirm every URL preserves `tab=market`."
      - "Open one catalog detail, use Back, and confirm Marketplace remains selected; use Manage and confirm Installed returns with `tab` omitted."
      - "Refresh both explicit scopes and verify the selected scope and daemon-backed card state remain truthful."
    must_avoid:
      - "Do not treat a catalog-empty fixture as proof of Installed management behavior."
```

<!-- The charter is durable and immutable: each run's debrief belongs in its dated report. -->
