# CH-gateway-audit-teardown: Clear a finding and remove exposure

```yaml
charter:
  id: CH-gateway-audit-teardown
  mission: "As Dora, run the Feature Tour through one real gateway finding and its own printed remediation, compare every control plane, then disable exposure and restart to prove the audit never reports stale safety."
  mode: charter-with-tour
  persona:
    name: Dora
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-audit-and-teardown-gateway
  scenarios: [RT-gateway-self-audit, RT-connectivity-provider-route, RT-gateway-local-only-boot, RT-gateway-browser-stream-reconnect, RT-secret-redaction-boundary]
  tour: Feature Tour
  time_box_minutes: 60
  guidance:
    must_try:
      - "Run audit from web, CLI, HTTP, UDS, and `compozy__gateway`; compare stable IDs, severity order, remediation, explicit no-findings, and unchanged desired/observed state."
      - "Create a real provider or exposure finding, follow only the remediation it prints, and re-run until that finding clears without hiding unrelated findings."
      - "Attempt a native mutation below approve-all, then disable every surface while remote streams and one mutation are active; admission, commit fencing, and permission refusal must hold."
      - "Restart the daemon, re-run status and audit on every plane, and scan their bytes plus events/logs for claim tokens, device credentials, pairing artifacts, webhook secrets, and stream tickets."
    must_avoid:
      - "Editing the finding catalog or weakening the posture to manufacture a clear result; claiming teardown before the isolated lab's terminal cleanup."
  evidence_expectations:
    - "Cross-plane audit payloads, before/after finding, remediation command/result, stream and mutation withdrawal, restart status, redaction scan, and terminal teardown record."
```

<!-- The charter is durable and immutable: each run's debrief belongs in its dated report. -->

