# CH-cross-workspace-consent-audit: Answer the crossing prompt, then interrupt everything that holds the answer

```yaml
charter:
  id: CH-cross-workspace-consent-audit
  mission: "As Bruno, answer a cross-workspace prompt every way it can be answered, then interrupt the session and the daemon to prove a session-scoped answer really dies with its session and that the audit trail told the whole story."
  mode: charter-with-tour
  persona:
    name: Bruno
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-cross-workspace-access
  scenarios: [ET-workspace-access-prompt-outcomes]
  tour: Interrupt Tour
  time_box_minutes: 60
  guidance:
    must_try:
      - "Bootstrap a fresh isolated lab with unique COMPOZY_HOME/ports/provider home/tmux socket, register PIDs, and run eval \"$TEARDOWN_COMMAND\" (or make qa-reap) on every pass/fail/blocked/abort exit; cite clean teardown.json."
      - "From an approve-reads session, trigger the native-tool crossing and confirm exactly one pending permission appears with the four daemon-computed options, labeled as once versus for-this-session — then answer allow_once and reject_once and confirm each governs only its own call."
      - "Answer allow_session, then cross at the task-claim, spawn, and coordination seams — which never prompt — and confirm they now succeed on the cached answer, with no approval visible in any list or revoke surface."
      - "Interrupt the holder: stop the session and start a new one for the same agent, then separately restart the daemon mid-session. The first crossing after each interruption must prompt again — a surviving answer is the headline finding."
      - "Answer reject_session and confirm every later crossing at every seam denies with no prompt; then leave one prompt unanswered past the approval timeout and confirm it denies and stores nothing."
      - "Answer through both operator surfaces (compozy session approve <session-id> --request-id <request-id> --decision <allow-once|allow-always|reject-once|reject-always> and the HTTP approve route) and confirm identical option ids and identical resulting behavior."
      - "Read the same decisions through compozy logs --type, GET /api/logs, compozy__logs, and compozy__observe_search and confirm they agree on target, seam, source, and mode; note that a best-effort append missing from a degraded store is a store finding, not a different decision."
    must_avoid:
      - "Re-walking the full mode-by-seam matrix — CH-cross-workspace-mode-seams owns it; only the approve-reads path is in this box."
      - "Any web surface, including the deep-link confirmation."
      - "Treating the absence of a revoke or list surface as a defect: there is deliberately no management surface for session consent (ADR-007), and its absence is the contract being verified."
      - "Parallel config writes against the shared isolated home."
  coverage:
    tier: targeted
    surfaces: [native-tools, ACP-approval-bridge, session-approve-CLI, HTTP-approve-route, task-claim, spawn, network-coordination, compozy-logs, GET-/api/logs, compozy__logs, compozy__observe_search, site-docs]
    invariants: [9, 8, 10]
    hot_spots:
      - "Invariant 9 consent volatility: the answer lives only in daemon memory, applies to every seam, and must not survive a session stop or a daemon restart — the interruptions in this charter exist to attack exactly that."
      - "Invariant 8 prompt containment: only the live native-tool seam may prompt, the option set is daemon-computed, and timeout, transport failure, or an out-of-set answer must deny and persist nothing."
      - "Invariant 10 audit-every-decision: one record per evaluation (spawn's two phases produce two), appended best-effort before a denial is masked, with the actor's home workspace as scope."
    adrs: [ADR-007, ADR-006]
    expected_evidence: "The pending-permission option set as presented; per-answer outcome traces across seams; before/after interruption traces for session stop and daemon restart; the timeout denial; and the same decision read back through all four audit surfaces."
    exit_criteria: "Each of the four answers behaves exactly as scoped, every interruption forced a fresh prompt, the timeout stored nothing, and every observed decision is attributable in the audit trail."
```

<!-- The charter is durable and immutable: re-run it in later cycles; each run's debrief belongs in that run's dated report. -->
