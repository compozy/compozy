# CH-compozy-platform-hard-cut: One Compozy runtime identity survives fresh boot and restart

```yaml
charter:
  id: CH-compozy-platform-hard-cut
  mission: "As Ada, prove the candidate exposes one Compozy executable, home, database, environment, native-tool, hosted-MCP, and official-skill identity with no legacy alias or state merge."
  mode: scenario-based
  persona:
    name: Ada
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-validate-compozy-hard-cut
  scenarios: [RT-compozy-cli-binary, RT-compozy-global-database, RT-compozy-home-layout, RT-compozy-home-isolation, RT-compozy-environment-namespace, ET-compozy-native-tool-invocation, ET-compozy-extension-contract-identity, ET-compozy-official-skill-discovery]
  tour: Garbage Tour
  time_box_minutes: 60
  guidance:
    must_try:
      - "Start two sequential isolated COMPOZY_HOME runtimes; compare on-disk paths, status/doctor JSON, helpers, compozy.db, and per-session events.db."
      - "Probe managed-session, hosted-MCP, Web proxy/assets, provider, and bridge environment inputs; only COMPOZY_* names may influence Compozy-owned behavior, while retired environment names, commands, and database files remain inert."
      - "List and invoke native tools through CLI and a managed session, compare hosted MCP tools/list and tools/call, then inspect the one official skill across CLI, HTTP, UDS, native tools, and Web; every legacy tool, host, metadata, or skill alias must be absent."
      - "Load extension manifests and inspect workspace packages plus generated API readers; only min_compozy_version, @compozy/*, openapi/compozy.json, and the two canonical TypeScript declarations may resolve."
    must_avoid:
      - "The Task-14 live migrator and first-boot legacy-state probe."
      - "Parallel config writes against one runtime home."
  coverage:
    tier: targeted
    surfaces: [CLI, HTTP, UDS, environment, extension-manifests, packages, OpenAPI, native-tools, hosted-MCP, filesystem, SQLite, doctor, Web-skills]
    invariants: [10, 13]
    adrs: [ADR-005, ADR-006]
    expected_evidence: "Structured parity captures, isolated home trees, legacy rejection results, and restart/reopen reads."
    exit_criteria: "Every listed surface is Compozy-only after restart and the two isolated homes never cross-read."
```

<!-- The charter is durable and immutable: each run's debrief belongs in that run's dated report. -->
