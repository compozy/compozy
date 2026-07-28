# CH-mcp-authorize-repair-truth: Interrupt the authorize flow everywhere and prove no false green survives

```yaml
charter:
  id: CH-mcp-authorize-repair-truth
  mission: "As Bruno repairing a needs_login remote MCP server on /mcp, run the Interrupt Tour through the authorize state machine — cancel, close, expire, supersede, and fail at every step — and prove success only ever appears as authenticated && token_present, that a failed or canceled attempt preserves the prior status and token, and that the status matrix never composes a plausible green."
  mode: charter-with-tour
  persona:
    name: Bruno
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-mcp-authorize-repair
  scenarios: [ET-web-mcp-status-matrix, ET-web-mcp-authorize, ET-web-mcp-remote-editor, MS-029]
  tour: Interrupt Tour
  time_box_minutes: 60
  guidance:
    must_try:
      - "Read the matrix cold: configured, auth, runtime, and probe must be four independent signals; tool count only on a succeeded probe; probe=skipped never a failure; stdio and non-OAuth servers must have no Authorize action; an unknown daemon status value must render neutral with the diagnostic preserved."
      - "Begin authorization and immediately close the dialog; begin again (supersession); let a session sit past its 5-minute TTL; cancel at the provider — after every interruption the server's prior public status and any prior token must be visibly intact."
      - "On an already-authenticated server, run a re-authorization and fail it deliberately: the working credential must survive (token-preservation contract, Safety Invariant 7)."
      - "Confirm the waiting dialog always keeps the live begin URL copyable, and that a tools-probe success alone never flips the UI to authorized — only the refetched authenticated && token_present does."
      - "Edit the repaired server across transports: switching stdio↔http/sse swaps field sets with daemon-matching validation; bound secret refs render as MonoId identifiers; typed values are never reflected back; scope+server survive a page refresh via the URL."
    must_avoid:
      - "The manual paste-back lane (CH-remote-operator-manual-auth owns it); installing new servers (CH-marketplace-under-a-minute); CLI authorize (CH-agent-marketplace-parity settles ET-cli-mcp-authorize)."
  evidence_expectations:
    - "Screenshots: the truthful matrix before repair, the waiting dialog with copyable URL, one interrupted attempt with prior status intact, and the confirmed state — plus a fresh scoped list read (web + CLI/API JSON) agreeing on authenticated && token_present."
    - "For the failed re-auth: before/after status reads proving the prior token survived — this is the truthful-readiness half of the PRD born-valid anchor (zero installed-but-broken, zero false greens)."
    - "Log/SSE/network scan notes confirming no code, verifier, or token material appeared anywhere."
```

<!-- The charter is durable and immutable: each run's debrief belongs in that run's dated report. -->
