# CH-archive-session-catalog: Hide finished work without losing it

```yaml
charter:
  id: CH-archive-session-catalog
  mission: "As Cora, archive finished sessions from the Web catalog, recover one after refresh, and manage each row without accidentally opening or deleting the session."
  mode: charter-with-tour
  persona:
    name: Cora
    device: laptop
    network: wifi-fast
    locale: en-US
  journey: J-archive-session-without-deleting
  scenarios: [RT-session-list-row-actions, RT-014, RT-082, ET-web-sessions-catalog-modal]
  tour: Feature Tour
  time_box_minutes: 60
  guidance:
    must_try:
      - "Archive a stopped session from the agent list and the global catalog, refresh, expand Archived, and unarchive the same row without opening its detail first."
      - "Use the row menu by pointer and keyboard at compact and desktop widths; opening it must not navigate, Delete must still confirm, and Cancel must preserve the row."
      - "Keep one active session beside the stopped row; its menu must not offer Archive, and Stop must remain available."
      - "Search and paginate both groups; the Archived number must stay exact and errors must be visible in plain language."
    must_avoid:
      - "CLI, native-tool, and extension parity owned by CH-archive-session-structured-parity."
```

<!-- The charter is durable and immutable: re-run it in later cycles; each run's debrief goes in that run's report. -->
