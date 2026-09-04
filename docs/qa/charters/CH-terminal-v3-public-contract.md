# CH-terminal-v3-public-contract: Drive the shared terminal through every public control plane

```yaml
charter:
  id: CH-terminal-v3-public-contract
  mission: "As Ada, complete one terminal lifecycle through CLI, HTTP, UDS, hooks, and native tools, looking for any v2 ownership field, removed verb, inconsistent result, or special typing permission that survived the v3 hard cut."
  mode: charter-with-tour
  persona:
    name: Ada
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-operate-terminal-by-cli
  scenarios: [ET-terminal-cli-public-contract, ET-terminal-hook-events, ET-terminal-approval-ladder-grants]
  tour: Feature Tour
  time_box_minutes: 90
  guidance:
    must_try:
      - "Exercise every terminal CLI verb, including two simultaneous interactive attachments, and compare structured reads and mutations with the matching HTTP and UDS routes."
      - "Open the catalog and terminal streams, reconnect from their cursors, and verify terminal wire v3 has no controller, lease, takeover, claim, or yield frame."
      - "List the native tool catalog and use all nine terminal tools; claim and yield must be absent, and terminal_write must follow ordinary native-tool policy."
      - "Drive every supported lifecycle hook once and confirm exactly ten terminal hook ids remain; shared writes and resizes must not synthesize an ownership event."
      - "Approve and reject ordinary terminal commands, exercise remembered command grants and the fixed irreversible set, then confirm no terminal-scoped typing grant, settings key, or grants row exists."
    must_avoid:
      - "Do not settle profile selector hostility or platform fallback; those remain owned by their existing terminal charters."
```

## Focus areas

- Wire v3 is one hard contract with no v2 decoder or compatibility response.
- Nine native terminal tools and ten terminal hooks agree across generated and runtime catalogs.
- Ordinary command/destructive-operation policy remains intact; only the special typing grant is gone.
- Interactive CLI attaches are writable immediately and retain their raw-mode detach contract.

<!-- The charter is durable and immutable: re-run it in later cycles; each run's debrief goes in that run's report (Session Debriefs), never here. -->
