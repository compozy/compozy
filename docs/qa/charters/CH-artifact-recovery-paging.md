# CH-artifact-recovery-paging: An oversized tool result is never lost, on any surface

```yaml
charter:
  id: CH-artifact-recovery-paging
  mission: "As Rafa, overflow a tool result with adversarial content and prove the preview plus byte-identical page-back on every surface, restart durability, retention, isolation, and the typed partial-failure path."
  mode: charter-with-tour
  persona:
    name: Rafa
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-14
  scenarios: [ET-tool-result-artifact-recovery]
  tour: Garbage Tour
  time_box_minutes: 60
  guidance:
    must_try:
      - "Overflow with a fixture that puts a multi-byte character across the 64 KiB page boundary; concatenate offset-ordered pages from Web, native reader, CLI, HTTP, and UDS and require byte-identical content with the same canonical JSON envelope."
      - "Restart the daemon and read again; probe from another workspace (same not-found shape as expired); exercise count, byte, and age retention independently."
      - "Inject one persistence failure → typed error with the bounded partial result and no fabricated durable URI."
      - "Web card: preview preserved through loading, not-found, and retry states."
    must_avoid:
      - "Trusting a single surface's read as proof of byte identity — the cross-surface concatenation is the check."
```

<!-- The charter is durable and immutable: re-run it in later cycles; each run's debrief goes in that run's report (Session Debriefs), never here. -->
