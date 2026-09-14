# CH-marketplace-public-ownership: Preserve operator ownership through Marketplace maintenance

```yaml
charter:
  id: CH-marketplace-public-ownership
  mission: "As Bruno maintaining local tooling, change Marketplace sources, development packages and profile names while keeping unrelated skills, MCPs and credentials available in their owning scope."
  mode: charter-with-tour
  persona:
    name: Bruno
    device: desktop CLI
    network: local
    locale: en-US
  journey: J-agent-marketplace-parity
  scenarios: [ET-api-marketplace-sources, ET-agent-plugin-dev-reload, RT-agent-hot-discovery-skill-isolation, ET-profile-cli-lifecycle, ET-cli-mcp-authorize]
  tour: Data Tour
  time_box_minutes: 90
  guidance:
    must_try:
      - "Add a local source, reject invalid mutations, and compare persisted source state through CLI and HTTP after an unrelated successful settings edit."
      - "Inspect and dev-link an authored package whose directory and manifest names differ; compare identity and resources across two workspaces, reload and restart."
      - "Open a previously installed Marketplace skill after restart and confirm its content remains while embedded MCP discovery is absent; keep a manual MCP as the adjacent canary."
      - "Rename a profile holding synthetic manual and extension MCP secret references in profile and workspace-profile scopes, then compare deletion preview with the applied result while shared/user references survive."
      - "Attempt MCP authorization with foreign or daemon-managed credential references and verify rejection before authorization begins; compare own/shared/environment reference handling without disclosing values."
    must_avoid:
      - "Do not inspect or mutate SQLite during the walk, call internal APIs, relax assertions, or mutate/expose operator credentials. A native_cli provider may read its existing login only to initialize an empty QA session; no model prompt is sent."
      - "Do not claim full provider OAuth completion from configuration or presence checks; keep injected persistence failures and ciphertext checks attributed to their owning CI integration suites."
  evidence_expectations:
    - "Exact CLI/HTTP transcripts, independent public reads, restart persistence, synthetic-secret values redacted, and a debrief distinguishing live observations from reused CI evidence."
```
