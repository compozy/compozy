# CH-untested-valid-021-mcp-authorize-repair-ada: Valid companion for J-mcp-authorize-repair as Ada

```yaml
charter:
  id: CH-untested-valid-021-mcp-authorize-repair-ada
  mission: "Re-run J-mcp-authorize-repair under the scenario's owning persona and a canonical tour, preserving the historical charter unchanged."
  mode: scenario-based
  persona:
    name: Ada
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-mcp-authorize-repair
  scenarios: [ET-047, ET-api-mcp-oauth-endpoints, ET-cli-mcp-authorize]
  tour: Network Tour
  time_box_minutes: 60
  guidance:
    must_try:
      - "Enter through every scenario's named public surface and capture the final observable from a fresh read."
      - "Exercise one failure, interruption, or invalid-input branch without masking it through an alternate surface."
      - "Compare public surfaces wherever parity is part of the expected behavior."
      - "Prioritize these representative observables first: MCP OAuth/PKCE login/logout/status; Manage scoped MCP OAuth through daemon API routes; Authorize a remote MCP server through the daemon."
    must_avoid:
      - "Do not inherit a verdict from the historical charter or static implementation evidence."
      - "Do not rewrite the historical charter; record this run only in the current report."
```

<!-- Immutable companion charter: historical planning remains untouched. -->
