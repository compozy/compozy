# CH-palette-approval-exactly-once: Prove an approval-gated invocation executes exactly once through every structured surface

```yaml
charter:
  id: CH-palette-approval-exactly-once
  mission: "As Ada, drive destructive and UI-effecting palette commands through CLI, HTTP/UDS, and native tools, interrupting the approval lifecycle at every seam — deny, cancel, timeout, duplicate invoke, disconnect — to prove the single-flight guard holds to the terminal outcome and no path ever executes twice or silently."
  mode: charter-with-tour
  persona:
    name: Ada
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-operate-command-palette
  scenarios: [ET-agent-command-invoke, ET-agent-palette-config-parity]
  tour: Interrupt Tour
  time_box_minutes: 60
  guidance:
    must_try:
      - "Invoke a destructive command → 202 approval_pending; while pending, invoke again (must be already_running), then resolve deny, cancel, and timeout on separate approvals — each terminal outcome must release the guard and leave zero side effects."
      - "Approve one invocation and count executions end-to-end (result + `compozy approvals show` + effect); disconnect the initiating client before approval resolves — the resolution must still land exactly once. Repeat the destructive invoke through CLI, HTTP/UDS, and the native tool to prove no transport bypasses policy and a UI confirmation never substitutes for an agent approval."
      - "Invoke a UI-effecting command with zero clients (no_attached_shell), with two attached clients and no --client (multiple_clients listing ids), and with an explicit --client; compare the error code, reason, and client ids across CLI, HTTP/UDS, and compozy__cmd_palette_invoke while allowing each transport's documented envelope."
      - "Interrupt configuration mutations too: fire two concurrent bind/alias writes for the same chord — one wins, the loser gets the structured conflict naming the owner, and `bindings -o json` converges."
    must_avoid:
      - "The web UI (this is the structured-surface lane); default COMPOZY_HOME or ports — use the isolated lab from the bootstrap manifest."
```

## Selection rationale

Targeted tier, highest blast radius: SI-1 (single-flight held across the detached approval
lifetime), SI-8/SI-17 (attached-client identity, explicit targeting, correlated delivery), and SI-9
(tool policy on every destructive path) are the invariants a duplicate execution or silent bypass
would break worst. ADR-005's cross-surface identifier mapping and ADR-006's daemon authority are
exercised through the same CLI/HTTP/UDS/native comparison. The async ApprovalCoordinator contract
(spec Data Models, migration 00069/00078 lineage) remains the central runtime hot spot.

<!-- The charter is durable and immutable: each run's debrief belongs in that run's dated report. -->
