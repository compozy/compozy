# CH-agent-plugin-remote-secrets: Reach one authenticated remote server without leaking its credential

```yaml
charter:
  id: CH-agent-plugin-remote-secrets
  mission: "As Bruno, bind a Vault-backed header to one portable streamable-HTTP server, invoke it through a managed session, and prove the value reaches only that daemon-owned request."
  mode: charter-with-tour
  persona:
    name: Bruno
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-extension-kit-lifecycle
  scenarios: [ET-agent-plugin-remote-header]
  tour: Paste Tour
  time_box_minutes: 60
  guidance:
    must_try:
      - "Paste a unique full Authorization value into the isolated Vault, bind it with --remote-header, and capture the named endpoint receiving it once while a sibling endpoint and provider environment do not."
      - "Compare secret list, status, inventory, HTTP, UDS, native reads, Web, logs, events, diagnostics, session transcript, and stored config; only key/server/header names and presence may appear."
      - "Probe package-declared authorization/cookie, content-type, mcp-* names, case-insensitive fixed/secret duplicates, and OAuth plus Authorization binding; each source-aware rule produces one deterministic component outcome."
      - "Rotate and unset the binding, reconnect, and scan retained evidence for both the Vault reference and plaintext value before teardown."
    must_avoid:
      - "Placing the secret in argv, reports, screenshots, prompts, provider config, or committed evidence; using a production endpoint or credential."
```

<!-- The charter is durable and immutable: each run's debrief belongs in its dated report. -->
