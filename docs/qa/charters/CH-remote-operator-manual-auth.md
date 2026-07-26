# CH-remote-operator-manual-auth: Complete authorization from a machine the callback can never reach

```yaml
charter:
  id: CH-remote-operator-manual-auth
  mission: "As Iris, whose daemon runs on another host (non-loopback bind, no local browser handoff), run the Paste Tour through the manual completion lane — copy the authorization URL out, paste back a bare code and a full redirect URL — and prove the ADR-011 floor: the link is always copyable, the auto-callback refuses to exist off-loopback, success requires the confirmed credential, and every malformed or stale paste fails deterministically without touching a prior token."
  mode: charter-with-tour
  persona:
    name: Iris
    device: laptop
    network: wifi-slow
    locale: en-US
  journey: J-mcp-authorize-repair
  scenarios: [ET-web-mcp-authorize-manual, ET-cli-mcp-auth-manual-exchange, ET-api-mcp-oauth-endpoints, ET-047]
  tour: Paste Tour
  time_box_minutes: 60
  guidance:
    must_try:
      - "With the daemon bound non-loopback, prove GET /api/mcp/oauth/callback is refused (Safety Invariant 14) and the web dialog + CLI still expose the copyable URL and the manual completion path."
      - "Complete once by pasting a bare code and once by pasting the full redirect URL — in the web dialog and via `agh mcp authorize <name> --manual` — confirming exactly one value is sent to exchange and success appears only on refetched authenticated && token_present."
      - "Paste garbage: a URL with mismatched state, an expired session's code, both code and redirect together, an empty paste — each must fail deterministically, leave the dialog in a truthful failed state, and preserve any prior token."
      - "Scan CLI output, web DOM, request logs, and events for the pasted code/redirect URL — the exchange inputs are secret-class and must never echo (Safety Invariant 9)."
      - "Run begin/exchange/logout for two workspace targets sharing one server name over HTTP and UDS: payloads must match per plane, and the two workspaces must hold distinct OAuth tokens and distinct canonical secret_env refs (both-channel workspace boundary, Safety Invariant 8)."
    must_avoid:
      - "The browser auto-callback happy path (CH-mcp-authorize-repair-truth owns it); guided installs; editing server config beyond what the auth lane requires."
  evidence_expectations:
    - "Proof of the non-loopback callback refusal (status code + bind config); screenshots of the copyable-URL dialog and the manual field accepting code and redirect-URL forms."
    - "Before/after status reads for each failed paste proving the prior token survived; the homonymous two-workspace token isolation read."
    - "Redaction scan notes: no code, redirect URL, verifier, or token in any output, log, or event."
```

<!-- The charter is durable and immutable: each run's debrief belongs in that run's dated report. -->
